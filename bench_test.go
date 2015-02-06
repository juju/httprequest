// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/juju/httprequest"
	"github.com/julienschmidt/httprouter"
)

const dateFormat = "2006-01-02"

func BenchmarkUnmarshal4Fields(b *testing.B) {
	fromDate, err1 := time.Parse(dateFormat, "2010-10-10")
	toDate, err2 := time.Parse(dateFormat, "2011-11-11")
	if err1 != nil || err2 != nil {
		b.Fatalf("bad times")
	}
	type P struct {
		Id    string   `httprequest:"id,path"`
		Limit int      `httprequest:"limit,form"`
		From  dateTime `httprequest:"from,form"`
		To    dateTime `httprequest:"to,form"`
	}
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

type dateTime struct {
	time.Time
}

func (dt *dateTime) UnmarshalText(b []byte) (err error) {
	dt.Time, err = time.Parse(dateFormat, string(b))
	return
}
