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

// Thile file implements Parse, which  parses the input as generic YANG and
// returns a slice of base Statements (which in turn may contain more
// Statements, i.e., a slice of Statement trees.)
//
// TODO(borman): remove this TODO once ast.go is part of of this package.
// See ast.go for the conversion of Statements into an AST tree of Nodes.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// a parser is used to parse the contents of a single .yang file.
type parser struct {
	lex        *lexer
	errout     *bytes.Buffer
	tokens     []*token     // stack of pushed tokens (for backing up)
	statements []*Statement // list of root statements

	// hitBrace is returned when we encounter a '}'.  The statement location
	// is updated with the location of the '}'.  The brace may be legitimate
	// but only the caller will know if it is.  That is, the brace may be
	// closing our parent or may be an error (we didn't expect it).
	// hitBrace is updated with the file, line, and column of the brace's
	// location.
	hitBrace *Statement
}

// A Statement is a generic YANG statement.  A Statement may have optional
// sub-statement (i.e., a Statement is a tree).
type Statement struct {
	Keyword     string
	HasArgument bool
	Argument    string
	statements  []*Statement

	file string
	line int // 1's based line number
	col  int // 1's based column number
}

// FakeStatement returns a statement filled in with keyword, file, line and col.
func FakeStatement(keyword, file string, line, col int) *Statement {
	return &Statement{
		Keyword: keyword,
		file:    file,
		line:    line,
		col:     col,
	}
}

// Make Statement statisfy Node

func (s *Statement) NName() string         { return s.Argument }
func (s *Statement) Kind() string          { return s.Keyword }
func (s *Statement) Statement() *Statement { return s }
func (s *Statement) ParentNode() Node      { return nil }
func (s *Statement) Exts() []*Statement    { return nil }

// Arg returns the optional argument to s.  It returns false if s has no
// argument.
func (s *Statement) Arg() (string, bool) { return s.Argument, s.HasArgument }

// Keyword returns the keyword of s.
//func (s *Statement) Keyword() string { return s.Keyword }

// SubStatements returns a slice of Statements found in s.
func (s *Statement) SubStatements() []*Statement { return s.statements }

// String returns s's tree as a string.
func (s *Statement) String() string {
	var b bytes.Buffer
	s.Write(&b, "")
	return b.String()
}

// Location returns the loction in the source where s was defined.
func (s *Statement) Location() string {
	switch {
	case s.file == "" && s.line == 0:
		return "unknown"
	case s.file == "":
		return fmt.Sprintf("line %d:%d", s.line, s.col)
	case s.line == 0:
		return fmt.Sprintf("%s", s.file)
	default:
		return fmt.Sprintf("%s:%d:%d", s.file, s.line, s.col)
	}
}

// Write writes the tree in s to w, each line indented by ident.  Children
// nodes are indented further by a tab.  Typically indent is "" at the top
// level.  Write is intended to display the contents of Statement, but
// not necessarily reproduce the input of Statement.
func (s *Statement) Write(w io.Writer, indent string) error {
	if s.Keyword == "" {
		// We are just a collection of statements at the top level.
		for _, s := range s.statements {
			if err := s.Write(w, indent); err != nil {
				return err
			}
		}
		return nil
	}

	parts := []string{fmt.Sprintf("%s%s", indent, s.Keyword)}
	if s.HasArgument {
		args := strings.Split(s.Argument, "\n")
		if len(args) == 1 {
			parts = append(parts, fmt.Sprintf(" %q", s.Argument))
		} else {
			parts = append(parts, ` "`, args[0], "\n")
			i := fmt.Sprintf("%*s", len(s.Keyword)+1, "")
			for x, p := range args[1:] {
				s := fmt.Sprintf("%q", p)
				s = s[1 : len(s)-1]
				parts = append(parts, indent, " ", i, s)
				if x == len(args[1:])-1 {
					// last part just needs the closing "
					parts = append(parts, `"`)
				} else {
					parts = append(parts, "\n")
				}
			}
		}
	}

	if len(s.statements) == 0 {
		_, err := fmt.Fprintf(w, "%s;\n", strings.Join(parts, ""))
		return err
	}
	if _, err := fmt.Fprintf(w, "%s {\n", strings.Join(parts, "")); err != nil {
		return err
	}
	for _, s := range s.statements {
		if err := s.Write(w, indent+"\t"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s}\n", indent); err != nil {
		return err
	}
	return nil
}

// ignoreMe is returned to continue processing after an error (the parse will
// fail, but we want to look for more errors).
var ignoreMe = &Statement{}

// Parse parses the input as generic YANG and returns the statements parsed.
// The path parameter should be the source name where input was read from (e.g.,
// the file name the input was read from).  If one more more errors are
// encountered, nil and an error are returned.  The error's text includes all
// errors encountered.
func Parse(input, path string) ([]*Statement, error) {
	var statements []*Statement
	p := &parser{
		lex:      newLexer(input, path),
		errout:   &bytes.Buffer{},
		hitBrace: &Statement{},
	}
	p.lex.errout = p.errout
Loop:
	for {
		switch ns := p.nextStatement(); ns {
		case nil:
			break Loop
		case p.hitBrace:
			fmt.Fprintf(p.errout, "%s:%d:%d: unexpected %c\n", ns.file, ns.line, ns.col, closeBrace)
		default:
			statements = append(statements, ns)
		}
	}

	if p.errout.Len() == 0 {
		return statements, nil
	}
	return nil, errors.New(strings.TrimSpace(p.errout.String()))

}

// push pushes tokens t back on the input stream so they will be the next
// tokens returned by next.  The tokens list is a LIFO so the final token
// listed to push will be the next token returned.
func (p *parser) push(t ...*token) {
	p.tokens = append(p.tokens, t...)
}

// pop returns the last token pushed, or nil if the token stack is empty.
func (p *parser) pop() *token {
	if n := len(p.tokens); n > 0 {
		n--
		defer func() { p.tokens = p.tokens[:n] }()
		return p.tokens[n]
	}
	return nil
}

// next returns the next token from the lexer.  It also handles string
// concatenation.
func (p *parser) next() *token {
	if t := p.pop(); t != nil {
		return t
	}
	next := func() *token {
		for {
			if t := p.lex.NextToken(); t.Code() != tError {
				return t
			}
		}
	}
	t := next()
	if t.Code() != tString {
		return t
	}
	// Handle `"string" + "string"`
	for {
		nt := next()
		switch nt.Code() {
		case tEOF:
			return t
		case tIdentifier:
			if nt.Text != "+" {
				p.push(nt)
				return t
			}
		default:
			p.push(nt)
			return t
		}
		// We found a +, now look for a following string
		st := next()
		switch st.Code() {
		case tEOF:
			p.push(nt)
			return t
		case tString:
			// concatenate the text and drop the nt and st tokens
			// try again
			t.Text += st.Text
		default:
			p.push(st, nt)
			return t
		}

	}
}

// nextStatement returns the next statement in the input, which may in turn
// recurse to read sub statements.
func (p *parser) nextStatement() *Statement {
	t := p.next()
	switch t.Code() {
	case tEOF:
		return nil
	case closeBrace:
		p.hitBrace.file = t.File
		p.hitBrace.line = t.Line
		p.hitBrace.col = t.Col
		return p.hitBrace
	case tIdentifier:
	default:
		fmt.Fprintf(p.errout, "%v: not an identifier\n", t)
		return ignoreMe
	}

	s := &Statement{
		Keyword: t.Text,
		file:    t.File,
		line:    t.Line,
		col:     t.Col,
	}

	// The keyword "pattern" must be treated special.  When
	// parsing the argument for "pattern", escape sequences
	// must be expanded differently.
	p.lex.inPattern = t.Text == "pattern"
	t = p.next()
	p.lex.inPattern = false
	switch t.Code() {
	case tString, tIdentifier:
		s.HasArgument = true
		s.Argument = t.Text
		t = p.next()
	}
	switch t.Code() {
	case tEOF:
		fmt.Fprintf(p.errout, "%s: unexpected EOF\n", s.file)
		return nil
	case ';':
		return s
	case openBrace:
		for {
			switch ns := p.nextStatement(); ns {
			case nil:
				return nil
			case p.hitBrace:
				return s
			default:
				s.statements = append(s.statements, ns)
			}
		}
	default:
		fmt.Fprintf(p.errout, "%v: syntax error\n", t)
		return ignoreMe
	}
}
