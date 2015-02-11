// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/errgo.v1"
)

// Marshal takes the input structure and creates an http request.
//
// See: Unmarshal for more details.
//
// For fields with a "path" item in the structural tag, the base uri must
// contain a placeholder with its name.
// Example:
// For
//    type Test struct {
//	    username string `httprequest:"user,path"`
//    }
// ...the request url must contain a ":user" placeholder:
//    http://localhost:8081/:user/files
//
// If url path contains a placeholder, but the input struct does not marshal
// to that placeholder, an error is raised.
func Marshal(baseURL, method string, x interface{}) (*http.Request, error) {
	xv := reflect.ValueOf(x)
	pt, err := getRequestType(xv.Type())
	if err != nil {
		return nil, errgo.WithCausef(err, ErrBadUnmarshalType, "bad type %s", xv.Type())
	}
	req, err := http.NewRequest(method, baseURL, bytes.NewBuffer(nil))
	if err != nil {
		return nil, errgo.Mask(err)
	}

	p := &Params{
		Request: req,
	}

	if err := marshal(p, xv, pt); err != nil {
		return nil, errgo.Mask(err, errgo.Is(ErrUnmarshal))
	}
	return p.Request, nil
}

// marshal is the internal version of Marshal.
func marshal(p *Params, xv reflect.Value, pt *requestType) error {
	xv = xv.Elem()

	for _, f := range pt.fields {
		fv := xv.FieldByIndex(f.index)

		// TODO store the field name in the field so
		// that we can produce a nice error message.
		if err := f.marshal(fv, p); err != nil {
			return errgo.WithCausef(err, ErrUnmarshal, "cannot marshal field")
		}
	}

	path := p.URL.Path
	var pathBuffer bytes.Buffer
	paramsByName := make(map[string]string)
	for _, param := range p.PathVar {
		paramsByName[param.Key] = param.Value
	}

	offset := 0
	hasParams := false
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c != ':' && c != '*' {
			continue
		}
		hasParams = true

		end := i + 1
		for end < len(path) && path[end] != ':' && path[end] != '/' {
			end++
		}

		if c == '*' && end != len(path) {
			return errgo.New("placeholders starting with * are only allowed at the end")
		}

		if end-i < 2 {
			return errgo.New("request wildcards must be named with a non-empty name")
		}
		if i > 0 {
			pathBuffer.WriteString(path[offset:i])
		}

		wildcard := path[i+1 : end]
		paramValue, ok := paramsByName[wildcard]
		if !ok {
			return errgo.Newf("missing value for path parameter %q", wildcard)
		}
		pathBuffer.WriteString(paramValue)
		offset = end
	}
	if !hasParams {
		pathBuffer.WriteString(path)
	}

	p.URL.Path = pathBuffer.String()

	p.URL.RawQuery = p.Form.Encode()

	return nil
}

// getMarshaler returns a marshaler function suitable for marshaling
// a field with the given tag into and http request.
func getMarshaler(tag tag, t reflect.Type) (marshaler, error) {
	switch {
	case tag.source == sourceNone:
		return marshalNop, nil
	case tag.source == sourceBody:
		return marshalBody, nil
	case t == reflect.TypeOf([]string(nil)):
		if tag.source != sourceForm {
			return nil, errgo.New("invalid target type []string for path parameter")
		}
		return marshalAllField(tag.name), nil
	case t == reflect.TypeOf(""):
		return marshalString(tag), nil
	case implementsTextMarshaler(t):
		return marshalWithMarshalText(t, tag), nil
	default:
		return marshalWithSprint(tag), nil
	}
}

func dereferenceIfPointer(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		return v.Elem()
	}
	return v
}

// marshalNop does nothing with the value.
func marshalNop(v reflect.Value, p *Params) error {
	return nil
}

// mashalBody marshals the specified value into the body of the http request.
func marshalBody(v reflect.Value, p *Params) error {
	// TODO allow body types that aren't necessarily JSON.
	bodyValue := dereferenceIfPointer(v)
	if bodyValue == emptyValue {
		return nil
	}

	if p.Method != "POST" && p.Method != "PUT" {
		return errgo.Newf("trying to marshal to body of a request with method %q", p.Method)
	}

	data, err := json.Marshal(bodyValue.Interface())
	if err != nil {
		return errgo.Notef(err, "cannot marshal request body")
	}
	p.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	return nil
}

// marshalAllField marshals a []string slice into form fields.
func marshalAllField(name string) marshaler {
	return func(v reflect.Value, p *Params) error {
		value := dereferenceIfPointer(v)
		if value == emptyValue {
			return nil
		}
		values := value.Interface().([]string)
		if p.Form == nil {
			p.Form = url.Values{}
		}
		for _, value := range values {
			p.Form.Add(name, value)
		}
		return nil
	}
}

// marshalString marshals s string field.
func marshalString(tag tag) marshaler {
	formSet := formSetters[tag.source]
	if formSet == nil {
		panic("unexpected source")
	}
	return func(v reflect.Value, p *Params) error {
		value := dereferenceIfPointer(v)
		if value == emptyValue {
			return nil
		}
		formSet(tag.name, value.String(), p)
		return nil
	}
}

var textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

func implementsTextMarshaler(t reflect.Type) bool {
	// Use the pointer type, because a pointer
	// type will implement a superset of the methods
	// of a non-pointer type.
	return reflect.PtrTo(t).Implements(textMarshalerType)
}

// marshalWithMarshalText returns a marshaler
// that marshals the given type from the given tag
// using its MarshalText method.
func marshalWithMarshalText(t reflect.Type, tag tag) marshaler {
	formSet := formSetters[tag.source]
	if formSet == nil {
		panic("unexpected source")
	}
	return func(v reflect.Value, p *Params) error {
		value := dereferenceIfPointer(v)
		if value == emptyValue {
			return nil
		}
		m := value.Addr().Interface().(encoding.TextMarshaler)
		data, err := m.MarshalText()
		if err != nil {
			return errgo.Mask(err)
		}
		formSet(tag.name, string(data), p)

		return nil
	}
}

// marshalWithScan returns an marshaler
// that unmarshals the given tag using fmt.Sprint.
func marshalWithSprint(tag tag) marshaler {
	formSet := formSetters[tag.source]
	if formSet == nil {
		panic("unexpected source")
	}
	return func(v reflect.Value, p *Params) error {
		value := dereferenceIfPointer(v)
		if value == emptyValue {
			return nil
		}
		valueString := fmt.Sprint(value.Interface())

		formSet(tag.name, valueString, p)

		return nil
	}
}

// formSetters maps from source to a function that
// sets the value for a given key.
var formSetters = []func(string, string, *Params){
	sourceForm: func(name, value string, p *Params) {
		if p.Form == nil {
			p.Form = url.Values{}
		}
		p.Form.Add(name, value)
	},
	sourcePath: func(name, value string, p *Params) {
		p.PathVar = append(p.PathVar, httprouter.Param{Key: name, Value: value})
	},
	sourceBody: nil,
}
