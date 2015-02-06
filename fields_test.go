// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest

import (
	"reflect"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

type fieldsSuite struct{}

var _ = gc.Suite(&fieldsSuite{})

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
}, {
	about: "embedded not a pointer",
	val: struct {
		M
	}{},
	expect: []structField{{
		name:  "M",
		index: []int{0},
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

type M map[string]interface{}

func (*fieldsSuite) TestFields(c *gc.C) {
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
