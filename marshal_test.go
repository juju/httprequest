// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"io/ioutil"
	"net/http"

	gc "gopkg.in/check.v1"
	"gopkg.in/errgo.v1"

	"github.com/juju/httprequest"
)

type marshalSuite struct{}

var _ = gc.Suite(&marshalSuite{})

type embedded struct {
	F1 string  `json:"name"`
	F2 int     `json:"age"`
	F3 *string `json:"address"`
}

var marshalTests = []struct {
	about           string
	urlString       string
	method          string
	val             interface{}
	expectURLString string
	expectBody      *string
	expectHeader    http.Header
	expectError     string
}{{
	about:     "struct with simple fields",
	urlString: "http://localhost:8081/:F1",
	val: &struct {
		F1 int    `httprequest:",path"`
		F2 string `httprequest:",form"`
	}{
		F1: 99,
		F2: "some text",
	},
	expectURLString: "http://localhost:8081/99?F2=some+text",
}, {
	about:     "struct with renamed fields",
	urlString: "http://localhost:8081/:name",
	val: &struct {
		F1 string `httprequest:"name,path"`
		F2 int    `httprequest:"age,form"`
	}{
		F1: "some random user",
		F2: 42,
	},
	expectURLString: "http://localhost:8081/some%20random%20user?age=42",
}, {
	about:     "fields without httprequest tags are ignored",
	urlString: "http://localhost:8081/:name",
	val: &struct {
		F1 string `httprequest:"name,path"`
		F2 int    `httprequest:"age,form"`
		F3 string
	}{
		F1: "some random user",
		F2: 42,
		F3: "some more random text",
	},
	expectURLString: "http://localhost:8081/some%20random%20user?age=42",
}, {
	about:     "pointer fields are correctly handled",
	urlString: "http://localhost:8081/:name",
	val: &struct {
		F1 *string `httprequest:"name,path"`
		F2 *string `httprequest:"age,form"`
		F3 *string `httprequest:"address,form"`
	}{
		F1: newString("some random user"),
		F2: newString("42"),
	},
	expectURLString: "http://localhost:8081/some%20random%20user?age=42",
}, {
	about:     "MarshalText called on TextMarshalers",
	urlString: "http://localhost:8081/:param1/:param2",
	val: &struct {
		F1 testMarshaler  `httprequest:"param1,path"`
		F2 *testMarshaler `httprequest:"param2,path"`
		F3 testMarshaler  `httprequest:"param3,form"`
		F4 *testMarshaler `httprequest:"param4,form"`
	}{
		F1: "test1",
		F2: (*testMarshaler)(newString("test2")),
		F3: "test3",
		F4: (*testMarshaler)(newString("test4")),
	},
	expectURLString: "http://localhost:8081/test_test1/test_test2?param3=test_test3&param4=test_test4",
}, {
	about:     "MarshalText not called on values that do not implement TextMarshaler",
	urlString: "http://localhost:8081/user/:name/:surname",
	val: &struct {
		F1 notTextMarshaler  `httprequest:"name,path"`
		F2 *notTextMarshaler `httprequest:"surname,path"`
	}{
		F1: "name",
		F2: (*notTextMarshaler)(newString("surname")),
	},
	expectURLString: "http://localhost:8081/user/name/surname",
}, {
	about:     "MarshalText returns an error",
	urlString: "http://localhost:8081/user/:name/:surname",
	val: &struct {
		F1 testMarshaler  `httprequest:"name,path"`
		F2 *testMarshaler `httprequest:"surname,path"`
	}{
		F1: "",
		F2: (*testMarshaler)(newString("surname")),
	},
	expectError: "cannot marshal field: empty string",
}, {
	about:     "[]string field form value",
	urlString: "http://localhost:8081/user",
	val: &struct {
		F1 []string `httprequest:"users,form"`
	}{
		F1: []string{"user1", "user2", "user3"},
	},
	expectURLString: "http://localhost:8081/user?users=user1&users=user2&users=user3",
}, {
	about:     "nil []string field form value",
	urlString: "http://localhost:8081/user",
	val: &struct {
		F1 *[]string `httprequest:"users,form"`
	}{
		F1: nil,
	},
	expectURLString: "http://localhost:8081/user",
}, {
	about:     "cannot marshal []string field to path",
	urlString: "http://localhost:8081/:users",
	val: &struct {
		F1 []string `httprequest:"users,path"`
	}{
		F1: []string{"user1", "user2"},
	},
	expectError: `bad type \*struct { F1 \[\]string "httprequest:\\"users,path\\"" }: invalid target type \[\]string for path parameter`,
}, {
	about:     "[]string field fails to marshal to path",
	urlString: "http://localhost:8081/user/:users",
	val: &struct {
		F1 []string `httprequest:"users,path"`
	}{
		F1: []string{"user1", "user2", "user3"},
	},
	expectError: "bad type .*: invalid target type.*",
}, {
	about:     "more than one field with body tag",
	urlString: "http://localhost:8081/user",
	method:    "POST",
	val: &struct {
		F1 string `httprequest:"user,body"`
		F2 int    `httprequest:"age,body"`
	}{
		F1: "test user",
		F2: 42,
	},
	expectError: "bad type .*: more than one body field specified",
}, {
	about:     "required path parameter, but not specified",
	urlString: "http://localhost:8081/u/:username",
	method:    "POST",
	val: &struct {
		F1 string `httprequest:"user,body"`
	}{
		F1: "test user",
	},
	expectError: `missing value for path parameter "username"`,
}, {
	about:     "marshal to body",
	urlString: "http://localhost:8081/u",
	method:    "POST",
	val: &struct {
		F1 embedded `httprequest:"info,body"`
	}{
		F1: embedded{
			F1: "test user",
			F2: 42,
			F3: newString("test address"),
		},
	},
	expectBody: newString(`{"name":"test user","age":42,"address":"test address"}`),
}, {
	about:     "empty path wildcard",
	urlString: "http://localhost:8081/u/:",
	method:    "POST",
	val: &struct {
		F1 string `httprequest:"user,body"`
	}{
		F1: "test user",
	},
	expectError: "empty path parameter",
}, {
	about:     "nil field to form",
	urlString: "http://localhost:8081/u",
	val: &struct {
		F1 *string `httprequest:"user,form"`
	}{},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "nil field to path",
	urlString: "http://localhost:8081/u",
	val: &struct {
		F1 *string `httprequest:"user,path"`
	}{},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "marshal to body of a GET request",
	urlString: "http://localhost:8081/u",
	val: &struct {
		F1 string `httprequest:",body"`
	}{
		F1: "hello test",
	},
	// Providing a body to a GET request is unusual but
	// some people do it anyway.

	expectBody: newString(`"hello test"`),
}, {
	about:     "marshal to nil value to body",
	urlString: "http://localhost:8081/u",
	val: &struct {
		F1 *string `httprequest:",body"`
	}{
		F1: nil,
	},
	expectBody: newString(""),
}, {
	about:     "nil TextMarshaler",
	urlString: "http://localhost:8081/u",
	val: &struct {
		F1 *testMarshaler `httprequest:"surname,form"`
	}{
		F1: (*testMarshaler)(nil),
	},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "marshal nil with Sprint",
	urlString: "http://localhost:8081/u",
	val: &struct {
		F1 *int `httprequest:"surname,form"`
	}{
		F1: (*int)(nil),
	},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "marshal to path with * placeholder",
	urlString: "http://localhost:8081/u/*name",
	val: &struct {
		F1 string `httprequest:"name,path"`
	}{
		F1: "/test",
	},
	expectURLString: "http://localhost:8081/u/test",
}, {
	about:     "marshal to path with * placeholder, but the marshaled value does not start with /",
	urlString: "http://localhost:8081/u/*name",
	val: &struct {
		F1 string `httprequest:"name,path"`
	}{
		F1: "test",
	},
	expectError: `value \"test\" for path parameter \"\*name\" does not start with required /`,
}, {
	about:     "* placeholder allowed only at the end",
	urlString: "http://localhost:8081/u/*name/document",
	val: &struct {
		F1 string `httprequest:"name,path"`
	}{
		F1: "test",
	},
	expectError: "star path parameter is not at end of path",
}, {
	about:     "unparsable base url string",
	urlString: "%%",
	val: &struct {
		F1 string `httprequest:"name,form"`
	}{
		F1: "test",
	},
	expectError: `parse %%: invalid URL escape \"%%\"`,
}, {
	about:     "value cannot be marshaled to json",
	urlString: "http://localhost",
	method:    "POST",
	val: &struct {
		F1 failJSONMarshaler `httprequest:"field,body"`
	}{
		F1: "test",
	},
	expectError: `cannot marshal field: cannot marshal request body: json: error calling MarshalJSON for type \*httprequest_test.failJSONMarshaler: marshal error`,
}, {
	about:     "url with query parameters",
	urlString: "http://localhost?a=b",
	method:    "POST",
	val: &struct {
		F1 failJSONMarshaler `httprequest:"f1,form"`
	}{
		F1: "test",
	},
	expectURLString: "http://localhost?a=b&f1=test",
}, {
	about:           "url with query parameters no form",
	urlString:       "http://localhost?a=b",
	method:          "POST",
	val:             &struct{}{},
	expectURLString: "http://localhost?a=b",
}, {
	about:     "struct with headers",
	urlString: "http://localhost:8081/",
	val: &struct {
		F1 string `httprequest:",header"`
		F2 int    `httprequest:",header"`
		F3 bool   `httprequest:",header"`
	}{
		F1: "some text",
		F2: 99,
		F3: true,
	},
	expectURLString: "http://localhost:8081/",
	expectHeader: http.Header{
		"F1": []string{"some text"},
		"F2": []string{"99"},
		"F3": []string{"true"},
	},
}, {
	about:     "struct with header slice",
	urlString: "http://localhost:8081/:F1",
	val: &struct {
		F1 int      `httprequest:",path"`
		F2 string   `httprequest:",form"`
		F3 []string `httprequest:",header"`
	}{
		F1: 99,
		F2: "some text",
		F3: []string{"A", "B", "C"},
	},
	expectURLString: "http://localhost:8081/99?F2=some+text",
	expectHeader:    http.Header{"F3": []string{"A", "B", "C"}},
}, {
	about:     "SetHeader called after marshaling",
	urlString: "http://localhost:8081/",
	val: &httprequest.CustomHeader{
		Body: &struct {
			F1 string `httprequest:",header"`
			F2 int    `httprequest:",header"`
			F3 bool   `httprequest:",header"`
		}{
			F1: "some text",
			F2: 99,
			F3: false,
		},
		SetHeaderFunc: func(h http.Header) {
			h.Set("F2", "some other text")
		},
	},
	expectURLString: "http://localhost:8081/",
	expectHeader: http.Header{
		"F1": []string{"some text"},
		"F2": []string{"some other text"},
		"F3": []string{"false"},
	},
}}

func getStruct() interface{} {
	return &struct {
		F1 string
	}{
		F1: "hello",
	}
}

func (*marshalSuite) TestMarshal(c *gc.C) {
	for i, test := range marshalTests {
		c.Logf("%d: %s", i, test.about)
		method := "GET"
		if test.method != "" {
			method = test.method
		}
		req, err := httprequest.Marshal(test.urlString, method, test.val)
		if test.expectError != "" {
			c.Assert(err, gc.ErrorMatches, test.expectError)
			continue
		}
		c.Assert(err, gc.IsNil)
		if test.expectURLString != "" {
			c.Assert(req.URL.String(), gc.DeepEquals, test.expectURLString)
		}
		if test.expectBody != nil {
			data, err := ioutil.ReadAll(req.Body)
			c.Assert(err, gc.IsNil)
			if *test.expectBody != "" {
				c.Assert(req.Header.Get("Content-Type"), gc.Equals, "application/json")
			}
			c.Assert(string(data), gc.DeepEquals, *test.expectBody)
		}
		for k, v := range test.expectHeader {
			c.Assert(req.Header[k], gc.DeepEquals, v)
		}
	}
}

type testMarshaler string

func (t *testMarshaler) MarshalText() ([]byte, error) {
	if len(*t) == 0 {
		return nil, errgo.New("empty string")
	}
	return []byte("test_" + *t), nil
}

type notTextMarshaler string

// MarshalText does *not* implement encoding.TextMarshaler
func (t *notTextMarshaler) MarshalText() {
	panic("unexpected call")
}

type failJSONMarshaler string

func (*failJSONMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errgo.New("marshal error")
}
