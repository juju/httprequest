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
const RequestUUIDHeader = "Request-Uuid"

var uuidGen = fastuuid.MustNewGenerator()

type requestUUIDContextKey struct{}

// RequestUUIDFromContext returns the unique identifier of the request.
// This will have either been taken from a Request-UUID header or
// assigned when the request is initially processed by httprequest. If
// the given context doesn't contain a request UUID then the return value
// will be the empty string.
func RequestUUIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestUUIDContextKey{}).(string)
	return v
}

// ContextWithRequestUUID adds the given request UUID to the given context.
func ContextWithRequestUUID(ctx context.Context, uuid string) context.Context {
	return context.WithValue(ctx, requestUUIDContextKey{}, uuid)
}

// uuidFromRequest gets a UUID for the request. If a Request-UUID header
// is present then the UUID will be taken from that otherwise a new
// random UUID will be generated.
func uuidFromRequest(req *http.Request) string {
	if uuid := req.Header.Get(RequestUUIDHeader); uuid != "" {
		return uuid
	}
	return fmt.Sprintf("%x", uuidGen.Next())
}
