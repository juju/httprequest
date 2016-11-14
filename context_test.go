// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
	gc "gopkg.in/check.v1"

	"github.com/juju/httprequest"
)

type contextSuite struct{}

var _ = gc.Suite(&contextSuite{})

func (s *contextSuite) TestRequestUUIDNotInContext(c *gc.C) {
	c.Assert(httprequest.RequestUUIDFromContext(context.Background()), gc.Equals, "")
}

type testRequest struct {
	httprequest.Route `httprequest:"GET /foo"`
}

func (s *contextSuite) TestRequestUUIDFromHeader(c *gc.C) {
	hnd := errorMapper.Handle(func(p httprequest.Params, req *testRequest) {
		uuid := httprequest.RequestUUIDFromContext(p.Context)
		c.Assert(uuid, gc.Equals, "test-uuid")
	})
	req, err := http.NewRequest("GET", "/foo", nil)
	req.Header.Set(httprequest.RequestUUIDHeader, "test-uuid")
	c.Assert(err, gc.Equals, nil)
	rr := httptest.NewRecorder()
	hnd.Handle(rr, req, nil)
}

func (s *contextSuite) TestRequestUUIDGenerated(c *gc.C) {
	hnd := errorMapper.Handle(func(p httprequest.Params, req *testRequest) {
		uuid := httprequest.RequestUUIDFromContext(p.Context)
		c.Assert(uuid, gc.Not(gc.Equals), "")
	})
	req, err := http.NewRequest("GET", "/foo", nil)
	c.Assert(err, gc.Equals, nil)
	rr := httptest.NewRecorder()
	hnd.Handle(rr, req, nil)
}

func (s *contextSuite) TestContextCancelledWhenDone(c *gc.C) {
	var ch <-chan struct{}
	hnd := errorMapper.Handle(func(p httprequest.Params, req *testRequest) {
		ch = p.Context.Done()
	})
	router := httprouter.New()
	router.Handle(hnd.Method, hnd.Path, hnd.Handle)
	srv := httptest.NewServer(router)
	_, err := http.Get(srv.URL + "/foo")
	c.Assert(err, gc.Equals, nil)
	select {
	case <-ch:
	default:
		c.Fatal("context not canceled at end of handler.")
	}
}
