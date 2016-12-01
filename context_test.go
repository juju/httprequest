// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/julienschmidt/httprouter"
	gc "gopkg.in/check.v1"

	"github.com/juju/httprequest"
)

type contextSuite struct{}

var _ = gc.Suite(&contextSuite{})

type testRequest struct {
	httprequest.Route `httprequest:"GET /foo"`
}

func (s *contextSuite) TestContextCancelledWhenDone(c *gc.C) {
	var ch <-chan struct{}
	hnd := testServer.Handle(func(p httprequest.Params, req *testRequest) {
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
