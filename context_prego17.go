// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

// +build !go1.7

package httprequest

import (
	"net/http"

	"golang.org/x/net/context"
)

func contextFromRequest(req *http.Request) (context.Context, *http.Request, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = ContextWithRequestUUID(ctx, uuidFromRequest(req))
	return ctx, req, cancel
}

func requestWithContext(req *http.Request, _ context.Context) *http.Request {
	return req
}
