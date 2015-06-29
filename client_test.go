package httprequest_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/errgo.v1"

	"github.com/juju/httprequest"
	"github.com/julienschmidt/httprouter"
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
	expectError: `httprequest: cannot unmarshal parameters: cannot unmarshal into field: cannot unmarshal request body: json: cannot unmarshal bool into Go value of type int`,
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
	expectError: `unexpected HTTP response status: 500 Internal Server Error`,
}, {
	about:       "unexpected redirect",
	req:         &chM2RedirectM2Req{},
	expectError: `unexpected redirect \(status 307 Temporary Redirect\) from "http://.*/m2/foo//" to "http://.*/m2/foo"`,
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
}}

func (*clientSuite) TestCall(c *gc.C) {
	f := func(p httprequest.Params) (clientHandlers, error) {
		return clientHandlers{}, nil
	}
	handlers := errorMapper.Handlers(f)
	router := httprouter.New()
	for _, h := range handlers {
		router.Handle(h.Method, h.Path, h.Handle)
	}

	srv := httptest.NewServer(router)
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

var appendURLTests = []struct {
	u      string
	p      string
	expect string
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
}}

func (*clientSuite) TestAppendURL(c *gc.C) {
	for i, test := range appendURLTests {
		c.Logf("test %d: %s %s", i, test.u, test.p)
		c.Assert(httprequest.AppendURL(test.u, test.p), gc.Equals, test.expect)
	}
}

type clientHandlers struct {
}

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
