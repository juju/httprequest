// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	"gopkg.in/errgo.v1"
)

// Doer is implemented by HTTP client packages
// to make an HTTP request. It is notably implemented
// by http.Client and httpbakery.Client.
//
// When httprequest uses a Doer value for requests
// with a non-empty body, it will use DoWithBody if
// the value implements it (see DoerWithBody).
// This enables httpbakery.Client to be used correctly.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// DoerWithBody is implemented by HTTP clients that need
// to be able to retry HTTP requests with a body.
// It is notably implemented by httpbakery.Client.
type DoerWithBody interface {
	DoWithBody(req *http.Request, body io.ReadSeeker) (*http.Response, error)
}

// Client represents a client that can invoke httprequest endpoints.
type Client struct {
	// BaseURL holds the base URL to use when making
	// HTTP requests.
	BaseURL string

	// Doer holds a value that will be used to actually
	// make the HTTP request. If it is nil, http.DefaultClient
	// will be used instead. If the request has a non-empty body
	// and Doer implements DoerWithBody, DoWithBody
	// will be used instead.
	Doer Doer

	// If a request returns an HTTP response that signifies an
	// error, UnmarshalError is used to unmarshal the response into
	// an appropriate error. See ErrorUnmarshaler for a convenient
	// way to create an UnmarshalError function for a given type. If
	// this is nil, DefaultErrorUnmarshaler will be used.
	UnmarshalError func(resp *http.Response) error
}

// DefaultErrorUnmarshaler is the default error unmarshaler
// used by Client.
var DefaultErrorUnmarshaler = ErrorUnmarshaler(new(RemoteError))

// Call invokes the endpoint implied by the given params,
// which should be of the form accepted by the ArgT
// argument to a function passed to Handle, and
// unmarshals the response into the given response parameter,
// which should be a pointer to the response value.
//
// If resp is nil, the response will be ignored if the
// request was successful.
//
// If resp is of type **http.Response, instead of unmarshaling
// into it, its element will be set to the returned HTTP
// response directly and the caller is responsible for
// closing its Body field.
//
// Any error that c.UnmarshalError or c.Doer returns will not
// have its cause masked.
func (c *Client) Call(params, resp interface{}) error {
	rt, err := getRequestType(reflect.TypeOf(params))
	if err != nil {
		return errgo.Mask(err)
	}
	if rt.method == "" {
		return errgo.Newf("type %T has no httprequest.Route field", params)
	}
	reqURL := appendURL(c.BaseURL, rt.path)
	req, err := Marshal(reqURL, rt.method, params)
	if err != nil {
		return errgo.Mask(err)
	}

	// Actually make the request.
	doer := c.Doer
	if doer == nil {
		doer = http.DefaultClient
	}
	var httpResp *http.Response
	body := req.Body.(BytesReaderCloser)
	// Always use DoWithBody when available.
	if doer1, ok := doer.(DoerWithBody); ok {
		req.Body = nil
		httpResp, err = doer1.DoWithBody(req, body)
	} else {
		httpResp, err = doer.Do(req)
	}
	if err != nil {
		return errgo.Mask(err, errgo.Any)
	}

	// Return response directly if required.
	if respPt, ok := resp.(**http.Response); ok {
		*respPt = httpResp
		return nil
	}
	defer httpResp.Body.Close()
	if 200 <= httpResp.StatusCode && httpResp.StatusCode < 300 {
		return UnmarshalJSONResponse(httpResp, resp)
	}

	errUnmarshaler := c.UnmarshalError
	if errUnmarshaler == nil {
		errUnmarshaler = DefaultErrorUnmarshaler
	}
	err = errUnmarshaler(httpResp)
	if err == nil {
		err = errgo.Newf("unexpected HTTP response status: %s", httpResp.Status)
	}
	return err
}

// ErrorUnmarshaler returns a function which will unmarshal error
// responses into new values of the same type as template. The argument
// must be a pointer. A new instance of it is created every time the
// returned function is called.
func ErrorUnmarshaler(template error) func(*http.Response) error {
	t := reflect.TypeOf(template)
	if t.Kind() != reflect.Ptr {
		panic(errgo.Newf("cannot unmarshal errors into value of type %T", template))
	}
	t = t.Elem()
	return func(resp *http.Response) error {
		if 300 <= resp.StatusCode && resp.StatusCode < 400 {
			// It's a redirection error.
			loc, _ := resp.Location()
			return fmt.Errorf("unexpected redirect (status %s) from %q to %q", resp.Status, resp.Request.URL, loc)
		}
		if err := checkIsJSON(resp.Header, resp.Body); err != nil {
			// TODO consider including some or all of the body
			// in the error.
			return fmt.Errorf("cannot unmarshal error response (status %s): %v", resp.Status, err)
		}
		errv := reflect.New(t)
		if err := UnmarshalJSONResponse(resp, errv.Interface()); err != nil {
			return fmt.Errorf("cannot unmarshal error response (status %s): %v", resp.Status, err)
		}
		return errv.Interface().(error)
	}
}

// UnmarshalJSONResponse unmarshals the given HTTP response
// into x, which should be a pointer to the result to be
// unmarshaled into. 
func UnmarshalJSONResponse(resp *http.Response, x interface{}) error {
	// Try to read all the body so that we can reuse the
	// connection, but don't try *too* hard.
	defer io.Copy(ioutil.Discard, io.LimitReader(resp.Body, 8*1024))
	if x == nil {
		return nil
	}
	if err := checkIsJSON(resp.Header, resp.Body); err != nil {
		return errgo.Mask(err)
	}
	// Decode only a single JSON value, and then
	// discard the rest of the body so that we can
	// reuse the connection even if some foolish server
	// has put garbage on the end.
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(x); err != nil {
		return errgo.Mask(err)
	}
	return nil
}

// RemoteError holds the default type of a remote error
// used by Client when no custom error unmarshaler
// is set.
type RemoteError struct {
	// Message holds the error message.
	Message string

	// Code may hold a code that classifies the error.
	Code string `json:",omitempty"`

	// Info holds any other information associated with the error.
	Info *json.RawMessage `json:",omitempty"`
}

// Error implements the error interface.
func (e *RemoteError) Error() string {
	if e.Message == "" {
		return "httprequest: no error message found"
	}
	return "httprequest: " + e.Message
}

// appendURL appends the path p to the URL u
// separated with a "/".
func appendURL(u, p string) string {
	if p == "" {
		return u
	}
	return strings.TrimSuffix(u, "/") + "/" + strings.TrimPrefix(p, "/")
}
