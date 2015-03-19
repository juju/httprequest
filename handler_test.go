// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	jc "github.com/juju/testing/checkers"
	"github.com/juju/testing/httptesting"
	"github.com/julienschmidt/httprouter"
	gc "gopkg.in/check.v1"
	"gopkg.in/errgo.v1"

	"github.com/juju/httprequest"
)

type handlerSuite struct{}

var _ = gc.Suite(&handlerSuite{})

var handleTests = []struct {
	about        string
	f            func(c *gc.C) interface{}
	req          *http.Request
	pathVar      httprouter.Params
	expectBody   interface{}
	expectStatus int
}{{
	about: "function with no return",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A string         `httprequest:"a,path"`
			B map[string]int `httprequest:",body"`
			C int            `httprequest:"c,form"`
		}
		return func(p httprequest.Params, s *testStruct) {
			c.Assert(s, jc.DeepEquals, &testStruct{
				A: "A",
				B: map[string]int{"hello": 99},
				C: 43,
			})
			c.Assert(p.PathVar, jc.DeepEquals, httprouter.Params{{
				Key:   "a",
				Value: "A",
			}})
			c.Assert(p.Request.Form, jc.DeepEquals, url.Values{
				"c": {"43"},
			})
			p.Response.Header().Set("Content-Type", "application/json")
			p.Response.Write([]byte("true"))
		}
	},
	req: &http.Request{
		Header: http.Header{"Content-Type": {"application/json"}},
		Form: url.Values{
			"c": {"43"},
		},
		Body: body(`{"hello": 99}`),
	},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "A",
	}},
	expectBody: true,
}, {
	about: "function with error return that returns no error",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) error {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			p.Response.Header().Set("Content-Type", "application/json")
			p.Response.Write([]byte("true"))
			return nil
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
	expectBody: true,
}, {
	about: "function with error return that returns an error",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) error {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			return errUnauth
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
	expectBody: errorResponse{
		Message: errUnauth.Error(),
	},
	expectStatus: http.StatusUnauthorized,
}, {
	about: "function with value return that returns a value",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) (int, error) {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			return 1234, nil
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
	expectBody: 1234,
}, {
	about: "function with value return that returns an error",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) (int, error) {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			return 0, errUnauth
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
	expectBody: errorResponse{
		Message: errUnauth.Error(),
	},
	expectStatus: http.StatusUnauthorized,
}, {
	about: "error when unmarshaling",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) (int, error) {
			c.Errorf("function should not have been called")
			return 0, nil
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "not a number",
	}},
	expectBody: errorResponse{
		Message: `cannot unmarshal parameters: cannot unmarshal into field: cannot parse "not a number" into int: expected integer`,
	},
	expectStatus: http.StatusBadRequest,
}, {
	about: "error when unmarshaling single value return",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) error {
			c.Errorf("function should not have been called")
			return nil
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "not a number",
	}},
	expectBody: errorResponse{
		Message: `cannot unmarshal parameters: cannot unmarshal into field: cannot parse "not a number" into int: expected integer`,
	},
	expectStatus: http.StatusBadRequest,
}, {
	about: "return type that can't be marshaled as JSON",
	f: func(c *gc.C) interface{} {
		return func(p httprequest.Params, s *struct{}) (map[int]int, error) {
			return map[int]int{0: 1}, nil
		}
	},
	req:     &http.Request{},
	pathVar: httprouter.Params{},
	expectBody: errorResponse{
		Message: "json: unsupported type: map[int]int",
	},
	expectStatus: http.StatusInternalServerError,
}}

func (*handlerSuite) TestHandle(c *gc.C) {
	for i, test := range handleTests {
		c.Logf("%d: %s", i, test.about)
		h := errorMapper.Handle(test.f(c))
		rec := httptest.NewRecorder()
		h(rec, test.req, test.pathVar)
		if test.expectStatus == 0 {
			test.expectStatus = http.StatusOK
		}
		httptesting.AssertJSONResponse(c, rec, test.expectStatus, test.expectBody)
	}
}

var handlePanicTests = []struct {
	f      interface{}
	expect string
}{{
	f:      42,
	expect: "bad handler function: not a function",
}, {
	f:      func(httprequest.Params) {},
	expect: "bad handler function: has 1 parameters, need 2",
}, {
	f:      func(httprequest.Params, *struct{}, struct{}) {},
	expect: "bad handler function: has 3 parameters, need 2",
}, {
	f:      func(httprequest.Params, *struct{}) struct{} { return struct{}{} },
	expect: "bad handler function: final result parameter is struct {}, need error",
}, {
	f: func(http.ResponseWriter, httprequest.Params) (struct{}, error) {
		return struct{}{}, nil
	},
	expect: "bad handler function: first argument is http.ResponseWriter, need httprequest.Params",
}, {
	f: func(httprequest.Params, *struct{}) (struct{}, struct{}) {
		return struct{}{}, struct{}{}
	},
	expect: "bad handler function: final result parameter is struct {}, need error",
}, {
	f:      func(*http.Request, *struct{}) {},
	expect: `bad handler function: first argument is \*http.Request, need httprequest.Params`,
}, {
	f:      func(httprequest.Params, struct{}) {},
	expect: "bad handler function: second argument cannot be used for Unmarshal: type is not pointer to struct",
}, {
	f: func(httprequest.Params, *struct {
		A int `httprequest:"a,the-ether"`
	}) {
	},
	expect: `bad handler function: second argument cannot be used for Unmarshal: bad tag "httprequest:\\"a,the-ether\\"" in field A: unknown tag flag "the-ether"`,
}, {
	f:      func(httprequest.Params, *struct{}) (a, b, c struct{}) { return },
	expect: `bad handler function: has 3 result parameters, need 0, 1 or 2`,
}}

func (*handlerSuite) TestHandlePanicsWithBadFunctions(c *gc.C) {
	for i, test := range handlePanicTests {
		c.Logf("%d: %s", i, test.expect)
		c.Check(func() {
			errorMapper.Handle(test.f)
		}, gc.PanicMatches, test.expect)
	}
}

func (*handlerSuite) TestBadForm(c *gc.C) {
	h := errorMapper.Handle(func(p httprequest.Params, _ *struct{}) {
		c.Fatalf("shouldn't be called")
	})
	rec := httptest.NewRecorder()
	req := &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type": {"application/x-www-form-urlencoded"},
		},
		Body: body("%6"),
	}
	h(rec, req, httprouter.Params{})
	httptesting.AssertJSONResponse(c, rec, http.StatusBadRequest, errorResponse{Message: `cannot parse HTTP request form: invalid URL escape "%6"`})
}

func (*handlerSuite) TestToHTTP(c *gc.C) {
	var h http.Handler
	h = httprequest.ToHTTP(errorMapper.Handle(func(p httprequest.Params, s *struct{}) {
		c.Assert(p.PathVar, gc.IsNil)
		p.Response.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := &http.Request{
		Body: body(""),
	}
	h.ServeHTTP(rec, req)
	c.Assert(rec.Code, gc.Equals, http.StatusOK)
}

func (*handlerSuite) TestWriteJSON(c *gc.C) {
	rec := httptest.NewRecorder()
	type Number struct {
		N int
	}
	err := httprequest.WriteJSON(rec, http.StatusTeapot, Number{1234})
	c.Assert(err, gc.IsNil)
	c.Assert(rec.Code, gc.Equals, http.StatusTeapot)
	c.Assert(rec.Body.String(), gc.Equals, `{"N":1234}`)
	c.Assert(rec.Header().Get("content-type"), gc.Equals, "application/json")
}

var (
	errUnauth = errors.New("unauth")
	errBadReq = errors.New("bad request")
	errOther  = errors.New("other")
	errNil    = errors.New("nil result")
)

type errorResponse struct {
	Message string
}

var errorMapper httprequest.ErrorMapper = func(err error) (int, interface{}) {
	resp := &errorResponse{
		Message: err.Error(),
	}
	status := http.StatusInternalServerError
	switch errgo.Cause(err) {
	case errUnauth:
		status = http.StatusUnauthorized
	case errBadReq, httprequest.ErrUnmarshal:
		status = http.StatusBadRequest
	case errNil:
		return status, nil
	}
	return status, &resp
}

var writeErrorTests = []struct {
	err          error
	expectStatus int
	expectResp   *errorResponse
}{{
	err:          errUnauth,
	expectStatus: http.StatusUnauthorized,
	expectResp: &errorResponse{
		Message: errUnauth.Error(),
	},
}, {
	err:          errBadReq,
	expectStatus: http.StatusBadRequest,
	expectResp: &errorResponse{
		Message: errBadReq.Error(),
	},
}, {
	err:          errOther,
	expectStatus: http.StatusInternalServerError,
	expectResp: &errorResponse{
		Message: errOther.Error(),
	},
}, {
	err:          errNil,
	expectStatus: http.StatusInternalServerError,
}}

func (s *handlerSuite) TestWriteError(c *gc.C) {
	for i, test := range writeErrorTests {
		c.Logf("%d: %s", i, test.err)
		rec := httptest.NewRecorder()
		errorMapper.WriteError(rec, test.err)
		resp := parseErrorResponse(c, rec.Body.Bytes())
		c.Assert(resp, gc.DeepEquals, test.expectResp)
		c.Assert(rec.Code, gc.Equals, test.expectStatus)
	}
}

func parseErrorResponse(c *gc.C, body []byte) *errorResponse {
	var errResp *errorResponse
	err := json.Unmarshal(body, &errResp)
	c.Assert(err, gc.IsNil)
	return errResp
}

func (s *handlerSuite) TestHandleErrors(c *gc.C) {
	req := new(http.Request)
	params := httprouter.Params{}
	// Test when handler returns an error.
	handler := errorMapper.HandleErrors(func(p httprequest.Params) error {
		c.Assert(p.Request, jc.DeepEquals, req)
		c.Assert(p.PathVar, jc.DeepEquals, params)
		return errUnauth
	})
	rec := httptest.NewRecorder()
	handler(rec, req, params)
	c.Assert(rec.Code, gc.Equals, http.StatusUnauthorized)
	resp := parseErrorResponse(c, rec.Body.Bytes())
	c.Assert(resp, gc.DeepEquals, &errorResponse{
		Message: errUnauth.Error(),
	})

	// Test when handler returns nil.
	handler = errorMapper.HandleErrors(func(p httprequest.Params) error {
		c.Assert(p.Request, jc.DeepEquals, req)
		c.Assert(p.PathVar, jc.DeepEquals, params)
		p.Response.WriteHeader(http.StatusCreated)
		p.Response.Write([]byte("something"))
		return nil
	})
	rec = httptest.NewRecorder()
	handler(rec, req, params)
	c.Assert(rec.Code, gc.Equals, http.StatusCreated)
	c.Assert(rec.Body.String(), gc.Equals, "something")
}

var handleErrorsWithErrorAfterWriteHeaderTests = []struct {
	about            string
	causeWriteHeader func(w http.ResponseWriter)
}{{
	about: "write",
	causeWriteHeader: func(w http.ResponseWriter) {
		w.Write([]byte(""))
	},
}, {
	about: "write header",
	causeWriteHeader: func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
	},
}, {
	about: "flush",
	causeWriteHeader: func(w http.ResponseWriter) {
		w.(http.Flusher).Flush()
	},
}}

func (s *handlerSuite) TestHandleErrorsWithErrorAfterWriteHeader(c *gc.C) {
	for i, test := range handleErrorsWithErrorAfterWriteHeaderTests {
		c.Logf("test %d: %s", i, test.about)
		handler := errorMapper.HandleErrors(func(p httprequest.Params) error {
			test.causeWriteHeader(p.Response)
			return errgo.New("unexpected")
		})
		rec := httptest.NewRecorder()
		handler(rec, new(http.Request), nil)
		c.Assert(rec.Code, gc.Equals, http.StatusOK)
		c.Assert(rec.Body.String(), gc.Equals, "")
	}
}

func (s *handlerSuite) TestHandleJSON(c *gc.C) {
	req := new(http.Request)
	params := httprouter.Params{}
	// Test when handler returns an error.
	handler := errorMapper.HandleJSON(func(p httprequest.Params) (interface{}, error) {
		c.Assert(p.Request, jc.DeepEquals, req)
		c.Assert(p.PathVar, jc.DeepEquals, params)
		return nil, errUnauth
	})
	rec := httptest.NewRecorder()
	handler(rec, new(http.Request), params)
	resp := parseErrorResponse(c, rec.Body.Bytes())
	c.Assert(resp, gc.DeepEquals, &errorResponse{
		Message: errUnauth.Error(),
	})
	c.Assert(rec.Code, gc.Equals, http.StatusUnauthorized)

	// Test when handler returns a body.
	handler = errorMapper.HandleJSON(func(p httprequest.Params) (interface{}, error) {
		c.Assert(p.Request, jc.DeepEquals, req)
		c.Assert(p.PathVar, jc.DeepEquals, params)
		p.Response.Header().Set("Some-Header", "value")
		return "something", nil
	})
	rec = httptest.NewRecorder()
	handler(rec, req, params)
	c.Assert(rec.Code, gc.Equals, http.StatusOK)
	c.Assert(rec.Body.String(), gc.Equals, `"something"`)
	c.Assert(rec.Header().Get("Some-Header"), gc.Equals, "value")
}
