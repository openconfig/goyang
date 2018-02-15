// Copyright 2015 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yang

import (
	"bytes"
	"testing"
)

func (s1 *Statement) equal(s2 *Statement) bool {
	if s1.Keyword != s2.Keyword ||
		s1.HasArgument != s2.HasArgument ||
		s1.Argument != s2.Argument ||
		len(s1.statements) != len(s2.statements) {
		return false
	}

	for x, ss := range s1.statements {
		if !ss.equal(s2.statements[x]) {
			return false
		}
	}
	return true
}

// SA returns a statement with an argument and optional substatements.
func SA(k, a string, ss ...*Statement) *Statement {
	return &Statement{
		Keyword:     k,
		Argument:    a,
		HasArgument: true,
		statements:  ss,
	}
}

// S returns a statement with no argument and optional substatements.
func S(k string, ss ...*Statement) *Statement {
	return &Statement{
		Keyword:    k,
		statements: ss,
	}
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		line int
		in   string
		out  []*Statement
		err  string
	}{
		{line: line()},
		{line: line(), in: `
foo;
`,
			out: []*Statement{
				S("foo"),
			},
		},
		{line: line(), in: `
foo {}
`,
			out: []*Statement{
				S("foo"),
			},
		},
		{line: line(), in: `
foo "";
`,
			out: []*Statement{
				SA("foo", ""),
			},
		},
		{line: line(), in: `
foo bar;
`,
			out: []*Statement{
				SA("foo", "bar"),
			},
		},
		{line: line(), in: `
foo "bar";
`,
			out: []*Statement{
				SA("foo", "bar"),
			},
		},
		{line: line(), in: `
foo "\\ \S \n";
`,
			err: `test.yang:2:5: invalid escape sequence: \S`,
		},
		{line: line(), in: `
pattern "\\ \S \n";
`,
			out: []*Statement{
				SA("pattern", `\ \S 
`),
			},
		},
		{line: line(), in: `
foo '\\ \S \n';
`,
			out: []*Statement{
				SA("foo", `\\ \S \n`),
			},
		},
		{line: line(), in: `
pattern '\\ \S \n';
`,
			out: []*Statement{
				SA("pattern", `\\ \S \n`),
			},
		},
		{line: line(), in: `
foo bar;
red black;
`,
			out: []*Statement{
				SA("foo", "bar"),
				SA("red", "black"),
			},
		},
		{line: line(), in: `
foo {
   key value;
}
`,
			out: []*Statement{
				S("foo",
					SA("key", "value"),
				),
			},
		},
		{line: line(), in: `
foo {
   key value;
}
`,
			out: []*Statement{
				S("foo",
					SA("key", "value"),
				),
			},
		},
		{line: line(), in: `
foo {
   key value;
   key2;
}
`,
			out: []*Statement{
				S("foo",
					SA("key", "value"),
					S("key2"),
				),
			},
		},
		{line: line(), in: `
foo1 {
   key value1;
}
foo2 {
   key value2;
}
foo3 value3;
`,
			out: []*Statement{
				S("foo1",
					SA("key", "value1"),
				),
				S("foo2",
					SA("key", "value2"),
				),
				SA("foo3", "value3"),
			},
		},
		{line: line(), in: `
foo1 {
    key value1;
    foo2 {
        key value2;
    }
}
`,
			out: []*Statement{
				S("foo1",
					SA("key", "value1"),
					S("foo2",
						SA("key", "value2"),
					),
				),
			},
		},
		{line: line(), in: `
 }
`,
			err: `test.yang:2:2: unexpected }`,
		},
		{line: line(), in: `
id
`,
			err: `test.yang: unexpected EOF`,
		},
		{line: line(), in: `
   {
`,
			err: `test.yang:2:4: {: not an identifier`,
		},
		{line: line(), in: `
;
`,
			err: `test.yang:2:1: ;: not an identifier`,
		},
		{line: line(), in: `
statement one two { }
`,
			err: `test.yang:2:15: two: syntax error
test.yang:2:19: {: not an identifier
test.yang:2:21: unexpected }`,
		},
		{line: line(), in: `
    }
foo {
	key: "value";
}
`,
			err: `test.yang:2:5: unexpected }`,
		},
		{line: line(), in: `
{
	something: "bad";
}
foo {
	key: "\Value";
	key2: "value2";
	bar {
		key3: "value\3;
	}
}`,
			err: `test.yang:2:1: {: not an identifier
test.yang:4:1: unexpected }
test.yang:6:7: invalid escape sequence: \V
test.yang:9:9: invalid escape sequence: \3
test.yang:9:9: missing closing "
test.yang: unexpected EOF`,
		},
	} {
		s, err := Parse(tt.in, "test.yang")
		if (s == nil) != (tt.out == nil) {
			if s == nil {
				t.Errorf("%d: did not get expected statements: %s", tt.line, tt.out)
			} else {
				t.Errorf("%d: get unexpected statements: %s", tt.line, s)
			}
		}
		switch {
		case err == nil && tt.err == "":
		case tt.err == "":
			t.Errorf("%d: unexpected error %v", tt.line, err)
			continue
		case err == nil:
			t.Errorf("%d: did not get expected error %v", tt.line, tt.err)
			continue
		case err.Error() == tt.err:
			continue
		default:
			t.Errorf("%d: got error:\n%s\nwant:\n%s", tt.line, err, tt.err)
			continue
		}
		s1 := &Statement{statements: s}
		s2 := &Statement{statements: tt.out}
		if !s1.equal(s2) {
			t.Errorf("%d: got:\n%s\nwant:\n%s", tt.line, s1, s2)
		}
	}
}

func TestWrite(t *testing.T) {
Testing:
	for _, tt := range []struct {
		line int
		in   string
		out  string
	}{
		{line: line(),
			in: `key arg { substatement; }`,
			out: `key "arg" {
	substatement;
}
`,
		},
		{line: line(),
			in: `key { substatement { key arg; }}`,
			out: `key {
	substatement {
		key "arg";
	}
}
`,
		},
		{line: line(),
			in: `
module base {
   namespace "urn:mod";
   prefix "base";

   typedef base-type { type int32; }

   grouping base-group {
     description
       "The base-group is used to test the
        'uses' statement below.  This description
        is here to simply include a multi-line
        string as an example of multi-line strings";
     leaf base-group-leaf {
       config false;
       type string;
     }
   }
   uses base-group;
}
`, out: `module "base" {
	namespace "urn:mod";
	prefix "base";
	typedef "base-type" {
		type "int32";
	}
	grouping "base-group" {
		description "The base-group is used to test the
		             'uses' statement below.  This description
		             is here to simply include a multi-line
		             string as an example of multi-line strings";
		leaf "base-group-leaf" {
			config "false";
			type "string";
		}
	}
	uses "base-group";
}
`,
		},
	} {
		in := tt.in
		// Run twice.  The first time we are parsing tt.in, the second
		// time we are parsing the output from the first parsing.
		for i := 0; i < 2; i++ {
			s, err := Parse(in, "test.yang")
			if err != nil {
				t.Errorf("%d: unexpected error %v", tt.line, err)
				continue Testing
			}
			if len(s) != 1 {
				t.Errorf("%d: got %d statements, expected 1", tt.line, len(s))
				continue Testing
			}
			var buf bytes.Buffer
			s[0].Write(&buf, "")
			out := buf.String()
			if out != tt.out {
				t.Errorf("%d: got:\n%swant:\n%s", tt.line, out, tt.out)
				continue Testing
			}
			in = out
		}
	}
}
