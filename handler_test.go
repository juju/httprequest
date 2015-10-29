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
	expectMethod string
	expectPath   string
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
	expectBody: httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
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
	expectBody: httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
	},
	expectStatus: http.StatusUnauthorized,
}, {
	about: "function with value return that writes to p.Response",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(p httprequest.Params, s *testStruct) (int, error) {
			_, err := p.Response.Write(nil)
			c.Assert(err, gc.ErrorMatches, "inappropriate call to ResponseWriter.Write in JSON-returning handler")
			p.Response.WriteHeader(http.StatusTeapot)
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
	about: "function with no Params and no return",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A string         `httprequest:"a,path"`
			B map[string]int `httprequest:",body"`
			C int            `httprequest:"c,form"`
		}
		return func(s *testStruct) {
			c.Assert(s, jc.DeepEquals, &testStruct{
				A: "A",
				B: map[string]int{"hello": 99},
				C: 43,
			})
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
}, {
	about: "function with no Params with error return that returns no error",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(s *testStruct) error {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			return nil
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
}, {
	about: "function with no Params with error return that returns an error",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(s *testStruct) error {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			return errUnauth
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
	expectBody: httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
	},
	expectStatus: http.StatusUnauthorized,
}, {
	about: "function with no Params with value return that returns a value",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(s *testStruct) (int, error) {
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
	about: "function with no Params with value return that returns an error",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(s *testStruct) (int, error) {
			c.Assert(s, jc.DeepEquals, &testStruct{123})
			return 0, errUnauth
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "123",
	}},
	expectBody: httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
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
	expectBody: httprequest.RemoteError{
		Message: `cannot unmarshal parameters: cannot unmarshal into field: cannot parse "not a number" into int: expected integer`,
		Code:    "bad request",
	},
	expectStatus: http.StatusBadRequest,
}, {
	about: "error when unmarshaling, no Params",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			A int `httprequest:"a,path"`
		}
		return func(s *testStruct) (int, error) {
			c.Errorf("function should not have been called")
			return 0, nil
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "a",
		Value: "not a number",
	}},
	expectBody: httprequest.RemoteError{
		Message: `cannot unmarshal parameters: cannot unmarshal into field: cannot parse "not a number" into int: expected integer`,
		Code:    "bad request",
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
	expectBody: httprequest.RemoteError{
		Message: `cannot unmarshal parameters: cannot unmarshal into field: cannot parse "not a number" into int: expected integer`,
		Code:    "bad request",
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
	expectBody: httprequest.RemoteError{
		Message: "json: unsupported type: map[int]int",
	},
	expectStatus: http.StatusInternalServerError,
}, {
	about: "argument with route",
	f: func(c *gc.C) interface{} {
		type testStruct struct {
			httprequest.Route `httprequest:"GET /foo/:bar"`
			A                 string `httprequest:"bar,path"`
		}
		return func(s *testStruct) {
			c.Check(s.A, gc.Equals, "val")
		}
	},
	req: &http.Request{},
	pathVar: httprouter.Params{{
		Key:   "bar",
		Value: "val",
	}},
	expectMethod: "GET",
	expectPath:   "/foo/:bar",
}}

func (*handlerSuite) TestHandle(c *gc.C) {
	for i, test := range handleTests {
		c.Logf("%d: %s", i, test.about)
		h := errorMapper.Handle(test.f(c))
		c.Assert(h.Method, gc.Equals, test.expectMethod)
		c.Assert(h.Path, gc.Equals, test.expectPath)
		rec := httptest.NewRecorder()
		h.Handle(rec, test.req, test.pathVar)
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
	expect: "bad handler function: no argument parameter after Params argument",
}, {
	f:      func(httprequest.Params, *struct{}, struct{}) {},
	expect: "bad handler function: has 3 parameters, need 1 or 2",
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
	expect: "bad handler function: last argument cannot be used for Unmarshal: type is not pointer to struct",
}, {
	f: func(httprequest.Params, *struct {
		A int `httprequest:"a,the-ether"`
	}) {
	},
	expect: `bad handler function: last argument cannot be used for Unmarshal: bad tag "httprequest:\\"a,the-ether\\"" in field A: unknown tag flag "the-ether"`,
}, {
	f:      func(httprequest.Params, *struct{}) (a, b, c struct{}) { return },
	expect: `bad handler function: has 3 result parameters, need 0, 1 or 2`,
}, {
	f: func(*struct {
		httprequest.Route
	}) {
	},
	expect: `bad handler function: last argument cannot be used for Unmarshal: bad route tag "": no httprequest tag`,
}, {
	f: func(*struct {
		httprequest.Route `othertag:"foo"`
	}) {
	},
	expect: `bad handler function: last argument cannot be used for Unmarshal: bad route tag "othertag:\\"foo\\"": no httprequest tag`,
}, {
	f: func(*struct {
		httprequest.Route `httprequest:""`
	}) {
	},
	expect: `bad handler function: last argument cannot be used for Unmarshal: bad route tag "httprequest:\\"\\"": no httprequest tag`,
}, {
	f: func(*struct {
		httprequest.Route `httprequest:"GET /foo /bar"`
	}) {
	},
	expect: `bad handler function: last argument cannot be used for Unmarshal: bad route tag "httprequest:\\"GET /foo /bar\\"": wrong field count`,
}, {
	f: func(*struct {
		httprequest.Route `httprequest:"BAD /foo"`
	}) {
	},
	expect: `bad handler function: last argument cannot be used for Unmarshal: bad route tag "httprequest:\\"BAD /foo\\"": invalid method`,
}}

func (*handlerSuite) TestHandlePanicsWithBadFunctions(c *gc.C) {
	for i, test := range handlePanicTests {
		c.Logf("%d: %s", i, test.expect)
		c.Check(func() {
			errorMapper.Handle(test.f)
		}, gc.PanicMatches, test.expect)
	}
}

var handlersTests = []struct {
	calledMethod string
	callParams   httptesting.JSONCallParams
}{{
	calledMethod: "M1",
	callParams: httptesting.JSONCallParams{
		URL: "/m1/99",
	},
}, {
	calledMethod: "M2",
	callParams: httptesting.JSONCallParams{
		URL:        "/m2/99",
		ExpectBody: 999,
	},
}, {
	calledMethod: "M3",
	callParams: httptesting.JSONCallParams{
		URL: "/m3/99",
		ExpectBody: &httprequest.RemoteError{
			Message: "m3 error",
		},
		ExpectStatus: http.StatusInternalServerError,
	},
}, {
	calledMethod: "M3Post",
	callParams: httptesting.JSONCallParams{
		Method:   "POST",
		URL:      "/m3/99",
		JSONBody: make(map[string]interface{}),
	},
}}

func (*handlerSuite) TestHandlers(c *gc.C) {
	handleVal := testHandlers{
		c: c,
	}
	f := func(p httprequest.Params) (*testHandlers, error) {
		handleVal.p = p
		return &handleVal, nil
	}
	handlers := errorMapper.Handlers(f)
	handlers1 := make([]httprequest.Handler, len(handlers))
	copy(handlers1, handlers)
	for i := range handlers1 {
		handlers1[i].Handle = nil
	}
	expectHandlers := []httprequest.Handler{{
		Method: "GET",
		Path:   "/m1/:p",
	}, {
		Method: "GET",
		Path:   "/m2/:p",
	}, {
		Method: "GET",
		Path:   "/m3/:p",
	}, {
		Method: "POST",
		Path:   "/m3/:p",
	}}
	c.Assert(handlers1, jc.DeepEquals, expectHandlers)
	c.Assert(handlersTests, gc.HasLen, len(expectHandlers))

	router := httprouter.New()
	for _, h := range handlers {
		c.Logf("adding %s %s", h.Method, h.Path)
		router.Handle(h.Method, h.Path, h.Handle)
	}
	for i, test := range handlersTests {
		c.Logf("test %d: %s", i, test.calledMethod)
		handleVal.calledMethod = ""
		test.callParams.Handler = router
		httptesting.AssertJSONCall(c, test.callParams)
		c.Assert(handleVal.calledMethod, gc.Equals, test.calledMethod)
	}
}

type testHandlers struct {
	calledMethod string
	c            *gc.C
	p            httprequest.Params
}

func (h *testHandlers) M1(p httprequest.Params, arg *struct {
	httprequest.Route `httprequest:"GET /m1/:p"`
	P                 int `httprequest:"p,path"`
}) {
	h.calledMethod = "M1"
	h.c.Check(arg.P, gc.Equals, 99)
	h.c.Check(p.Response, gc.Equals, h.p.Response)
	h.c.Check(p.Request, gc.Equals, h.p.Request)
	h.c.Check(p.PathVar, gc.DeepEquals, h.p.PathVar)
}

func (h *testHandlers) M2(arg *struct {
	httprequest.Route `httprequest:"GET /m2/:p"`
	P                 int `httprequest:"p,path"`
}) (int, error) {
	h.calledMethod = "M2"
	h.c.Check(arg.P, gc.Equals, 99)
	return 999, nil
}

func (h *testHandlers) unexported() {
}

func (h *testHandlers) M3(arg *struct {
	httprequest.Route `httprequest:"GET /m3/:p"`
	P                 int `httprequest:"p,path"`
}) (int, error) {
	h.calledMethod = "M3"
	h.c.Check(arg.P, gc.Equals, 99)
	return 0, errgo.New("m3 error")
}

func (h *testHandlers) M3Post(arg *struct {
	httprequest.Route `httprequest:"POST /m3/:p"`
	P                 int `httprequest:"p,path"`
}) {
	h.calledMethod = "M3Post"
	h.c.Check(arg.P, gc.Equals, 99)
}

var badHandlersFuncTests = []struct {
	f           interface{}
	expectPanic string
}{{
	f:           123,
	expectPanic: "bad handler function: expected function, got int",
}, {
	f:           (func())(nil),
	expectPanic: "bad handler function: function is nil",
}, {
	f:           func() {},
	expectPanic: "bad handler function: got 0 arguments, want 1",
}, {
	f:           func(http.ResponseWriter, *http.Request) {},
	expectPanic: "bad handler function: got 2 arguments, want 1",
}, {
	f:           func(httprequest.Params) {},
	expectPanic: "bad handler function: function returns 0 values, want 2",
}, {
	f:           func(httprequest.Params) string { return "" },
	expectPanic: "bad handler function: function returns 1 values, want 2",
}, {
	f:           func(httprequest.Params) (string, error, error) { return "", nil, nil },
	expectPanic: "bad handler function: function returns 3 values, want 2",
}, {
	f:           func(string) (string, error) { return "", nil },
	expectPanic: `bad handler function: invalid argument or return values, want func\(httprequest.Params\) \(any, error\), got func\(string\) \(string, error\)`,
}, {
	f:           func(httprequest.Params) (string, string) { return "", "" },
	expectPanic: `bad handler function: invalid argument or return values, want func\(httprequest.Params\) \(any, error\), got func\(httprequest.Params\) \(string, string\)`,
}, {
	f:           func(httprequest.Params) (string, error) { return "", nil },
	expectPanic: `no exported methods defined on string`,
}, {
	f:           func(httprequest.Params) (a badHandlersType1, b error) { return },
	expectPanic: `bad type for method M: has 3 parameters, need 1 or 2`,
}, {
	f:           func(httprequest.Params) (a badHandlersType2, b error) { return },
	expectPanic: `method M does not specify route method and path`,
}, {
	f:           func(httprequest.Params) (a badHandlersType3, b error) { return },
	expectPanic: `bad type for Close method \(got func\(httprequest_test\.badHandlersType3\) want func\(httprequest_test.badHandlersType3\) error`,
}}

type badHandlersType1 struct{}

func (badHandlersType1) M(a, b, c int) {
}

type badHandlersType2 struct{}

func (badHandlersType2) M(*struct {
	P int `httprequest:",path"`
}) {
}

type badHandlersType3 struct{}

func (badHandlersType3) M(arg *struct {
	httprequest.Route `httprequest:"GET /m1/:P"`
	P                 int `httprequest:",path"`
}) {
}

func (badHandlersType3) Close() {
}

func (*handlerSuite) TestBadHandlersFunc(c *gc.C) {
	for i, test := range badHandlersFuncTests {
		c.Logf("test %d: %s", i, test.expectPanic)
		c.Check(func() {
			errorMapper.Handlers(test.f)
		}, gc.PanicMatches, test.expectPanic)
	}
}

func (*handlerSuite) TestHandlersFuncReturningError(c *gc.C) {
	handlers := errorMapper.Handlers(func(httprequest.Params) (*testHandlers, error) {
		return nil, errgo.WithCausef(errgo.New("failure"), errUnauth, "something")
	})
	router := httprouter.New()
	for _, h := range handlers {
		router.Handle(h.Method, h.Path, h.Handle)
	}
	httptesting.AssertJSONCall(c, httptesting.JSONCallParams{
		URL:          "/m1/p",
		Handler:      router,
		ExpectStatus: http.StatusUnauthorized,
		ExpectBody: &httprequest.RemoteError{
			Message: "something: failure",
			Code:    "unauthorized",
		},
	})
}

type closeHandlersType struct {
	p      int
	closed bool
}

func (h *closeHandlersType) M(arg *struct {
	httprequest.Route `httprequest:"GET /m1/:P"`
	P                 int `httprequest:",path"`
}) {
	h.p = arg.P
}

func (h *closeHandlersType) Close() error {
	h.closed = true
	return nil
}

func (*handlerSuite) TestHandlersWithTypeThatImplementsIOCloser(c *gc.C) {
	var v closeHandlersType
	handlers := errorMapper.Handlers(func(httprequest.Params) (*closeHandlersType, error) {
		return &v, nil
	})
	router := httprouter.New()
	for _, h := range handlers {
		router.Handle(h.Method, h.Path, h.Handle)
	}
	httptesting.AssertJSONCall(c, httptesting.JSONCallParams{
		URL:     "/m1/99",
		Handler: router,
	})
	c.Assert(v.closed, gc.Equals, true)
	c.Assert(v.p, gc.Equals, 99)
}

func (*handlerSuite) TestBadForm(c *gc.C) {
	h := errorMapper.Handle(func(p httprequest.Params, _ *struct{}) {
		c.Fatalf("shouldn't be called")
	})
	testBadForm(c, h.Handle)
}

func (*handlerSuite) TestBadFormNoParams(c *gc.C) {
	h := errorMapper.Handle(func(_ *struct{}) {
		c.Fatalf("shouldn't be called")
	})
	testBadForm(c, h.Handle)
}

func testBadForm(c *gc.C, h httprouter.Handle) {
	rec := httptest.NewRecorder()
	req := &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type": {"application/x-www-form-urlencoded"},
		},
		Body: body("%6"),
	}
	h(rec, req, httprouter.Params{})
	httptesting.AssertJSONResponse(c, rec, http.StatusBadRequest, httprequest.RemoteError{
		Message: `cannot parse HTTP request form: invalid URL escape "%6"`,
		Code:    "bad request",
	})
}

func (*handlerSuite) TestToHTTP(c *gc.C) {
	var h http.Handler
	h = httprequest.ToHTTP(errorMapper.Handle(func(p httprequest.Params, s *struct{}) {
		c.Assert(p.PathVar, gc.IsNil)
		p.Response.WriteHeader(http.StatusOK)
	}).Handle)
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
	errUnauth        = errors.New("unauth")
	errBadReq        = errors.New("bad request")
	errOther         = errors.New("other")
	errCustomHeaders = errors.New("custom headers")
	errNil           = errors.New("nil result")
)

type HeaderNumber struct {
	N int
}

func (HeaderNumber) SetHeader(h http.Header) {
	h.Add("some-custom-header", "yes")
}

func (*handlerSuite) TestSetHeader(c *gc.C) {
	rec := httptest.NewRecorder()
	err := httprequest.WriteJSON(rec, http.StatusTeapot, HeaderNumber{1234})
	c.Assert(err, gc.IsNil)
	c.Assert(rec.Code, gc.Equals, http.StatusTeapot)
	c.Assert(rec.Body.String(), gc.Equals, `{"N":1234}`)
	c.Assert(rec.Header().Get("content-type"), gc.Equals, "application/json")
	c.Assert(rec.Header().Get("some-custom-header"), gc.Equals, "yes")
}

func (*handlerSuite) TestSetHeaderOnErrorMapper(c *gc.C) {

}

var errorMapper httprequest.ErrorMapper = func(err error) (int, interface{}) {
	resp := &httprequest.RemoteError{
		Message: err.Error(),
	}
	status := http.StatusInternalServerError
	switch errgo.Cause(err) {
	case errUnauth:
		status = http.StatusUnauthorized
		resp.Code = "unauthorized"
	case errBadReq, httprequest.ErrUnmarshal:
		status = http.StatusBadRequest
		resp.Code = "bad request"
	case errCustomHeaders:
		return http.StatusNotAcceptable, httprequest.CustomHeader{
			Body: resp,
			SetHeaderFunc: func(h http.Header) {
				h.Set("Acceptability", "not at all")
			},
		}
	case errNil:
		return status, nil
	}
	return status, &resp
}

var writeErrorTests = []struct {
	err          error
	expectStatus int
	expectResp   *httprequest.RemoteError
	expectHeader http.Header
}{{
	err:          errUnauth,
	expectStatus: http.StatusUnauthorized,
	expectResp: &httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
	},
}, {
	err:          errBadReq,
	expectStatus: http.StatusBadRequest,
	expectResp: &httprequest.RemoteError{
		Message: errBadReq.Error(),
		Code:    "bad request",
	},
}, {
	err:          errOther,
	expectStatus: http.StatusInternalServerError,
	expectResp: &httprequest.RemoteError{
		Message: errOther.Error(),
	},
}, {
	err:          errNil,
	expectStatus: http.StatusInternalServerError,
}, {
	err:          errCustomHeaders,
	expectStatus: http.StatusNotAcceptable,
	expectResp: &httprequest.RemoteError{
		Message: errCustomHeaders.Error(),
	},
	expectHeader: http.Header{
		"Acceptability": {"not at all"},
	},
}}

func (s *handlerSuite) TestWriteError(c *gc.C) {
	for i, test := range writeErrorTests {
		c.Logf("%d: %s", i, test.err)
		rec := httptest.NewRecorder()
		errorMapper.WriteError(rec, test.err)
		resp := parseErrorResponse(c, rec.Body.Bytes())
		c.Assert(resp, gc.DeepEquals, test.expectResp)
		c.Assert(rec.Code, gc.Equals, test.expectStatus)
		for name, vals := range test.expectHeader {
			c.Assert(rec.HeaderMap[name], jc.DeepEquals, vals)
		}
	}
}

func parseErrorResponse(c *gc.C, body []byte) *httprequest.RemoteError {
	var errResp *httprequest.RemoteError
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
	c.Assert(resp, gc.DeepEquals, &httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
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
	c.Assert(resp, gc.DeepEquals, &httprequest.RemoteError{
		Message: errUnauth.Error(),
		Code:    "unauthorized",
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
