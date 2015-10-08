package httprequest_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"

	jc "github.com/juju/testing/checkers"
	"github.com/julienschmidt/httprouter"
	gc "gopkg.in/check.v1"
	"gopkg.in/errgo.v1"

	"github.com/juju/httprequest"
)

type clientSuite struct{}

var _ = gc.Suite(&clientSuite{})

var callTests = []struct {
	about       string
	client      httprequest.Client
	req         interface{}
	expectError string
	expectCause interface{}
	expectResp  interface{}
}{{
	about: "GET success",
	req: &chM1Req{
		P: "hello",
	},
	expectResp: &chM1Resp{"hello"},
}, {
	about: "GET with nil response",
	req: &chM1Req{
		P: "hello",
	},
}, {
	about: "POST success",
	req: &chM2Req{
		P:    "hello",
		Body: struct{ I int }{999},
	},
	expectResp: &chM2Resp{"hello", 999},
}, {
	about:       "GET marshal error",
	req:         123,
	expectError: `type is not pointer to struct`,
}, {
	about: "error response",
	req: &chInvalidM2Req{
		P:    "hello",
		Body: struct{ I bool }{true},
	},
	expectError: `POST http://.*/m2/hello: httprequest: cannot unmarshal parameters: cannot unmarshal into field: cannot unmarshal request body: json: cannot unmarshal bool into Go value of type int`,
	expectCause: &httprequest.RemoteError{
		Message: `cannot unmarshal parameters: cannot unmarshal into field: cannot unmarshal request body: json: cannot unmarshal bool into Go value of type int`,
		Code:    "bad request",
	},
}, {
	about: "error unmarshaler returns nil",
	client: httprequest.Client{
		UnmarshalError: func(*http.Response) error {
			return nil
		},
	},
	req:         &chM3Req{},
	expectError: `GET http://.*/m3: unexpected HTTP response status: 500 Internal Server Error`,
}, {
	about:       "unexpected redirect",
	req:         &chM2RedirectM2Req{},
	expectError: `POST http://.*/m2/foo//: unexpected redirect \(status 307 Temporary Redirect\) from "http://.*/m2/foo//" to "http://.*/m2/foo"`,
}, {
	about: "doer with body",
	client: httprequest.Client{
		Doer: doerFunc(func(req *http.Request, body io.ReadSeeker) (*http.Response, error) {
			if body == nil {
				panic("Do called when DoWithBody expected")
			}
			req.Body = ioutil.NopCloser(body)
			return http.DefaultClient.Do(req)
		}),
	},
	req: &chM2Req{
		P:    "hello",
		Body: struct{ I int }{999},
	},
	expectResp: &chM2Resp{"hello", 999},
}, {
	about: "doer that implements DoWithBody but no body",
	client: httprequest.Client{
		Doer: doerFunc(func(req *http.Request, body io.ReadSeeker) (*http.Response, error) {
			if body == nil {
				panic("Do called but DoWithBody should always be called")
			}
			return http.DefaultClient.Do(req)
		}),
	},
	req: &chM1Req{
		P: "hello",
	},
	expectResp: &chM1Resp{"hello"},
}, {
	about:       "bad content in successful response",
	req:         &chM4Req{},
	expectResp:  new(int),
	expectError: `GET http://.*/m4: unexpected content type text/plain; want application/json; content: bad response`,
}}

func (s *clientSuite) TestCall(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()

	for i, test := range callTests {
		c.Logf("test %d: %s", i, test.about)
		var resp interface{}
		if test.expectResp != nil {
			resp = reflect.New(reflect.TypeOf(test.expectResp).Elem()).Interface()
		}
		client := test.client
		client.BaseURL = srv.URL
		err := client.Call(test.req, resp)
		if test.expectError != "" {
			c.Assert(err, gc.ErrorMatches, test.expectError)
			if test.expectCause != nil {
				c.Assert(errgo.Cause(err), jc.DeepEquals, test.expectCause)
			}
			continue
		}
		c.Assert(err, gc.IsNil)
		c.Assert(resp, jc.DeepEquals, test.expectResp)
	}
}

func (s *clientSuite) TestCallURLNoRequestPath(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()

	var client httprequest.Client
	req := struct {
		httprequest.Route `httprequest:"GET"`
		chM1Req
	}{
		chM1Req: chM1Req{
			P: "hello",
		},
	}
	var resp chM1Resp
	err := client.CallURL(srv.URL+"/m1/:P", &req, &resp)
	c.Assert(err, gc.IsNil)
	c.Assert(resp, jc.DeepEquals, chM1Resp{"hello"})
}

func mustNewRequest(url string, method string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

var doTests = []struct {
	about   string
	client  httprequest.Client
	request *http.Request
	body    io.ReadSeeker

	expectError string
	expectCause interface{}
	expectResp  interface{}
}{{
	about:      "GET success",
	request:    mustNewRequest("/m1/hello", "GET", nil),
	expectResp: &chM1Resp{"hello"},
}, {
	about:   "appendURL error",
	request: mustNewRequest("/m1/hello", "GET", nil),
	client: httprequest.Client{
		BaseURL: ":::",
	},
	expectError: `cannot parse ":::": parse :::: missing protocol scheme`,
}, {
	about:       "body supplied in request",
	request:     mustNewRequest("/m1/hello", "GET", strings.NewReader("")),
	expectError: `GET http://.*/m1/hello: request body supplied unexpectedly`,
}, {
	about:      "content length is inferred from strings.Reader",
	request:    mustNewRequest("/content-length", "PUT", nil),
	body:       strings.NewReader("hello"),
	expectResp: newInt64(int64(len("hello"))),
}, {
	about:      "content length is inferred from bytes.Reader",
	request:    mustNewRequest("/content-length", "PUT", nil),
	body:       bytes.NewReader([]byte("hello")),
	expectResp: newInt64(int64(len("hello"))),
}, {
	about: "DoWithBody implemented but no body",
	client: httprequest.Client{
		Doer: doerFunc(func(req *http.Request, body io.ReadSeeker) (*http.Response, error) {
			if body != nil {
				panic("DoWithBody called when Do expected")
			}
			return http.DefaultClient.Do(req)
		}),
	},
	request:    mustNewRequest("/m1/hello", "GET", nil),
	expectResp: &chM1Resp{"hello"},
}, {
	about: "DoWithBody not implemented and body present",
	client: httprequest.Client{
		Doer: doerOnlyFunc(func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}),
	},
	request: mustNewRequest("/m2/foo", "POST", nil),
	body:    strings.NewReader(`{"I": 999}`),
	expectResp: &chM2Resp{
		P:   "foo",
		Arg: 999,
	},
}, {
	about: "DoWithBody implemented and body present",
	client: httprequest.Client{
		Doer: doerFunc(func(req *http.Request, body io.ReadSeeker) (*http.Response, error) {
			if body == nil {
				panic("Do called when DoWithBody expected")
			}
			req.Body = ioutil.NopCloser(body)
			return http.DefaultClient.Do(req)
		}),
	},
	request: mustNewRequest("/m2/foo", "POST", nil),
	body:    strings.NewReader(`{"I": 999}`),
	expectResp: &chM2Resp{
		P:   "foo",
		Arg: 999,
	},
}, {
	about: "Do returns error",
	client: httprequest.Client{
		Doer: doerOnlyFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errgo.Newf("an error")
		}),
	},
	request:     mustNewRequest("/m2/foo", "POST", nil),
	body:        strings.NewReader(`{"I": 999}`),
	expectError: "POST http://.*/m2/foo: an error",
}}

func newInt64(i int64) *int64 {
	return &i
}

func (s *clientSuite) TestDo(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	for i, test := range doTests {
		c.Logf("test %d: %s", i, test.about)
		var resp interface{}
		if test.expectResp != nil {
			resp = reflect.New(reflect.TypeOf(test.expectResp).Elem()).Interface()
		}
		client := test.client
		if client.BaseURL == "" {
			client.BaseURL = srv.URL
		}
		err := client.Do(test.request, test.body, resp)
		if test.expectError != "" {
			c.Assert(err, gc.ErrorMatches, test.expectError)
			if test.expectCause != nil {
				c.Assert(errgo.Cause(err), jc.DeepEquals, test.expectCause)
			}
			continue
		}
		c.Assert(err, gc.IsNil)
		c.Assert(resp, jc.DeepEquals, test.expectResp)
	}
}

func (s *clientSuite) TestDoWithHTTPReponse(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	client := &httprequest.Client{
		BaseURL: srv.URL,
	}
	var resp *http.Response
	err := client.Get("/m1/foo", &resp)
	c.Assert(err, gc.IsNil)
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, `{"P":"foo"}`)
}

func (s *clientSuite) TestDoWithHTTPReponseAndError(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	var doer closeCountingDoer // Also check the body is closed.
	client := &httprequest.Client{
		BaseURL: srv.URL,
		Doer:    &doer,
	}
	var resp *http.Response
	err := client.Get("/m3", &resp)
	c.Assert(resp, gc.IsNil)
	c.Assert(err, gc.ErrorMatches, `GET http://.*/m3: httprequest: m3 error`)
	c.Assert(doer.openedBodies, gc.Equals, 1)
	c.Assert(doer.closedBodies, gc.Equals, 1)
}

func (s *clientSuite) TestCallWithHTTPResponse(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	client := &httprequest.Client{
		BaseURL: srv.URL,
	}
	var resp *http.Response
	err := client.Call(&chM1Req{
		P: "foo",
	}, &resp)
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, `{"P":"foo"}`)
}

func (s *clientSuite) TestCallClosesResponseBodyOnSuccess(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	var doer closeCountingDoer
	client := &httprequest.Client{
		BaseURL: srv.URL,
		Doer:    &doer,
	}
	var resp chM1Resp
	err := client.Call(&chM1Req{
		P: "foo",
	}, &resp)
	c.Assert(err, gc.IsNil)
	c.Assert(resp, jc.DeepEquals, chM1Resp{"foo"})
	c.Assert(doer.openedBodies, gc.Equals, 1)
	c.Assert(doer.closedBodies, gc.Equals, 1)
}

func (s *clientSuite) TestCallClosesResponseBodyOnError(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	var doer closeCountingDoer
	client := &httprequest.Client{
		BaseURL: srv.URL,
		Doer:    &doer,
	}
	err := client.Call(&chM3Req{}, nil)
	c.Assert(err, gc.ErrorMatches, ".*m3 error")
	c.Assert(doer.openedBodies, gc.Equals, 1)
	c.Assert(doer.closedBodies, gc.Equals, 1)
}

func (s *clientSuite) TestDoClosesResponseBodyOnSuccess(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	var doer closeCountingDoer
	client := &httprequest.Client{
		BaseURL: srv.URL,
		Doer:    &doer,
	}
	req, err := http.NewRequest("GET", "/m1/foo", nil)
	c.Assert(err, gc.IsNil)
	var resp chM1Resp
	err = client.Do(req, nil, &resp)
	c.Assert(err, gc.IsNil)
	c.Assert(resp, jc.DeepEquals, chM1Resp{"foo"})
	c.Assert(doer.openedBodies, gc.Equals, 1)
	c.Assert(doer.closedBodies, gc.Equals, 1)
}

func (s *clientSuite) TestDoClosesResponseBodyOnError(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	var doer closeCountingDoer
	client := &httprequest.Client{
		BaseURL: srv.URL,
		Doer:    &doer,
	}
	req, err := http.NewRequest("GET", "/m3", nil)
	c.Assert(err, gc.IsNil)
	err = client.Do(req, nil, nil)
	c.Assert(err, gc.ErrorMatches, ".*m3 error")
	c.Assert(doer.openedBodies, gc.Equals, 1)
	c.Assert(doer.closedBodies, gc.Equals, 1)
}

func (s *clientSuite) TestGet(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	client := httprequest.Client{
		BaseURL: srv.URL,
	}
	var resp chM1Resp
	err := client.Get("/m1/foo", &resp)
	c.Assert(err, gc.IsNil)
	c.Assert(resp, jc.DeepEquals, chM1Resp{"foo"})
}

func (s *clientSuite) TestGetNoBaseURL(c *gc.C) {
	srv := s.newServer()
	defer srv.Close()
	client := httprequest.Client{}
	var resp chM1Resp
	err := client.Get(srv.URL+"/m1/foo", &resp)
	c.Assert(err, gc.IsNil)
	c.Assert(resp, jc.DeepEquals, chM1Resp{"foo"})
}

func (*clientSuite) newServer() *httptest.Server {
	f := func(p httprequest.Params) (clientHandlers, error) {
		return clientHandlers{}, nil
	}
	handlers := errorMapper.Handlers(f)
	router := httprouter.New()
	for _, h := range handlers {
		router.Handle(h.Method, h.Path, h.Handle)
	}

	return httptest.NewServer(router)
}

var appendURLTests = []struct {
	u           string
	p           string
	expect      string
	expectError string
}{{
	u:      "http://foo",
	p:      "bar",
	expect: "http://foo/bar",
}, {
	u:      "http://foo",
	p:      "/bar",
	expect: "http://foo/bar",
}, {
	u:      "http://foo/",
	p:      "bar",
	expect: "http://foo/bar",
}, {
	u:      "http://foo/",
	p:      "/bar",
	expect: "http://foo/bar",
}, {
	u:      "",
	p:      "bar",
	expect: "/bar",
}, {
	u:      "http://xxx",
	p:      "",
	expect: "http://xxx",
}, {
	u:           "http://xxx.com",
	p:           "http://foo.com",
	expectError: "relative URL specifies a host",
}, {
	u:      "http://xxx.com/a/b",
	p:      "foo?a=45&b=c",
	expect: "http://xxx.com/a/b/foo?a=45&b=c",
}, {
	u:      "http://xxx.com",
	p:      "?a=45&b=c",
	expect: "http://xxx.com?a=45&b=c",
}, {
	u:      "http://xxx.com/a?z=w",
	p:      "foo?a=45&b=c",
	expect: "http://xxx.com/a/foo?z=w&a=45&b=c",
}, {
	u:      "http://xxx.com?z=w",
	p:      "/a/b/c",
	expect: "http://xxx.com/a/b/c?z=w",
}}

func (*clientSuite) TestAppendURL(c *gc.C) {
	for i, test := range appendURLTests {
		c.Logf("test %d: %s %s", i, test.u, test.p)
		u, err := httprequest.AppendURL(test.u, test.p)
		if test.expectError != "" {
			c.Assert(u, gc.IsNil)
			c.Assert(err, gc.ErrorMatches, test.expectError)
		} else {
			c.Assert(err, gc.IsNil)
			c.Assert(u.String(), gc.Equals, test.expect)
		}
	}
}

type clientHandlers struct{}

type chM1Req struct {
	httprequest.Route `httprequest:"GET /m1/:P"`
	P                 string `httprequest:",path"`
}

type chM1Resp struct {
	P string
}

func (clientHandlers) M1(p *chM1Req) (*chM1Resp, error) {
	return &chM1Resp{p.P}, nil
}

type chM2Req struct {
	httprequest.Route `httprequest:"POST /m2/:P"`
	P                 string `httprequest:",path"`
	Body              struct {
		I int
	} `httprequest:",body"`
}

type chInvalidM2Req struct {
	httprequest.Route `httprequest:"POST /m2/:P"`
	P                 string `httprequest:",path"`
	Body              struct {
		I bool
	} `httprequest:",body"`
}

type chM2RedirectM2Req struct {
	httprequest.Route `httprequest:"POST /m2/foo//"`
}

type chM2Resp struct {
	P   string
	Arg int
}

func (clientHandlers) M2(p *chM2Req) (*chM2Resp, error) {
	return &chM2Resp{p.P, p.Body.I}, nil
}

type chM3Req struct {
	httprequest.Route `httprequest:"GET /m3"`
}

func (clientHandlers) M3(p *chM3Req) error {
	return errgo.New("m3 error")
}

type chM4Req struct {
	httprequest.Route `httprequest:"GET /m4"`
}

func (clientHandlers) M4(p httprequest.Params, _ *chM4Req) {
	p.Response.Write([]byte("bad response"))
}

type chContentLengthReq struct {
	httprequest.Route `httprequest:"PUT /content-length"`
}

func (clientHandlers) ContentLength(rp httprequest.Params, p *chContentLengthReq) (int64, error) {
	return rp.Request.ContentLength, nil
}

type doerFunc func(req *http.Request, body io.ReadSeeker) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req, nil)
}

func (f doerFunc) DoWithBody(req *http.Request, body io.ReadSeeker) (*http.Response, error) {
	if req.Body != nil {
		panic("unexpected non-nil body in request")
	}
	if body == nil {
		panic("unexpected nil body argument")
	}
	return f(req, body)
}

type doerOnlyFunc func(req *http.Request) (*http.Response, error)

func (f doerOnlyFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type closeCountingDoer struct {
	// openBodies records the number of response bodies
	// that have been returned.
	openedBodies int

	// closedBodies records the number of response bodies
	// that have been closed.
	closedBodies int
}

func (doer *closeCountingDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &closeCountingReader{
		doer:       doer,
		ReadCloser: resp.Body,
	}
	doer.openedBodies++
	return resp, nil
}

type closeCountingReader struct {
	doer *closeCountingDoer
	io.ReadCloser
}

func (r *closeCountingReader) Close() error {
	r.doer.closedBodies++
	return r.ReadCloser.Close()
}
