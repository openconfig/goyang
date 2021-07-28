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

// This file implements Parse, which  parses the input as generic YANG and
// returns a slice of base Statements (which in turn may contain more
// Statements, i.e., a slice of Statement trees.)

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// a parser is used to parse the contents of a single .yang file.
type parser struct {
	lex    *lexer
	errout *bytes.Buffer
	tokens []*token // stack of pushed tokens (for backing up)

	// Depth of statements in nested braces
	statementDepth int

	// hitBrace is returned when we encounter a '}'.  The statement location
	// is updated with the location of the '}'.  The brace may be legitimate
	// but only the caller will know if it is.  That is, the brace may be
	// closing our parent or may be an error (we didn't expect it).
	// hitBrace is updated with the file, line, and column of the brace's
	// location.
	hitBrace *Statement
}

// Statement is a generic YANG statement that may have sub-statements.
// It implements the Node interface.
//
// Within the parser, it represents a non-terminal token.
// From https://tools.ietf.org/html/rfc7950#section-6.3:
// statement = keyword [argument] (";" / "{" *statement "}")
// The argument is a string.
type Statement struct {
	Keyword     string
	HasArgument bool
	Argument    string
	statements  []*Statement

	file string
	line int // 1's based line number
	col  int // 1's based column number
}

func (s *Statement) NName() string         { return s.Argument }
func (s *Statement) Kind() string          { return s.Keyword }
func (s *Statement) Statement() *Statement { return s }
func (s *Statement) ParentNode() Node      { return nil }
func (s *Statement) Exts() []*Statement    { return nil }

// Arg returns the optional argument to s.  It returns false if s has no
// argument.
func (s *Statement) Arg() (string, bool) { return s.Argument, s.HasArgument }

// SubStatements returns a slice of Statements found in s.
func (s *Statement) SubStatements() []*Statement { return s.statements }

// Location returns the location in the source where s was defined.
func (s *Statement) Location() string {
	switch {
	case s.file == "" && s.line == 0:
		return "unknown"
	case s.file == "":
		return fmt.Sprintf("line %d:%d", s.line, s.col)
	case s.line == 0:
		return s.file
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

// ignoreMe is an error recovery token used by the parser in order
// to continue processing for other errors in the file.
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
			fmt.Fprintf(p.errout, "%s:%d:%d: unexpected %c\n", ns.file, ns.line, ns.col, '}')
		default:
			statements = append(statements, ns)
		}
	}

	p.checkStatementDepthIsZero()

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

// next returns the next token from the lexer. If the next token is a
// concatenated string, it returns the concatenated string as the token.
func (p *parser) next() *token {
	if t := p.pop(); t != nil {
		return t
	}
	// next returns the next unprocessed lexer token.
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
	// Process string concatenation (both single and double quote).
	// See https://tools.ietf.org/html/rfc7950#section-6.1.3.1
	// The lexer trimmed the quotes already.
	for {
		nt := next()
		switch nt.Code() {
		case tEOF:
			return t
		case tUnquoted:
			if nt.Text != "+" {
				p.push(nt)
				return t
			}
		default:
			p.push(nt)
			return t
		}
		// Invariant: nt is a + sign.
		nnt := next()
		switch nnt.Code() {
		case tEOF:
			p.push(nt)
			return t
		case tString:
			// Accumulate the concatenation.
			t.Text += nnt.Text
		default:
			p.push(nnt, nt)
			return t
		}
	}
}

// nextStatement returns the next statement in the input, which may in turn
// recurse to read sub statements.
// nil is returned when EOF has been reached, or is reached halfway through
// parsing the next statement (with associated syntax errors printed to
// errout).
func (p *parser) nextStatement() *Statement {
	t := p.next()
	switch t.Code() {
	case tEOF:
		return nil
	case '}':
		p.statementDepth -= 1
		p.hitBrace.file = t.File
		p.hitBrace.line = t.Line
		p.hitBrace.col = t.Col
		return p.hitBrace
	case tUnquoted:
	default:
		fmt.Fprintf(p.errout, "%v: keyword token not an unquoted string\n", t)
		return ignoreMe
	}
	// Invariant: t represents a keyword token.

	s := &Statement{
		Keyword: t.Text,
		file:    t.File,
		line:    t.Line,
		col:     t.Col,
	}

	// The keyword "pattern" must be treated specially. When
	// parsing the argument for "pattern", escape sequences
	// must be expanded differently.
	p.lex.inPattern = t.Text == "pattern"
	t = p.next()
	p.lex.inPattern = false
	switch t.Code() {
	case tString, tUnquoted:
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
	case '{':
		p.statementDepth += 1
		for {
			switch ns := p.nextStatement(); ns {
			case nil:
				// Signal EOF reached.
				return nil
			case p.hitBrace:
				return s
			default:
				s.statements = append(s.statements, ns)
			}
		}
	default:
		fmt.Fprintf(p.errout, "%v: syntax error, expected ';' or '{'\n", t)
		return ignoreMe
	}
}

// checkStatementDepthIsZero checks that we aren't missing closing
// braces. Note: the parser will error out for the case where we
// start with an unmatched close brace, i.e. depth < 0
//
// This test should only be done if there are no other errors as
// we may exit early due to those errors -- and therefore there *might*
// not really be a mismatched brace issue.
func (p *parser) checkStatementDepthIsZero() {
	if p.errout.Len() > 0 || p.statementDepth == 0 {
		return
	}

	plural := ""
	if p.statementDepth > 1 {
		plural = "s"
	}
	fmt.Fprintf(p.errout, "%s:%d:%d: missing %d closing brace%s\n",
		p.lex.file, p.lex.line, p.lex.col, p.statementDepth, plural)
}
