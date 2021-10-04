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
	"runtime"
	"testing"
)

// line returns the line number from which it was called.
// Used to mark where test entries are in the source.
func line() int {
	_, _, line, _ := runtime.Caller(1)
	return line

}

// Equal returns true if t and tt are equal (have the same code and text),
// false if not.
func (t *token) Equal(tt *token) bool {
	return t.code == tt.code && t.Text == tt.Text
}

// T Creates a new token from the provided code and string.
func T(c code, text string) *token { return &token{code: c, Text: text} }

func TestLex(t *testing.T) {
Tests:
	for _, tt := range []struct {
		line   int
		in     string
		tokens []*token
	}{
		{line(), "", nil},
		{line(), "bob", []*token{
			T(tUnquoted, "bob"),
		}},
		{line(), "bob //bob", []*token{
			T(tUnquoted, "bob"),
		}},
		{line(), "/the/path", []*token{
			T(tUnquoted, "/the/path"),
		}},
		{line(), "+the/path", []*token{
			T(tUnquoted, "+the/path"),
		}},
		{line(), "+the+path", []*token{
			T(tUnquoted, "+the+path"),
		}},
		{line(), "+ the/path", []*token{
			T(tUnquoted, "+"),
			T(tUnquoted, "the/path"),
		}},
		{line(), "{bob}", []*token{
			T('{', "{"),
			T(tUnquoted, "bob"),
			T('}', "}"),
		}},
		{line(), "bob;fred", []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), "\t bob\t; fred ", []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), `
	bob;
	fred
`, []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), `
	// This is a comment
	bob;
	fred
`, []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), `
	/* This is a comment */
	bob;
	fred
`, []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), `
	/*
	 * This is a comment
	 */
	bob;
	fred
`, []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), `
	bob; // This is bob
	fred // This is fred
`, []*token{
			T(tUnquoted, "bob"),
			T(';', ";"),
			T(tUnquoted, "fred"),
		}},
		{line(), `
pattern '[a-zA-Z0-9!#$%&'+"'"+'*+/=?^_` + "`" + `{|}~-]+';
`, []*token{
			T(tUnquoted, "pattern"),
			T(tString, "[a-zA-Z0-9!#$%&"),
			T(tUnquoted, "+"),
			T(tString, "'"),
			T(tUnquoted, "+"),
			T(tString, "*+/=?^_`{|}~-]+"),
			T(';', ";"),
		}},
		{line(), `
// tab indent both lines
	"Broken
	line"
`, []*token{
			T(tString, "Broken\nline"),
		}},
		{line(), `
// tab indent both lines, trailing spaces and tabs
	"Broken
	 line"
`, []*token{
			T(tString, "Broken\nline"),
		}},
		{line(), `
// tab indent first line, spaces and tab second line
	"Broken
    	 line"
`, []*token{
			T(tString, "Broken\nline"),
		}},
		{line(), `
// tab indent first line, spaces second linfe
	"Broken
         line"
`, []*token{
			T(tString, "Broken\nline"),
		}},
		{line(), `
// extra space in second line
	"Broken
          space"
`, []*token{
			T(tString, "Broken\n space"),
		}},
		{line(), `
// spaces first line, tab on second
       "Broken
	space"
`, []*token{
			T(tString, "Broken\nspace"),
		}},
		{line(), `
// Odd indenting
   "Broken
  space"
`, []*token{
			T(tString, "Broken\nspace"),
		}},
		{line(), `
// Odd indenting
   "Broken  \t
  space with trailing space"
`, []*token{
			T(tString, "Broken\nspace with trailing space"),
		}},
	} {
		l := newLexer(tt.in, "")
		// l.debug = true
		for i := 0; ; i++ {
			token := l.NextToken()
			if token == nil {
				if len(tt.tokens) != i {
					t.Errorf("%d: got %d tokens, want %d", tt.line, i, len(tt.tokens))
				}
				continue Tests
			}
			if len(tt.tokens) > i && !token.Equal(tt.tokens[i]) {
				t.Errorf("%d, %d: got (%v, %q) want (%v, %q)", tt.line, i, token.code, token.Text, tt.tokens[i].code, tt.tokens[i].Text)
			}
		}
	}
}

func TestLexErrors(t *testing.T) {
	for _, tt := range []struct {
		line   int
		in     string
		errcnt int
		errs   string
	}{
		{line(),
			`1: "no closing quote`,
			1,
			`test.yang:1:4: missing closing "
`,
		},
		{line(),
			`1: on another line
2: there is "no closing quote\"`,
			1,
			`test.yang:2:13: missing closing "
`,
		},
		{line(),
			`1:
2: "Mares eat oats,"
3: "And does eat oats,"
4: "But little lambs eat ivy,"
5: "and if I were a little lamb,"
6: "I'ld eat ivy too.
5: So saith the sage.`,
			1,
			`test.yang:6:4: missing closing "
`,
		},
		{line(),
			`1:
2: "Quoted string"
3: "Missing quote
4: "Another quoted string"
`,
			1,
			`test.yang:4:26: missing closing "
`,
		},
		{line(),
			`1:
2: 'Quoted string'
3: 'Missing quote
4: 'Another quoted string'
`,
			1,
			`test.yang:4:26: missing closing '
`,
		},
		{line(),
			`1: "Quoted string\"
2: Missing end-quote\q`,
			2,
			`test.yang:2:21: invalid escape sequence: \q
test.yang:1:4: missing closing "
`,
		},
		{line(),
			`/* This is a comment
without an ending.
`,
			1,
			`test.yang:1:1: missing closing */
`,
		},
		{line(),
			// Two errors too many.
			`yang-version 1.1;description "\/\/\/\/\/\/\/\/\/\/";`,
			9,
			`test.yang:1:31: invalid escape sequence: \/
test.yang:1:33: invalid escape sequence: \/
test.yang:1:35: invalid escape sequence: \/
test.yang:1:37: invalid escape sequence: \/
test.yang:1:39: invalid escape sequence: \/
test.yang:1:41: invalid escape sequence: \/
test.yang:1:43: invalid escape sequence: \/
test.yang:1:45: invalid escape sequence: \/
` + tooMany,
		},
	} {
		l := newLexer(tt.in, "test.yang")
		errbuf := &bytes.Buffer{}
		l.errout = errbuf
		for l.NextToken() != nil {

		}
		if l.errcnt != tt.errcnt {
			t.Errorf("%d: got %d errors, want %v", tt.line, l.errcnt, tt.errcnt)
		}
		errs := errbuf.String()
		if errs != tt.errs {
			t.Errorf("%d: got errors:\n%s\nwant:\n%s", tt.line, errs, tt.errs)
		}
	}
}
