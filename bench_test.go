// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/errgo.v1"

	"github.com/juju/httprequest"
)

const dateFormat = "2006-01-02"

type testResult struct {
	Key   string `json:",omitempty"`
	Date  string `json:",omitempty"`
	Count int64
}

type testParams2Fields struct {
	Id    string `httprequest:"id,path"`
	Limit int    `httprequest:"limit,form"`
}

type testParams4Fields struct {
	Id    string   `httprequest:"id,path"`
	Limit int      `httprequest:"limit,form"`
	From  dateTime `httprequest:"from,form"`
	To    dateTime `httprequest:"to,form"`
}

type dateTime struct {
	time.Time
}

func (dt *dateTime) UnmarshalText(b []byte) (err error) {
	dt.Time, err = time.Parse(dateFormat, string(b))
	return
}

type testParams2StringFields struct {
	Field0 string `httprequest:",form"`
	Field1 string `httprequest:",form"`
}

type testParams4StringFields struct {
	Field0 string `httprequest:",form"`
	Field1 string `httprequest:",form"`
	Field2 string `httprequest:",form"`
	Field3 string `httprequest:",form"`
}

type testParams8StringFields struct {
	Field0 string `httprequest:",form"`
	Field1 string `httprequest:",form"`
	Field2 string `httprequest:",form"`
	Field3 string `httprequest:",form"`
	Field4 string `httprequest:",form"`
	Field5 string `httprequest:",form"`
	Field6 string `httprequest:",form"`
	Field7 string `httprequest:",form"`
}

type testParams16StringFields struct {
	Field0  string `httprequest:",form"`
	Field1  string `httprequest:",form"`
	Field2  string `httprequest:",form"`
	Field3  string `httprequest:",form"`
	Field4  string `httprequest:",form"`
	Field5  string `httprequest:",form"`
	Field6  string `httprequest:",form"`
	Field7  string `httprequest:",form"`
	Field8  string `httprequest:",form"`
	Field9  string `httprequest:",form"`
	Field10 string `httprequest:",form"`
	Field11 string `httprequest:",form"`
	Field12 string `httprequest:",form"`
	Field13 string `httprequest:",form"`
	Field14 string `httprequest:",form"`
	Field15 string `httprequest:",form"`
}

func BenchmarkUnmarshal2Fields(b *testing.B) {
	params := httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"limit": {"2000"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "id",
			Value: "someid",
		}},
	}
	var arg testParams2Fields

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arg = testParams2Fields{}
		err := httprequest.Unmarshal(params, &arg)
		if err != nil {
			b.Fatalf("unmarshal failed: %v", err)
		}
	}
	b.StopTimer()
	if !reflect.DeepEqual(arg, testParams2Fields{
		Id:    "someid",
		Limit: 2000,
	}) {
		b.Errorf("unexpected result: got %#v", arg)
	}
}

func BenchmarkHandle2FieldsTrad(b *testing.B) {
	results := []testResult{}
	benchmarkHandle2Fields(b, errorMapper.HandleJSON(func(p httprequest.Params) (interface{}, error) {
		limit := -1
		if limitStr := p.Request.Form.Get("limit"); limitStr != "" {
			var err error
			limit, err = strconv.Atoi(limitStr)
			if err != nil || limit <= 0 {
				panic("unreachable")
			}
		}
		if id := p.PathVar.ByName("id"); id == "" {
			panic("unreachable")
		}
		return results, nil
	}))
}

func BenchmarkHandle2Fields(b *testing.B) {
	results := []testResult{}
	benchmarkHandle2Fields(b, errorMapper.Handle(func(p httprequest.Params, arg *testParams2Fields) ([]testResult, error) {
		if arg.Limit <= 0 {
			panic("unreachable")
		}
		return results, nil
	}).Handle)
}

func BenchmarkHandle2FieldsUnmarshalOnly(b *testing.B) {
	results := []testResult{}
	benchmarkHandle2Fields(b, errorMapper.HandleJSON(func(p httprequest.Params) (interface{}, error) {
		var arg testParams2Fields
		if err := httprequest.Unmarshal(p, &arg); err != nil {
			return nil, err
		}
		if arg.Limit <= 0 {
			panic("unreachable")
		}
		return results, nil
	}))
}

func benchmarkHandle2Fields(b *testing.B, handle func(w http.ResponseWriter, req *http.Request, pvar httprouter.Params)) {
	rec := httptest.NewRecorder()
	params := httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"limit": {"2000"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "id",
			Value: "someid",
		}},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec.Body.Reset()
		handle(rec, params.Request, params.PathVar)
	}
}

func BenchmarkUnmarshal4Fields(b *testing.B) {
	fromDate, err1 := time.Parse(dateFormat, "2010-10-10")
	toDate, err2 := time.Parse(dateFormat, "2011-11-11")
	if err1 != nil || err2 != nil {
		b.Fatalf("bad times")
	}
	type P testParams4Fields
	params := httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"limit": {"2000"},
				"from":  {fromDate.Format(dateFormat)},
				"to":    {toDate.Format(dateFormat)},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "id",
			Value: "someid",
		}},
	}
	var args P

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		args = P{}
		err := httprequest.Unmarshal(params, &args)
		if err != nil {
			b.Fatalf("unmarshal failed: %v", err)
		}
	}
	b.StopTimer()
	if !reflect.DeepEqual(args, P{
		Id:    "someid",
		Limit: 2000,
		From:  dateTime{fromDate},
		To:    dateTime{toDate},
	}) {
		b.Errorf("unexpected result: got %#v", args)
	}
}

func BenchmarkHandle4FieldsTrad(b *testing.B) {
	results := []testResult{}
	benchmarkHandle4Fields(b, errorMapper.HandleJSON(func(p httprequest.Params) (interface{}, error) {
		start, stop, err := parseDateRange(p.Request.Form)
		if err != nil {
			panic("unreachable")
		}
		_ = start
		_ = stop
		limit := -1
		if limitStr := p.Request.Form.Get("limit"); limitStr != "" {
			limit, err = strconv.Atoi(limitStr)
			if err != nil || limit <= 0 {
				panic("unreachable")
			}
		}
		if id := p.PathVar.ByName("id"); id == "" {
			panic("unreachable")
		}
		return results, nil
	}))
}

// parseDateRange parses a date range as specified in an http
// request. The returned times will be zero if not specified.
func parseDateRange(form url.Values) (start, stop time.Time, err error) {
	if v := form.Get("start"); v != "" {
		var err error
		start, err = time.Parse(dateFormat, v)
		if err != nil {
			return time.Time{}, time.Time{}, errgo.Newf("invalid 'start' value %q", v)
		}
	}
	if v := form.Get("stop"); v != "" {
		var err error
		stop, err = time.Parse(dateFormat, v)
		if err != nil {
			return time.Time{}, time.Time{}, errgo.Newf("invalid 'stop' value %q", v)
		}
		// Cover all timestamps within the stop day.
		stop = stop.Add(24*time.Hour - 1*time.Second)
	}
	return
}

func BenchmarkHandle4Fields(b *testing.B) {
	results := []testResult{}
	benchmarkHandle4Fields(b, errorMapper.Handle(func(p httprequest.Params, arg *testParams4Fields) ([]testResult, error) {
		if arg.To.Before(arg.From.Time) {
			panic("unreachable")
		}
		if arg.Limit <= 0 {
			panic("unreachable")
		}
		return results, nil
	}).Handle)
}

func BenchmarkHandle4FieldsUnmarshalOnly(b *testing.B) {
	results := []testResult{}
	benchmarkHandle4Fields(b, errorMapper.HandleJSON(func(p httprequest.Params) (interface{}, error) {
		var arg testParams4Fields
		if err := httprequest.Unmarshal(p, &arg); err != nil {
			return nil, err
		}
		if arg.To.Before(arg.From.Time) {
			panic("unreachable")
		}
		if arg.Limit <= 0 {
			panic("unreachable")
		}
		return results, nil
	}))
}

func benchmarkHandle4Fields(b *testing.B, handle func(w http.ResponseWriter, req *http.Request, pvar httprouter.Params)) {
	// example taken from charmstore changes/published endpoint
	fromDate, err1 := time.Parse(dateFormat, "2010-10-10")
	toDate, err2 := time.Parse(dateFormat, "2011-11-11")
	if err1 != nil || err2 != nil {
		b.Fatalf("bad times")
	}
	rec := httptest.NewRecorder()
	params := httprequest.Params{
		Request: &http.Request{
			Form: url.Values{
				"limit": {"2000"},
				"from":  {fromDate.Format(dateFormat)},
				"to":    {toDate.Format(dateFormat)},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "id",
			Value: "someid",
		}},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec.Body.Reset()
		handle(rec, params.Request, params.PathVar)
	}
}

func BenchmarkHandle2StringFields(b *testing.B) {
	benchmarkHandleNFields(b, 2, errorMapper.Handle(func(p httprequest.Params, arg *testParams2StringFields) error {
		return nil
	}).Handle)
}

func BenchmarkHandle2StringFieldsUnmarshalOnly(b *testing.B) {
	benchmarkHandleNFields(b, 2, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams2StringFields
		return httprequest.Unmarshal(p, &arg)
	}))
}

func BenchmarkHandle2StringFieldsTrad(b *testing.B) {
	benchmarkHandleNFields(b, 2, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams2StringFields
		arg.Field0 = p.Request.Form.Get("Field0")
		arg.Field1 = p.Request.Form.Get("Field1")
		return nil
	}))
}

func BenchmarkHandle4StringFields(b *testing.B) {
	benchmarkHandleNFields(b, 4, errorMapper.Handle(func(p httprequest.Params, arg *testParams4StringFields) error {
		return nil
	}).Handle)
}

func BenchmarkHandle4StringFieldsUnmarshalOnly(b *testing.B) {
	benchmarkHandleNFields(b, 4, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams4StringFields
		return httprequest.Unmarshal(p, &arg)
	}))
}

func BenchmarkHandle4StringFieldsTrad(b *testing.B) {
	benchmarkHandleNFields(b, 4, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams4StringFields
		arg.Field0 = p.Request.Form.Get("Field0")
		arg.Field1 = p.Request.Form.Get("Field1")
		arg.Field2 = p.Request.Form.Get("Field2")
		arg.Field3 = p.Request.Form.Get("Field3")
		return nil
	}))
}

func BenchmarkHandle8StringFields(b *testing.B) {
	benchmarkHandleNFields(b, 8, errorMapper.Handle(func(p httprequest.Params, arg *testParams8StringFields) error {
		return nil
	}).Handle)
}

func BenchmarkHandle8StringFieldsUnmarshalOnly(b *testing.B) {
	benchmarkHandleNFields(b, 8, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams8StringFields
		return httprequest.Unmarshal(p, &arg)
	}))
}

func BenchmarkHandle8StringFieldsTrad(b *testing.B) {
	benchmarkHandleNFields(b, 8, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams8StringFields
		arg.Field0 = p.Request.Form.Get("Field0")
		arg.Field1 = p.Request.Form.Get("Field1")
		arg.Field2 = p.Request.Form.Get("Field2")
		arg.Field3 = p.Request.Form.Get("Field3")
		arg.Field4 = p.Request.Form.Get("Field4")
		arg.Field5 = p.Request.Form.Get("Field5")
		arg.Field6 = p.Request.Form.Get("Field6")
		arg.Field7 = p.Request.Form.Get("Field7")
		return nil
	}))
}

func BenchmarkHandle16StringFields(b *testing.B) {
	benchmarkHandleNFields(b, 16, errorMapper.Handle(func(p httprequest.Params, arg *testParams16StringFields) error {
		return nil
	}).Handle)
}

func BenchmarkHandle16StringFieldsUnmarshalOnly(b *testing.B) {
	benchmarkHandleNFields(b, 16, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams16StringFields
		return httprequest.Unmarshal(p, &arg)
	}))
}

func BenchmarkHandle16StringFieldsTrad(b *testing.B) {
	benchmarkHandleNFields(b, 16, errorMapper.HandleErrors(func(p httprequest.Params) error {
		var arg testParams16StringFields
		arg.Field0 = p.Request.Form.Get("Field0")
		arg.Field1 = p.Request.Form.Get("Field1")
		arg.Field2 = p.Request.Form.Get("Field2")
		arg.Field3 = p.Request.Form.Get("Field3")
		arg.Field4 = p.Request.Form.Get("Field4")
		arg.Field5 = p.Request.Form.Get("Field5")
		arg.Field6 = p.Request.Form.Get("Field6")
		arg.Field7 = p.Request.Form.Get("Field7")
		arg.Field8 = p.Request.Form.Get("Field8")
		arg.Field9 = p.Request.Form.Get("Field9")
		arg.Field10 = p.Request.Form.Get("Field10")
		arg.Field11 = p.Request.Form.Get("Field11")
		arg.Field12 = p.Request.Form.Get("Field12")
		arg.Field13 = p.Request.Form.Get("Field13")
		arg.Field14 = p.Request.Form.Get("Field14")
		arg.Field15 = p.Request.Form.Get("Field15")
		return nil
	}))
}

func benchmarkHandleNFields(b *testing.B, n int, handle func(w http.ResponseWriter, req *http.Request, pvar httprouter.Params)) {
	form := make(url.Values)
	for i := 0; i < n; i++ {
		form[fmt.Sprint("Field", i)] = []string{fmt.Sprintf("field %d", i)}
	}
	rec := httptest.NewRecorder()
	params := httprequest.Params{
		Request: &http.Request{
			Form: form,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec.Body.Reset()
		handle(rec, params.Request, params.PathVar)
	}
}
