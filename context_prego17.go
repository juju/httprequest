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
	ctx = contextWithRequestUUID(ctx, req)
	return ctx, req, cancel
}
