// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

// Package httprequest provides functionality for unmarshaling
// HTTP request parameters into a struct type.
package httprequest

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/errgo.v1"
)

// TODO include field name and source in error messages.

var (
	typeMutex sync.RWMutex
	typeMap   = make(map[reflect.Type]*requestType)
)

// Params holds request parameters that can
// be unmarshaled into a struct.
type Params struct {
	*http.Request
	PathVar httprouter.Params
}

// resultMaker is provided to the unmarshal functions.
// When called with the value passed to the unmarshaler,
// it returns the field value to be assigned to,
// creating it if necessary.
type resultMaker func(reflect.Value) reflect.Value

// unmarshaler unmarshals some value from params into
// the given value. The value should not be assigned to directly,
// but passed to makeResult and then updated.
type unmarshaler func(v reflect.Value, p Params, makeResult resultMaker) error

// marshaler marshals the specified value into params.
type marshaler func(reflect.Value, *Params) error

// requestType holds information derived from a request
// type, preprocessed so that it's nice and quick to unmarshal.
type requestType struct {
	fields []field
}

// field holds preprocessed information on an individual field
// in the result.
type field struct {
	// index holds the index slice of the field.
	index []int

	// unmarshal is used to unmarshal the value into
	// the given field. The value passed as its first
	// argument is not a pointer type, but is addressable.
	unmarshal unmarshaler

	// marshal is used to marshal the value into the
	// give filed. The value passed as its first argument is not
	// a pointer type, but it is addressable.
	marshal marshaler

	// makeResult is the resultMaker that will be
	// passed into the unmarshaler.
	makeResult resultMaker
}

var (
	ErrUnmarshal        = errgo.New("httprequest unmarshal error")
	ErrBadUnmarshalType = errgo.New("httprequest bad unmarshal type")
	emptyValue          = reflect.Value{}
)

// Unmarshal takes values from given parameters and fills
// out fields in x, which must be a pointer to a struct.
//
// Tags on the struct's fields determine where each field is filled in
// from. Similar to encoding/json and other encoding packages, the tag
// holds a comma-separated list. The first item in the list is an
// alternative name for the field (the field name itself will be used if
// this is empty). The next item specifies where the field is filled in
// from. It may be:
//
//	"path" - the field is taken from a parameter in p.PathVar
//		with a matching field name.
//
// 	"form" - the field is taken from the given name in p.Form
//		(note that this covers both URL query parameters and
//		POST form parameters)
//
//	"body" - the field is filled in by parsing the request body
//		as JSON.
//
// For path and form parameters, the field will be filled out from
// the field in p.PathVar or p.Form using one of the following
// methods (in descending order of preference):
//
// - if the type is string, it will be set from the first value.
//
// - if the type is []string, it will be filled out using all values for that field
//    (allowed only for form)
//
// - if the type implements encoding.TextUnmarshaler, its
// UnmarshalText method will be used
//
// -  otherwise fmt.Sscan will be used to set the value.
//
// When the unmarshaling fails, Unmarshal returns an error with an
// ErrUnmarshal cause. If the type of x is inappropriate,
// it returns an error with an ErrBadUnmarshalType cause.
func Unmarshal(p Params, x interface{}) error {
	xv := reflect.ValueOf(x)
	pt, err := getRequestType(xv.Type())
	if err != nil {
		return errgo.WithCausef(err, ErrBadUnmarshalType, "bad type %s", xv.Type())
	}
	if err := unmarshal(p, xv, pt); err != nil {
		return errgo.Mask(err, errgo.Is(ErrUnmarshal))
	}
	return nil
}

// unmarshal is the internal version of Unmarshal.
func unmarshal(p Params, xv reflect.Value, pt *requestType) error {
	xv = xv.Elem()
	for _, f := range pt.fields {
		fv := xv.FieldByIndex(f.index)
		// TODO store the field name in the field so
		// that we can produce a nice error message.
		if err := f.unmarshal(fv, p, f.makeResult); err != nil {
			return errgo.WithCausef(err, ErrUnmarshal, "cannot unmarshal into field")
		}
	}
	return nil
}

// getRequestType is like parseRequestType except that
// it returns the cached requestType when possible,
// adding the type to the cache otherwise.
func getRequestType(t reflect.Type) (*requestType, error) {
	typeMutex.RLock()
	pt := typeMap[t]
	typeMutex.RUnlock()
	if pt != nil {
		return pt, nil
	}
	typeMutex.Lock()
	defer typeMutex.Unlock()
	if pt = typeMap[t]; pt != nil {
		// The type has been parsed after we dropped
		// the read lock, so use it.
		return pt, nil
	}
	pt, err := parseRequestType(t)
	if err != nil {
		return nil, errgo.Mask(err)
	}
	typeMap[t] = pt
	return pt, nil
}

// parseRequestType preprocesses the given type
// into a form that can be efficiently interpreted
// by Unmarshal.
func parseRequestType(t reflect.Type) (*requestType, error) {
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("type is not pointer to struct")
	}

	var reflectType reflect.Type
	if t.Kind() == reflect.Ptr {
		reflectType = t.Elem()
	} else {
		reflectType = t
	}

	hasBody := false
	var pt requestType
	for _, f := range fields(reflectType) {
		if f.PkgPath != "" {
			// Ignore unexported fields (note that this
			// does not apply to anonymous fields).
			continue
		}
		tag, err := parseTag(f.Tag, f.Name)
		if err != nil {
			return nil, errgo.Notef(err, "bad tag %q in field %s", f.Tag, f.Name)
		}
		if tag.source == sourceBody {
			if hasBody {
				return nil, errgo.New("more than one body field specified")
			}
			hasBody = true
		}
		field := field{
			index: f.Index,
		}
		if f.Type.Kind() == reflect.Ptr {
			// The field is a pointer, so when the value is set,
			// we need to create a new pointer to put
			// it into.
			field.makeResult = makePointerResult
			f.Type = f.Type.Elem()
		} else {
			field.makeResult = makeValueResult
		}

		field.unmarshal, err = getUnmarshaler(tag, f.Type)
		if err != nil {
			return nil, errgo.Mask(err)
		}

		field.marshal, err = getMarshaler(tag, f.Type)
		if err != nil {
			return nil, errgo.Mask(err)
		}

		if f.Anonymous {
			if tag.source != sourceBody && tag.source != sourceNone {
				return nil, errgo.New("httprequest tag not yet supported on anonymous fields")
			}
		}
		pt.fields = append(pt.fields, field)
	}
	return &pt, nil
}

func makePointerResult(v reflect.Value) reflect.Value {
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return v.Elem()
}

func makeValueResult(v reflect.Value) reflect.Value {
	return v
}

// getUnmarshaler returns an unmarshaler function
// suitable for unmarshaling a field with the given tag
// into a value of the given type.
func getUnmarshaler(tag tag, t reflect.Type) (unmarshaler, error) {
	switch {
	case tag.source == sourceNone:
		return unmarshalNop, nil
	case tag.source == sourceBody:
		return unmarshalBody, nil
	case t == reflect.TypeOf([]string(nil)):
		if tag.source != sourceForm {
			return nil, errgo.New("invalid target type []string for path parameter")
		}
		return unmarshalAllField(tag.name), nil
	case t == reflect.TypeOf(""):
		return unmarshalString(tag), nil
	case implementsTextUnmarshaler(t):
		return unmarshalWithUnmarshalText(t, tag), nil
	default:
		return unmarshalWithScan(tag), nil
	}
}

// unmarshalNop just creates the result value but does not
// fill it out with anything. This is used to create pointers
// to new anonymous field members.
func unmarshalNop(v reflect.Value, p Params, makeResult resultMaker) error {
	makeResult(v)
	return nil
}

// unmarshalAllField unmarshals all the form fields for a given
// attribute into a []string slice.
func unmarshalAllField(name string) unmarshaler {
	return func(v reflect.Value, p Params, makeResult resultMaker) error {
		vals := p.Form[name]
		if len(vals) > 0 {
			makeResult(v).Set(reflect.ValueOf(vals))
		}
		return nil
	}
}

// unmarshalString unmarshals into a string field.
func unmarshalString(tag tag) unmarshaler {
	getVal := formGetters[tag.source]
	if getVal == nil {
		panic("unexpected source")
	}
	return func(v reflect.Value, p Params, makeResult resultMaker) error {
		val, ok := getVal(tag.name, p)
		if ok {
			makeResult(v).SetString(val)
		}
		return nil
	}
}

// unmarshalBody unmarshals the http request body
// into the given value.
func unmarshalBody(v reflect.Value, p Params, makeResult resultMaker) error {
	data, err := ioutil.ReadAll(p.Body)
	if err != nil {
		return errgo.Notef(err, "cannot read request body")
	}
	// TODO allow body types that aren't necessarily JSON.
	result := makeResult(v)
	if err := json.Unmarshal(data, result.Addr().Interface()); err != nil {
		return errgo.Notef(err, "cannot unmarshal request body")
	}
	return nil
}

// formGetters maps from source to a function that
// returns the value for a given key and reports
// whether the value was found.
var formGetters = []func(name string, p Params) (string, bool){
	sourceForm: func(name string, p Params) (string, bool) {
		vs := p.Form[name]
		if len(vs) == 0 {
			return "", false
		}
		return vs[0], true
	},
	sourcePath: func(name string, p Params) (string, bool) {
		for _, pv := range p.PathVar {
			if pv.Key == name {
				return pv.Value, true
			}
		}
		return "", false
	},
	sourceBody: nil,
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func implementsTextUnmarshaler(t reflect.Type) bool {
	// Use the pointer type, because a pointer
	// type will implement a superset of the methods
	// of a non-pointer type.
	return reflect.PtrTo(t).Implements(textUnmarshalerType)
}

// unmarshalWithUnmarshalText returns an unmarshaler
// that unmarshals the given type from the given tag
// using its UnmarshalText method.
func unmarshalWithUnmarshalText(t reflect.Type, tag tag) unmarshaler {
	getVal := formGetters[tag.source]
	if getVal == nil {
		panic("unexpected source")
	}
	return func(v reflect.Value, p Params, makeResult resultMaker) error {
		val, _ := getVal(tag.name, p)
		uv := makeResult(v).Addr().Interface().(encoding.TextUnmarshaler)
		return uv.UnmarshalText([]byte(val))
	}
}

// unmarshalWithScan returns an unmarshaler
// that unmarshals the given tag using fmt.Scan.
func unmarshalWithScan(tag tag) unmarshaler {
	formGet := formGetters[tag.source]
	if formGet == nil {
		panic("unexpected source")
	}
	return func(v reflect.Value, p Params, makeResult resultMaker) error {
		val, ok := formGet(tag.name, p)
		if !ok {
			// TODO allow specifying that a field is mandatory?
			return nil
		}
		_, err := fmt.Sscan(val, makeResult(v).Addr().Interface())
		if err != nil {
			return errgo.Notef(err, "cannot parse %q into %s", val, v.Type())
		}
		return nil
	}
}

type tagSource uint8

const (
	sourceNone = iota
	sourcePath
	sourceForm
	sourceBody
)

type tag struct {
	name   string
	source tagSource
}

// parseTag parses the given struct tag attached to the given
// field name into a tag structure.
func parseTag(rtag reflect.StructTag, fieldName string) (tag, error) {
	t := tag{
		name: fieldName,
	}
	tagStr := rtag.Get("httprequest")
	if tagStr == "" {
		return t, nil
	}
	fields := strings.Split(tagStr, ",")
	if fields[0] != "" {
		t.name = fields[0]
	}
	for _, f := range fields[1:] {
		switch f {
		case "path":
			t.source = sourcePath
		case "form":
			t.source = sourceForm
		case "body":
			t.source = sourceBody
		default:
			return tag{}, fmt.Errorf("unknown tag flag %q", f)
		}
	}
	return t, nil
}

// fields returns all the fields in the given struct type
// including fields inside anonymous struct members.
// The fields are ordered with top level fields first
// followed by the members of those fields
// for anonymous fields.
func fields(t reflect.Type) []reflect.StructField {
	byName := make(map[string]reflect.StructField)
	addFields(t, byName, nil)
	fields := make(fieldsByIndex, 0, len(byName))
	for _, f := range byName {
		if f.Name != "" {
			fields = append(fields, f)
		}
	}
	sort.Sort(fields)
	return fields
}

func addFields(t reflect.Type, byName map[string]reflect.StructField, index []int) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		index := append(index, i)
		var add bool
		old, ok := byName[f.Name]
		switch {
		case ok && len(old.Index) == len(index):
			// Fields with the same name at the same depth
			// cancel one another out. Set the field name
			// to empty to signify that has happened.
			old.Name = ""
			byName[f.Name] = old
			add = false
		case ok:
			// Fields at less depth win.
			add = len(index) < len(old.Index)
		default:
			// The field did not previously exist.
			add = true
		}
		if add {
			// copy the index so that it's not overwritten
			// by the other appends.
			f.Index = append([]int(nil), index...)
			byName[f.Name] = f
		}
		if f.Anonymous {
			if f.Type.Kind() == reflect.Ptr {
				f.Type = f.Type.Elem()
			}
			if f.Type.Kind() == reflect.Struct {
				addFields(f.Type, byName, index)
			}
		}
	}
}

type fieldsByIndex []reflect.StructField

func (f fieldsByIndex) Len() int {
	return len(f)
}

func (f fieldsByIndex) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f fieldsByIndex) Less(i, j int) bool {
	indexi, indexj := f[i].Index, f[j].Index
	for len(indexi) != 0 && len(indexj) != 0 {
		ii, ij := indexi[0], indexj[0]
		if ii != ij {
			return ii < ij
		}
		indexi, indexj = indexi[1:], indexj[1:]
	}
	return len(indexi) < len(indexj)
}
