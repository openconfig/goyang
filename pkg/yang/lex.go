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

// This file implements the lexical tokenization of yang.  The lexer returns
// a series of tokens with one of the following codes:
//
//    tError       // an error was encountered
//    tEOF         // end-of-file
//    tString      // A de-quoted string (e.g., "\"bob\"" becomes "bob")
//    tIdentifier  // An un-quoted string
//    '{'
//    ';'
//    '}'

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	eof       = 0x7fffffff // end of file, also an invalid rune
	maxErrors = 8
	tooMany   = "too many errors...\n"

	openBrace  = '{'
	closeBrace = '}'
)

// stateFn represents a state in the lexer as a function, returning the next
// state the lexer should move to.
type stateFn func(*lexer) stateFn

// A lexer holds the internal state of the lexer.
type lexer struct {
	errout io.Writer // destination for errors, defaults to os.Stderr
	errcnt int       // number of errors encountered

	file  string // name of file we are processing
	input string // contents of the file
	start int    // start position in input of unconsumed data.
	pos   int    // current position in the input.
	line  int    // the current line number (1's based)
	col   int    // the current column number (0 based, add 1 before displaying)

	debug     bool        // set to true to include internal debugging
	inPattern bool        // set when parsing the argument to a pattern
	items     chan *token // channel of scanned items.
	tcol      int         // column with tabs expanded (for multi-line strings)
	scol      int         // starting col of current token
	sline     int         // starting line of current token
	state     stateFn     // current state of the lexer
	width     int         // width of last rune read from input.
}

// A code is a token code.  Single character tokens (i.e., punctuation)
// are represented by their unicode code point.
type code int

const (
	tEOF        = code(-1 - iota) // Reached end of file
	tError                        // An error
	tString                       // A dequoted string
	tIdentifier                   // A non-quoted string
)

// String returns c as a string.
func (c code) String() string {
	switch c {
	case tError:
		return "Error"
	case tString:
		return "String"
	case tIdentifier:
		return "Identifier"
	}
	if c < 0 || c > '~' {
		return fmt.Sprintf("%d", c)
	}
	return fmt.Sprintf("'%c'", c)
}

// A token represents one lexical unit read from the input.
// Line and Col are both 1's based.
type token struct {
	code code
	Text string // the actual text of the token
	File string // the source file the token is from
	Line int    // the source line number the token is from
	Col  int    // the source column number the token is from (8 space tabs)
}

// Code returns the code of t.  If t is nil, tEOF is returned.
func (t *token) Code() code {
	if t == nil {
		return tEOF
	}
	return t.code
}

// String returns the location, code, and text of t as a string.
func (t *token) String() string {
	var s []string
	if t.File != "" {
		s = append(s, t.File+":")
	}
	if t.Line != 0 {
		s = append(s, fmt.Sprintf("%d:%d:", t.Line, t.Col))
	}
	if t.Text == "" {
		s = append(s, fmt.Sprintf(" %v", t.code))
	} else {
		s = append(s, " ", t.Text)
	}
	return strings.Join(s, "")
}

// A note on writing to errout.  Errors should always be written to errout
// in a single Write call.  The test code makes this assumption for testing
// expected errors.

// newLexer imports the provided input into the lexer l at its the current
// location, returning the lexer.  If l is nil then a new lexer is returned.
// The provided path should indicate where the source originated.
func newLexer(input, path string) *lexer {
	// Force input to be newline terminated.
	if len(input) > 0 && input[len(input)-1] != '\n' {
		input += "\n"
	}
	return &lexer{
		file:   path,
		input:  input,
		line:   1, // humans start with 1
		items:  make(chan *token, 3),
		state:  lexGround,
		errout: os.Stderr,
	}
}

// NextToken returns the next token from the input, returning nil on EOF.
func (l *lexer) NextToken() *token {
	for {
		select {
		case item := <-l.items:
			return item
		default:
			if l.state == nil {
				return nil
			}
			if l.debug {
				name := runtime.FuncForPC(reflect.ValueOf(l.state).Pointer()).Name()
				name = name[strings.LastIndex(name, ".")+1:]
				name = strings.TrimPrefix(name, "lex")
				input := l.input[l.pos:]
				if len(input) > 8 {
					input = input[:8] + "..."
				}
				fmt.Fprintf(os.Stderr, "%d:%d: state %s %q\n", l.line, l.col+1, name, input)
			}
			l.state = l.state(l)
		}
	}
}

// emit emits the currently parsed token marked with code c using emitText.
func (l *lexer) emit(c code) {
	l.emitText(c, l.input[l.start:l.pos])
}

// emitText emits text as a token marked with c.
// All input up to the current cursor (pos) is consumed.
func (l *lexer) emitText(c code, text string) {
	if l.debug {
		fmt.Fprintf(os.Stderr, "%v: %q\n", c, text)
	}
	l.items <- &token{
		code: c,
		Text: text,
		File: l.file,
		Line: l.sline,
		Col:  l.scol + 1,
	}
	l.consume()
}

// consume consumes all input to the current cursor.
func (l *lexer) consume() {
	l.start = l.pos
}

// backup steps back one rune.  It can be called only immediately after a call
// of next.  Backing up over a tab will set tcol to the last position of the
// tab, not where the tab started.  This is okay as when we call next again it
// will move tcol back to where it was before backup was called.
func (l *lexer) backup() {
	l.pos -= l.width
	if l.width > 0 {
		l.col--
		l.tcol--
		if l.col < 0 {
			// We must have backuped up over a newline.
			// Don't bother to figure out the column number
			// as the next call to next will reset it to 0.
			l.line--
			l.col = 0
			l.tcol = 0
		}
	}
}

// peek returns but does not move past the next rune in the input.  backup
// is not supported over peeked characters.
func (l *lexer) peek() rune {
	rune := l.next()
	l.backup()
	return rune
}

// next returns the next rune in the input.  If next encounters the end of input
// then it will return eof.
func (l *lexer) next() (rune rune) {
	for l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	// l.width is what limits more than a single backup.
	rune, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	switch rune {
	case '\n':
		l.line++
		l.col = 0
		l.tcol = 0
	case '\t':
		l.tcol = (l.tcol + 8) & ^7
		l.col++ // should this be l.width?
	default:
		l.tcol++
		l.col++ // should this be l.width?
	}
	return rune
}

// acceptRun moves the cursor forward up to, but not including, the first rune
// not found in the valid set.  It returns true if any runes were accepted.
func (l *lexer) acceptRun(valid string) bool {
	ret := false
	for strings.IndexRune(valid, l.next()) >= 0 {
		ret = true
	}
	l.backup()
	return ret
}

// skipTo moves the cursor up to, but not including, s.
func (l *lexer) skipTo(s string) bool {
	if x := strings.Index(l.input[l.pos:], s); x >= 0 {
		l.updateCursor(x)
		return true
	}
	return false
}

// updateCursor moves the cursor forward n bytes.  updateCursor does not
// correctly handle tabs.  This is okay as it is only used by skipTo and skipTo
// is never used to skip to an initial " (which is the only time that tcol is
// necessary, as per YANG's multi-line quoted string requirement).
func (l *lexer) updateCursor(n int) {
	s := l.input[l.pos : l.pos+n]
	l.pos += n
	// we could get away without updating width at all because backup is
	// only promised to work after a call to next.
	l.width = n

	if c := strings.Count(s, "\n"); c > 0 {
		l.line += c
		l.col = 0
	}
	l.col += utf8.RuneCountInString(s[strings.LastIndex(s, "\n")+1:])
}

// Errorf writes an error on l.errout and increments the error count.
// If too many errors (8) are encountered then lexing will stop and
// eof is returned as the next token.
func (l *lexer) Errorf(f string, v ...interface{}) {
	buf := &bytes.Buffer{}

	if l.debug {
		// For internal debugging, print the file and line number
		// of the call to Errorf
		_, name, line, _ := runtime.Caller(1)

		fmt.Fprintf(buf, "%s:%d: ", name, line)
	}
	fmt.Fprintf(buf, "%s:%d:%d: ", l.file, l.line, l.col+1)
	fmt.Fprintf(buf, f, v...)
	b := buf.Bytes()
	if b[len(b)-1] != '\n' {
		buf.Write([]byte{'\n'})
	}
	l.emit(tError)
	l.adderror(buf.Bytes())
}

func (l *lexer) ErrorfAt(line, col int, f string, v ...interface{}) {
	oline, ocol := l.line, l.col
	defer func() {
		l.line, l.col = oline, ocol
	}()
	l.line, l.col = line, col
	l.Errorf(f, v...)
}

// adderror writes out the error string err and increases the error count.
// If more than maxErrors are encountered, a "too many errors" message is
// displayed and processing stops (by clearing the input).
func (l *lexer) adderror(err []byte) {
	if l.errcnt >= maxErrors {
		l.pos = 0
		l.start = 0
		l.input = ""
		l.errout.Write([]byte(tooMany))
		return
	}
	l.errout.Write(err)
	l.errcnt++
}

// Below are all the states

// lexGround is the state when the lexer is not in the middle of a token.  The
// ground state is left once the start of a token is found.  Pure comment lines
// leave the lexer in the ground state.
func lexGround(l *lexer) stateFn {
	l.acceptRun(" \t\r\n") // Skip leading spaces
	l.consume()
	l.sline = l.line
	l.scol = l.col

	switch c := l.peek(); c {
	case eof:
		return nil
	case ';', openBrace, closeBrace:
		l.next()
		l.emit(code(c))
		return lexGround
	case '\'':
		l.next()
		l.consume() // Toss the leading '
		l.skipTo("'")
		l.emit(tString)
		l.next() // Either EOF or the matching '
		return lexGround
	case '"':
		l.next()
		return lexQString
	case '/':
		l.next()
		switch l.peek() {
		case '/':
			// Start of a // comment
			l.skipTo("\n")
			return lexGround
		case '*':
			// Start of a /* comment
			l.skipTo("*/")
			// Now actually skip the */
			l.next()
			l.next()
			return lexGround
		}
		fallthrough
	default:
		return lexIdentifier
	}
}

// From the YANG standard:
//
//   If the double-quoted string contains a line break followed by space
//   or tab characters that are used to indent the text according to the
//   layout in the YANG file, this leading whitespace is stripped from the
//   string, up to and including the column of the double quote character,
//   or to the first non-whitespace character, whichever occurs first.  In
//   this process, a tab character is treated as 8 space characters.
//
//   If the double-quoted string contains space or tab characters before a
//   line break, this trailing whitespace is stripped from the string.

// lexQString handles double quoted strings, see the above text on how they
// work.  The leading " has already been parsed.
func lexQString(l *lexer) stateFn {
	indent := l.tcol // the column our text starts on
	over := true     // set to false when we are not past the indent

	// Keep track of where the starting quote was
	line, col := l.line, l.col-1

	var text []byte
	for {
		// l.next can return non-8bit unicode code points.
		// c cannot be treated as only a single byte.
		switch c := l.next(); c {
		case eof:
			l.ErrorfAt(line, col, `missing closing "`)
			return nil
		case '"':
			l.emitText(tString, string(text))

			return lexGround
		case '\n':
		Loop:
			// Trim trailing white space from the line.
			for i := len(text); i > 0; {
				i--
				switch text[i] {
				case ' ', '\t':
					text = text[:i]
				default:
					break Loop
				}
			}
			text = append(text, []byte(string(c))...)
			over = false
		case ' ', '\t':
			// Ignore leading white space up to our indent.
			if !over && l.tcol <= indent {
				break
			}
			over = true
			text = append(text, []byte(string(c))...)
		case '\\':
			switch c = l.next(); c {
			case eof:
				l.ErrorfAt(line, col, `missing closing "`)
			case 'n':
				c = '\n'
			case 't':
				c = '\t'
			case '"':
			case '\\':
			default:
				// Strings are use both in descriptions and
				// in patterns.  In strings only \n, \t, \"
				// and \\ are defined.  In patterns the \
				// can either mean to escape the character
				// (e..g., \{) or to be part of of a special
				// sequence such as \S.
				if !l.inPattern {
					l.ErrorfAt(line, col, `invalid escape sequence: \`+string(c))
				}
				text = append(text, '\\')
			}
			fallthrough
		default:
			over = true
			text = append(text, []byte(string(c))...)
		}
	}
}

// lexIdentifier reads one identifier/number/un-quoted-string/...
func lexIdentifier(l *lexer) stateFn {
	for {
		switch c := l.peek(); c {
		case ' ', '\r', '\n', '\t', ';', '"', openBrace, closeBrace, eof:
			l.emit(tIdentifier)
			return lexGround
		default:
			l.next()
		}
	}
}
