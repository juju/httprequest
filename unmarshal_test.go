// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	jc "github.com/juju/testing/checkers"
	"github.com/julienschmidt/httprouter"
	gc "gopkg.in/check.v1"

	"github.com/juju/httprequest"
)

type unmarshalSuite struct{}

var _ = gc.Suite(&unmarshalSuite{})

var unmarshalTests = []struct {
	about       string
	val         interface{}
	expect      interface{}
	params      httprequest.Params
	expectError string
	// TODO expectErrorCause func(error) bool
}{{
	about: "struct with simple fields",
	val: struct {
		F1          int    `httprequest:",form"`
		F2          int    `httprequest:",form"`
		G1          string `httprequest:",path"`
		G2          string `httprequest:",path"`
		H           string `httprequest:",body"`
		UnknownForm string `httprequest:",form"`
		UnknownPath string `httprequest:",path"`
	}{
		F1: 99,
		F2: -35,
		G1: "g1 val",
		G2: "g2 val",
		H:  "h val",
	},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"application/json"}},
			Form: url.Values{
				"F1": {"99"},
				"F2": {"-35", "not a number"},
			},
			Body: body(`"h val"`),
		},
		PathVar: httprouter.Params{{
			Key:   "G2",
			Value: "g2 val",
		}, {
			Key:   "G1",
			Value: "g1 val",
		}, {
			Key:   "G1",
			Value: "g1 wrong val",
		}},
	},
}, {
	about: "struct with renamed fields",
	val: struct {
		F1 int    `httprequest:"x1,form"`
		F2 int    `httprequest:"x2,form"`
		G1 string `httprequest:"g1,path"`
		G2 string `httprequest:"g2,path"`
	}{
		F1: 99,
		F2: -35,
		G1: "g1 val",
		G2: "g2 val",
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"x1": {"99"},
				"x2": {"-35", "not a number"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "g2",
			Value: "g2 val",
		}, {
			Key:   "g1",
			Value: "g1 val",
		}, {
			Key:   "g1",
			Value: "g1 wrong val",
		}},
	},
}, {
	about: "unexported fields are ignored",
	val: struct {
		f int `httprequest:",form"`
		G int `httprequest:",form"`
	}{
		G: 99,
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"G": {"99"},
				"f": {"100"},
			},
		},
	},
}, {
	about: "unexported embedded type works ok",
	val: struct {
		sFG
	}{
		sFG: sFG{
			F: 99,
			G: 100,
		},
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"99"},
				"G": {"100"},
			},
		},
	},
}, {
	about: "unexported embedded type for body works ok",
	val: struct {
		sFG `httprequest:",body"`
	}{
		sFG: sFG{
			F: 99,
			G: 100,
		},
	},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   body(`{"F": 99, "G": 100}`),
		},
	},
}, {
	about: "unexported type for body is ignored",
	val: struct {
		foo sFG `httprequest:",body"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   body(`{"F": 99, "G": 100}`),
		},
	},
}, {
	about: "fields without httprequest tags are ignored",
	val: struct {
		F int
	}{},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"foo"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "F",
			Value: "foo",
		}},
	},
}, {
	about: "pointer fields are filled out",
	val: struct {
		F *int `httprequest:",form"`
		*SFG
		S *string `httprequest:",form"`
		T *string `httprequest:",form"`
	}{
		F: newInt(99),
		SFG: &SFG{
			F: 0,
			G: 534,
		},
		S: newString("s val"),
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"99"},
				"G": {"534"},
				"S": {"s val"},
			},
		},
	},
}, {
	about: "UnmarshalText called on TextUnmarshalers",
	val: struct {
		F  exclamationUnmarshaler  `httprequest:",form"`
		G  exclamationUnmarshaler  `httprequest:",path"`
		FP *exclamationUnmarshaler `httprequest:",form"`
	}{
		F:  "yes!",
		G:  "no!",
		FP: (*exclamationUnmarshaler)(newString("maybe!")),
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"F":  {"yes"},
				"FP": {"maybe"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "G",
			Value: "no",
		}},
	},
}, {
	about: "UnmarshalText not called on values with a non-TextUnmarshaler UnmarshalText method",
	val: struct {
		F notTextUnmarshaler `httprequest:",form"`
	}{
		F: "hello",
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"hello"},
			},
		},
	},
}, {
	about: "UnmarshalText returning an error",
	val: struct {
		F exclamationUnmarshaler `httprequest:",form"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{},
	},
	expectError: "cannot unmarshal into field: empty string!",
}, {
	about: "all field form values",
	val: struct {
		A []string  `httprequest:",form"`
		B *[]string `httprequest:",form"`
		C []string  `httprequest:",form"`
		D *[]string `httprequest:",form"`
	}{
		A: []string{"a1", "a2"},
		B: func() *[]string {
			x := []string{"b1", "b2", "b3"}
			return &x
		}(),
	},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"A": {"a1", "a2"},
				"B": {"b1", "b2", "b3"},
			},
		},
	},
}, {
	about: "invalid scan field",
	val: struct {
		A int `httprequest:",form"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"A": {"not an int"},
			},
		},
	},
	expectError: `cannot unmarshal into field: cannot parse "not an int" into int: expected integer`,
}, {
	about: "scan field not present",
	val: struct {
		A int `httprequest:",form"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{},
	},
}, {
	about: "invalid JSON body",
	val: struct {
		A string `httprequest:",body"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   body("invalid JSON"),
		},
	},
	expectError: "cannot unmarshal into field: cannot unmarshal request body: invalid character 'i' looking for beginning of value",
}, {
	about: "body with read error",
	val: struct {
		A string `httprequest:",body"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   errorReader("some error"),
		},
	},
	expectError: "cannot unmarshal into field: cannot read request body: some error",
}, {
	about: "[]string not allowed for URL source",
	val: struct {
		A []string `httprequest:",path"`
	}{},
	expectError: `bad type .*: invalid target type \[]string for path parameter`,
}, {
	about: "duplicated body",
	val: struct {
		B1 int    `httprequest:",body"`
		B2 string `httprequest:",body"`
	}{},
	expectError: "bad type .*: more than one body field specified",
}, {
	about: "body tag name is ignored",
	val: struct {
		B string `httprequest:"foo,body"`
	}{
		B: "hello",
	},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   body(`"hello"`),
		},
	},
}, {
	about: "tag with invalid source",
	val: struct {
		B1 int `httprequest:",xxx"`
	}{},
	expectError: `bad type .*: bad tag "httprequest:\\",xxx\\"" in field B1: unknown tag flag "xxx"`,
}, {
	about:       "non-struct pointer",
	val:         0,
	expectError: `bad type \*int: type is not pointer to struct`,
}, {
	about: "unmarshaling with wrong request content type",
	val: struct {
		A string `httprequest:",body"`
	}{},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{"Content-Type": {"text/html"}},
			Body:   body("invalid JSON"),
		},
	},
	expectError: `cannot unmarshal into field: unexpected content type text/html; want application/json; content: invalid JSON`,
}, {
	about: "struct with header fields",
	val: struct {
		F1 int    `httprequest:"x1,header"`
		G1 string `httprequest:"g1,header"`
	}{
		F1: 99,
		G1: "g1 val",
	},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{
				"x1": {"99"},
				"g1": {"g1 val"},
			},
		},
	},
}, {
	about: "all field header values",
	val: struct {
		A []string  `httprequest:",header"`
		B *[]string `httprequest:",header"`
		C []string  `httprequest:",header"`
		D *[]string `httprequest:",header"`
	}{
		A: []string{"a1", "a2"},
		B: func() *[]string {
			x := []string{"b1", "b2", "b3"}
			return &x
		}(),
	},
	params: httprequest.Params{
		Request: &http.Request{
			Header: http.Header{
				"A": {"a1", "a2"},
				"B": {"b1", "b2", "b3"},
			},
		},
	},
}}

type SFG struct {
	F int `httprequest:",form"`
	G int `httprequest:",form"`
}

type sFG struct {
	F int `httprequest:",form"`
	G int `httprequest:",form"`
}

func (*unmarshalSuite) TestUnmarshal(c *gc.C) {
	for i, test := range unmarshalTests {
		c.Logf("%d: %s", i, test.about)
		t := reflect.TypeOf(test.val)
		fillv := reflect.New(t)
		err := httprequest.Unmarshal(test.params, fillv.Interface())
		if test.expectError != "" {
			c.Assert(err, gc.ErrorMatches, test.expectError)
			continue
		}
		c.Assert(fillv.Elem().Interface(), jc.DeepEquals, test.val)
	}
}

// TODO non-pointer struct

type notTextUnmarshaler string

// UnmarshalText does *not* implement encoding.TextUnmarshaler
// (it has no arguments or error return value)
func (t *notTextUnmarshaler) UnmarshalText() {
	panic("unexpected call")
}

type exclamationUnmarshaler string

func (t *exclamationUnmarshaler) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("empty string!")
	}
	*t = exclamationUnmarshaler(b) + "!"
	return nil
}

func newInt(i int) *int {
	return &i
}

func newString(s string) *string {
	return &s
}

type errorReader string

func (r errorReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("%s", r)
}

func (r errorReader) Close() error {
	return nil
}

func body(s string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(s))
}
