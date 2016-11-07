// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest

import (
	"fmt"
	"net/http"

	"github.com/rogpeppe/fastuuid"
	"golang.org/x/net/context"
)

// RequestUUIDHeader contains the name of the header used to store the
// request UUID.
const RequestUUIDHeader = "Request-UUID"

var uuidGen = fastuuid.MustNewGenerator()

type requestUUIDContextKey struct{}

// RequestUUID returns the unique identifier of the request. This will
// have either been taken from a Request-UUID header or assigned when
// the request is initially processed by httprequest. If the given
// context doesn't contain a request UUID then the return value will be
// the empty string.
func RequestUUID(ctx context.Context) string {
	v, _ := ctx.Value(requestUUIDContextKey{}).(string)
	return v
}

// contextWithRequestUUID adds a request UUID to the given context. The request
// UUID either comes from a Request-UUID header in the given Request,
// or generates a random UUID.
func contextWithRequestUUID(ctx context.Context, req *http.Request) context.Context {
	uuid := req.Header.Get(RequestUUIDHeader)
	if uuid == "" {
		bytes := uuidGen.Next()
		uuid = fmt.Sprintf("%x", bytes)
	}
	return context.WithValue(ctx, requestUUIDContextKey{}, uuid)
}
