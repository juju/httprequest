// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

// +build go1.7

package httprequest

import (
	"context"
	"net/http"
)

func contextFromRequest(req *http.Request) (context.Context, *http.Request, context.CancelFunc) {
	ctx := req.Context()
	ctx = contextWithRequestUUID(ctx, req)
	req = req.WithContext(ctx)
	// Note there is no need to make httprequest cancel the context
	// as the standard HTTP server will cancel it when the ServeHTTP
	// method completes.
	return ctx, req, func() {}
}
