// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	jc "github.com/juju/testing/checkers"
	"github.com/julienschmidt/httprouter"
	gc "gopkg.in/check.v1"
)

type testSuite struct{}

var _ = gc.Suite(&testSuite{})

func body(s string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(s))
}

var unmarshalTests = []struct {
	about       string
	val         interface{}
	expect      interface{}
	params      Params
	expectError string
}{{
	about: "struct with simple fields",
	val: struct {
		F1          int    `httprequest:",form"`
		F2          int    `httprequest:",form"`
		G1          string `httprequest:",path"`
		G2          string `httprequest:",path"`
		H           string `httprequest:",body"`
		UnknownForm string `httprequest:",form"`
		UnknownPath string `httprequest:",path"`
	}{
		F1: 99,
		F2: -35,
		G1: "g1 val",
		G2: "g2 val",
		H:  "h val",
	},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"F1": {"99"},
				"F2": {"-35", "not a number"},
			},
			Body: body(`"h val"`),
		},
		PathVar: httprouter.Params{{
			Key:   "G2",
			Value: "g2 val",
		}, {
			Key:   "G1",
			Value: "g1 val",
		}, {
			Key:   "G1",
			Value: "g1 wrong val",
		}},
	},
}, {
	about: "struct with renamed fields",
	val: struct {
		F1 int    `httprequest:"x1,form"`
		F2 int    `httprequest:"x2,form"`
		G1 string `httprequest:"g1,path"`
		G2 string `httprequest:"g2,path"`
	}{
		F1: 99,
		F2: -35,
		G1: "g1 val",
		G2: "g2 val",
	},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"x1": {"99"},
				"x2": {"-35", "not a number"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "g2",
			Value: "g2 val",
		}, {
			Key:   "g1",
			Value: "g1 val",
		}, {
			Key:   "g1",
			Value: "g1 wrong val",
		}},
	},
}, {
	about: "fields without httprequest tags are ignored",
	val: struct {
		F int
	}{},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"foo"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "F",
			Value: "foo",
		}},
	},
}, {
	about: "pointer fields are filled out",
	val: struct {
		F *int `httprequest:",form"`
		*SFG
		S *string `httprequest:",form"`
		T *string `httprequest:",form"`
	}{
		F: newInt(99),
		SFG: &SFG{
			F: 0,
			G: 534,
		},
		S: newString("s val"),
	},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"99"},
				"G": {"534"},
				"S": {"s val"},
			},
		},
	},
}, {
	about: "UnmarshalText called on TextUnmarshalers",
	val: struct {
		F  exclamationUnmarshaler  `httprequest:",form"`
		G  exclamationUnmarshaler  `httprequest:",path"`
		FP *exclamationUnmarshaler `httprequest:",form"`
	}{
		F:  "yes!",
		G:  "no!",
		FP: (*exclamationUnmarshaler)(newString("maybe!")),
	},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"F":  {"yes"},
				"FP": {"maybe"},
			},
		},
		PathVar: httprouter.Params{{
			Key:   "G",
			Value: "no",
		}},
	},
}, {
	about: "UnmarshalText not called on values with a non-TextUnmarshaler UnmarshalText method",
	val: struct {
		F notTextUnmarshaler `httprequest:",form"`
	}{
		F: "hello",
	},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"F": {"hello"},
			},
		},
	},
}, {
	about: "UnmarshalText returning an error",
	val: struct {
		F exclamationUnmarshaler `httprequest:",form"`
	}{},
	params: Params{
		Request: &http.Request{},
	},
	expectError: "empty string!",
}, {
	about: "all field form values",
	val: struct {
		A []string  `httprequest:",form"`
		B *[]string `httprequest:",form"`
		C []string  `httprequest:",form"`
		D *[]string `httprequest:",form"`
	}{
		A: []string{"a1", "a2"},
		B: func() *[]string {
			x := []string{"b1", "b2", "b3"}
			return &x
		}(),
	},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"A": {"a1", "a2"},
				"B": {"b1", "b2", "b3"},
			},
		},
	},
}, {
	about: "invalid scan field",
	val: struct {
		A int `httprequest:",form"`
	}{},
	params: Params{
		Request: &http.Request{
			Form: url.Values{
				"A": {"not an int"},
			},
		},
	},
	expectError: `cannot parse "not an int" into int: expected integer`,
}, {
	about: "scan field not present",
	val: struct {
		A int `httprequest:",form"`
	}{},
	params: Params{
		Request: &http.Request{},
	},
}, {
	about: "invalid JSON body",
	val: struct {
		A string `httprequest:",body"`
	}{},
	params: Params{
		Request: &http.Request{
			Body: body("invalid JSON"),
		},
	},
	expectError: "cannot unmarshal request body: invalid character 'i' looking for beginning of value",
}, {
	about: "body with read error",
	val: struct {
		A string `httprequest:",body"`
	}{},
	params: Params{
		Request: &http.Request{
			Body: errorReader("some error"),
		},
	},
	expectError: "cannot read request body: some error",
}, {
	about: "[]string not allowed for URL source",
	val: struct {
		A []string `httprequest:",path"`
	}{},
	expectError: `bad type .*: invalid target type \[]string for path parameter`,
}, {
	about: "duplicated body",
	val: struct {
		B1 int    `httprequest:",body"`
		B2 string `httprequest:",body"`
	}{},
	expectError: "bad type .*: more than one body field specified",
}, {
	about: "body tag name is ignored",
	val: struct {
		B string `httprequest:"foo,body"`
	}{
		B: "hello",
	},
	params: Params{
		Request: &http.Request{
			Body: body(`"hello"`),
		},
	},
}, {
	about: "tag with invalid source",
	val: struct {
		B1 int `httprequest:",xxx"`
	}{},
	expectError: `bad type .*: bad tag "httprequest:\\",xxx\\"" in field B1: unknown tag flag "xxx"`,
}, {
	about:       "non-struct pointer",
	val:         0,
	expectError: `bad type \*int: type is not pointer to struct`,
}}

func (*testSuite) TestUnmarshal(c *gc.C) {
	for i, test := range unmarshalTests {
		c.Logf("%d: %s", i, test.about)
		t := reflect.TypeOf(test.val)
		fillv := reflect.New(t)
		err := Unmarshal(test.params, fillv.Interface())
		if test.expectError != "" {
			c.Assert(err, gc.ErrorMatches, test.expectError)
			continue
		}
		c.Assert(fillv.Elem().Interface(), jc.DeepEquals, test.val)
	}
}

type notTextUnmarshaler string

// UnmarshalText does *not* implement encoding.TextUnmarshaler
// (it has no arguments or error return value)
func (t *notTextUnmarshaler) UnmarshalText() {
	panic("unexpected call")
}

type exclamationUnmarshaler string

func (t *exclamationUnmarshaler) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("empty string!")
	}
	*t = exclamationUnmarshaler(b) + "!"
	return nil
}

// TODO non-pointer struct

type structField struct {
	name  string
	index []int
}

var fieldsTests = []struct {
	about  string
	val    interface{}
	expect []structField
}{{
	about: "simple struct",
	val: struct {
		A int
		B string
		C bool
	}{},
	expect: []structField{{
		name:  "A",
		index: []int{0},
	}, {
		name:  "B",
		index: []int{1},
	}, {
		name:  "C",
		index: []int{2},
	}},
}, {
	about: "non-embedded struct member",
	val: struct {
		A struct {
			X int
		}
	}{},
	expect: []structField{{
		name:  "A",
		index: []int{0},
	}},
}, {
	about: "embedded exported struct",
	val: struct {
		SFG
	}{},
	expect: []structField{{
		name:  "SFG",
		index: []int{0},
	}, {
		name:  "F",
		index: []int{0, 0},
	}, {
		name:  "G",
		index: []int{0, 1},
	}},
}, {
	about: "embedded unexported struct",
	val: struct {
		sFG
	}{},
	expect: []structField{{
		name:  "sFG",
		index: []int{0},
	}, {
		name:  "F",
		index: []int{0, 0},
	}, {
		name:  "G",
		index: []int{0, 1},
	}},
}, {
	about: "two embedded structs with cancelling members",
	val: struct {
		SFG
		SF
	}{},
	expect: []structField{{
		name:  "SFG",
		index: []int{0},
	}, {
		name:  "G",
		index: []int{0, 1},
	}, {
		name:  "SF",
		index: []int{1},
	}},
}, {
	about: "embedded structs with same fields at different depths",
	val: struct {
		SFGH3
		SG1
		SFG2
		SF2
		L int
	}{},
	expect: []structField{{
		name:  "SFGH3",
		index: []int{0},
	}, {
		name:  "SFGH2",
		index: []int{0, 0},
	}, {
		name:  "SFGH1",
		index: []int{0, 0, 0},
	}, {
		name:  "SFGH",
		index: []int{0, 0, 0, 0},
	}, {
		name:  "H",
		index: []int{0, 0, 0, 0, 2},
	}, {
		name:  "SG1",
		index: []int{1},
	}, {
		name:  "SG",
		index: []int{1, 0},
	}, {
		name:  "G",
		index: []int{1, 0, 0},
	}, {
		name:  "SFG2",
		index: []int{2},
	}, {
		name:  "SFG1",
		index: []int{2, 0},
	}, {
		name:  "SFG",
		index: []int{2, 0, 0},
	}, {
		name:  "SF2",
		index: []int{3},
	}, {
		name:  "SF1",
		index: []int{3, 0},
	}, {
		name:  "SF",
		index: []int{3, 0, 0},
	}, {
		name:  "L",
		index: []int{4},
	}},
}, {
	about: "embedded pointer struct",
	val: struct {
		*SF
	}{},
	expect: []structField{{
		name:  "SF",
		index: []int{0},
	}, {
		name:  "F",
		index: []int{0, 0},
	}},
}}

type SFG struct {
	F int `httprequest:",form"`
	G int `httprequest:",form"`
}

type SFG1 struct {
	SFG
}

type SFG2 struct {
	SFG1
}

type SFGH struct {
	F int `httprequest:",form"`
	G int `httprequest:",form"`
	H int `httprequest:",form"`
}

type SFGH1 struct {
	SFGH
}

type SFGH2 struct {
	SFGH1
}

type SFGH3 struct {
	SFGH2
}

type SF struct {
	F int `httprequest:",form"`
}

type SF1 struct {
	SF
}

type SF2 struct {
	SF1
}

type SG struct {
	G int `httprequest:",form"`
}

type SG1 struct {
	SG
}

type sFG struct {
	F int `httprequest:",form"`
	G int `httprequest:",form"`
}

func (*testSuite) TestFields(c *gc.C) {
	for i, test := range fieldsTests {
		c.Logf("%d: %s", i, test.about)
		t := reflect.TypeOf(test.val)
		got := fields(t)
		c.Assert(got, gc.HasLen, len(test.expect))
		for j, field := range got {
			expect := test.expect[j]
			c.Logf("field %d: %s", j, expect.name)
			gotField := t.FieldByIndex(field.Index)
			// Unfortunately, FieldByIndex does not return
			// a field with the same index that we passed in,
			// so we set it to the expected value so that
			// it can be compared later with the result of FieldByName.
			gotField.Index = field.Index
			expectField := t.FieldByIndex(expect.index)
			// ditto.
			expectField.Index = expect.index
			c.Assert(gotField, jc.DeepEquals, expectField)

			// Sanity check that we can actually access the field by the
			// expected name.
			expectField1, ok := t.FieldByName(expect.name)
			c.Assert(ok, jc.IsTrue)
			c.Assert(expectField1, jc.DeepEquals, expectField)
		}
	}
}

func newInt(i int) *int {
	return &i
}

func newString(s string) *string {
	return &s
}

type errorReader string

func (r errorReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("%s", r)
}

func (r errorReader) Close() error {
	return nil
}
