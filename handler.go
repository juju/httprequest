// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/errgo.v1"
)

// ErrorMapper holds a function that can convert a Go error
// into a form that can be returned as a JSON body from an HTTP request.
// The httpStatus value reports the desired HTTP status.
type ErrorMapper func(err error) (httpStatus int, errorBody interface{})

var (
	paramsType             = reflect.TypeOf(Params{})
	errorType              = reflect.TypeOf((*error)(nil)).Elem()
	httpResponseWriterType = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	httpHeaderType         = reflect.TypeOf(http.Header(nil))
)

// Handle converts a function into an httprouter.Handle. The argument f
// must be a function of one of the following six forms, where ArgT
// must be a struct type acceptable to Unmarshal and ResultT is a type
// that can be marshaled as JSON:
//
//	func(p Params, arg *ArgT)
//	func(p Params, arg *ArgT) error
//	func(p Params, arg *ArgT) (ResultT, error)
//
//	func(arg *ArgT)
//	func(arg *ArgT) error
//	func(arg *ArgT) (ResultT, error)
//
// When processing a call to the returned handler, the provided
// parameters are unmarshaled into a new ArgT value using Unmarshal,
// then f is called with this value. If the unmarshaling fails, f will
// not be called and the unmarshal error will be written as a JSON
// response.
//
// If an error is returned from f, it is passed through the error mapper before
// writing as a JSON response.
//
// In the third form, when no error is returned, the result is written
// as a JSON response with status http.StatusOK. In this case,
// any calls to p.Response.Write and p.Response.WriteHeader
// are ignored.
//
// Handle will panic if the provided function is not in one
// of the above forms.
func (e ErrorMapper) Handle(f interface{}) httprouter.Handle {
	fv := reflect.ValueOf(f)
	pt, err := checkHandleType(fv.Type())
	if err != nil {
		panic(errgo.Notef(err, "bad handler function"))
	}
	return e.handleResult(fv.Type(), handleParams(fv, pt))
}

func checkHandleType(t reflect.Type) (*requestType, error) {
	if t.Kind() != reflect.Func {
		return nil, errgo.New("not a function")
	}
	if t.NumIn() != 1 && t.NumIn() != 2 {
		return nil, errgo.Newf("has %d parameters, need 1 or 2", t.NumIn())
	}
	if t.NumOut() > 2 {
		return nil, errgo.Newf("has %d result parameters, need 0, 1 or 2", t.NumOut())
	}
	if t.NumIn() == 2 {
		if t.In(0) != paramsType {
			return nil, errgo.Newf("first argument is %v, need httprequest.Params", t.In(0))
		}
	} else {
		if t.In(0) == paramsType {
			return nil, errgo.Newf("no argument parameter after Params argument")
		}
	}
	pt, err := getRequestType(t.In(t.NumIn() - 1))
	if err != nil {
		return nil, errgo.Notef(err, "last argument cannot be used for Unmarshal")
	}
	if t.NumOut() > 0 {
		//	func(p Params, arg *ArgT) error
		//	func(p Params, arg *ArgT) (ResultT, error)
		if et := t.Out(t.NumOut() - 1); et != errorType {
			return nil, errgo.Newf("final result parameter is %s, need error", et)
		}
	}
	return pt, nil
}

func handleParams(fv reflect.Value, pt *requestType) func(w http.ResponseWriter, req *http.Request, p httprouter.Params) ([]reflect.Value, error) {
	ft := fv.Type()
	returnJSON := ft.NumOut() > 1
	needsParams := ft.In(0) == paramsType
	if needsParams {
		argStructType := ft.In(1).Elem()
		return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) ([]reflect.Value, error) {
			if err := req.ParseForm(); err != nil {
				return nil, errgo.WithCausef(err, ErrUnmarshal, "cannot parse HTTP request form")
			}
			if returnJSON {
				w = headerOnlyResponseWriter{w.Header()}
			}
			params := Params{
				Response: w,
				Request:  req,
				PathVar:  p,
			}
			pv := reflect.New(argStructType)
			if err := unmarshal(params, pv, pt); err != nil {
				return nil, errgo.NoteMask(err, "cannot unmarshal parameters", errgo.Is(ErrUnmarshal))
			}
			return fv.Call([]reflect.Value{
				reflect.ValueOf(params),
				pv,
			}), nil
		}
	}
	argStructType := ft.In(0).Elem()
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) ([]reflect.Value, error) {
		if err := req.ParseForm(); err != nil {
			return nil, errgo.WithCausef(err, ErrUnmarshal, "cannot parse HTTP request form")
		}
		pv := reflect.New(argStructType)
		if err := unmarshal(Params{
			Request: req,
			PathVar: p,
		}, pv, pt); err != nil {
			return nil, errgo.NoteMask(err, "cannot unmarshal parameters", errgo.Is(ErrUnmarshal))
		}
		return fv.Call([]reflect.Value{pv}), nil
	}

}

func (e ErrorMapper) handleResult(ft reflect.Type, f func(w http.ResponseWriter, req *http.Request, p httprouter.Params) ([]reflect.Value, error)) httprouter.Handle {
	switch ft.NumOut() {
	case 0:
		//	func(w http.ResponseWriter, p Params, arg *ArgT)
		return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
			_, err := f(w, req, p)
			if err != nil {
				e.WriteError(w, err)
			}
		}
	case 1:
		//	func(w http.ResponseWriter, p Params, arg *ArgT) error
		return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
			out, err := f(w, req, p)
			if err != nil {
				e.WriteError(w, err)
				return
			}
			herr := out[0].Interface()
			if herr != nil {
				e.WriteError(w, herr.(error))
			}
		}
	case 2:
		//	func(header http.Header, p Params, arg *ArgT) (ResultT, error)
		return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
			out, err := f(w, req, p)
			if err != nil {
				e.WriteError(w, err)
				return
			}
			herr := out[1].Interface()
			if herr != nil {
				e.WriteError(w, herr.(error))
				return
			}
			err = WriteJSON(w, http.StatusOK, out[0].Interface())
			if err != nil {
				e.WriteError(w, err)
			}
		}
	default:
		panic("unreachable")
	}
}

// ToHTTP converts an httprouter.Handle into an http.Handler.
// It will pass no path variables to h.
func ToHTTP(h httprouter.Handle) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h(w, req, nil)
	})
}

// JSONHandler is like httprouter.Handle except that it returns a
// body (to be converted to JSON) and an error.
// The Header parameter can be used to set
// custom headers on the response.
type JSONHandler func(Params) (interface{}, error)

// ErrorHandler is like httprouter.Handle except it returns an error
// which may be returned as the error body of the response.
// An ErrorHandler function should not itself write to the ResponseWriter
// if it returns an error.
type ErrorHandler func(Params) error

// HandleJSON returns a handler that writes the return value of handle
// as a JSON response. If handle returns an error, it is passed through
// the error mapper.
func (e ErrorMapper) HandleJSON(handle JSONHandler) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		val, err := handle(Params{
			Response: headerOnlyResponseWriter{w.Header()},
			Request:  req,
			PathVar:  p,
		})
		if err == nil {
			if err = WriteJSON(w, http.StatusOK, val); err == nil {
				return
			}
		}
		e.WriteError(w, err)
	}
}

// HandleErrors returns a handler that passes any non-nil error returned
// by handle through the error mapper and writes it as a JSON response.
func (e ErrorMapper) HandleErrors(handle ErrorHandler) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		w1 := responseWriter{
			ResponseWriter: w,
		}
		if err := handle(Params{
			Response: &w1,
			Request:  req,
			PathVar:  p,
		}); err != nil {
			// We write the error only if the header hasn't
			// already been written, because if it has, then
			// we will not be able to set the appropriate error
			// response code, and there's a danger that we
			// may be corrupting output by appending
			// a JSON error message to it.
			if !w1.headerWritten {
				e.WriteError(w, err)
			}
			// TODO log the error?
		}
	}
}

// WriteError writes an error to a ResponseWriter
// and sets the HTTP status code.
func (e ErrorMapper) WriteError(w http.ResponseWriter, err error) {
	status, resp := e(err)
	WriteJSON(w, status, resp)
}

// WriteJSON writes the given value to the ResponseWriter
// and sets the HTTP status to the given code.
func WriteJSON(w http.ResponseWriter, code int, val interface{}) error {
	// TODO consider marshalling directly to w using json.NewEncoder.
	// pro: this will not require a full buffer allocation.
	// con: if there's an error after the first write, it will be lost.
	data, err := json.Marshal(val)
	if err != nil {
		// TODO(rog) log an error if this fails and lose the
		// error return, because most callers will need
		// to do that anyway.
		return errgo.Mask(err)
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
	return nil
}

// Ensure statically that responseWriter does implement http.Flusher.
var _ http.Flusher = (*responseWriter)(nil)

// responseWriter wraps http.ResponseWriter but allows us
// to find out whether any body has already been written.
type responseWriter struct {
	headerWritten bool
	http.ResponseWriter
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.headerWritten = true
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteHeader(code int) {
	w.headerWritten = true
	w.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher.Flush.
func (w *responseWriter) Flush() {
	w.headerWritten = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type headerOnlyResponseWriter struct {
	h http.Header
}

func (w headerOnlyResponseWriter) Header() http.Header {
	return w.h
}

func (w headerOnlyResponseWriter) Write([]byte) (int, error) {
	// TODO log or panic when this happens?
	return 0, errgo.New("inappropriate call to ResponseWriter.Write in JSON-returning handler")
}

func (w headerOnlyResponseWriter) WriteHeader(code int) {
	// TODO log or panic when this happens?
}
