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

// Program yang parses YANG files, displays errors, and possibly writes
// something related to the input on output.
//
// Usage: yang [--path PATH] [--format FORMAT] [MODULE] [FILE ...]
//
// If MODULE is specified (an argument that does not end in .yang), it is taken
// as the name of the module to display.  Any FILEs specified are read, and the
// tree for MODULE is displayed.  If MODULE was not defined in FILEs (or no
// files were specified), then the file MODULES.yang is read as well.  An error
// is displayed if no definition for MODULE was found.
//
// If MODULE is missing, then all base modules read from the FILEs are
// displayed.  If there are no arguments then standard input is parsed.
//
// If PATH is specified, it is considered a comma separated list of paths
// to append to the search directory.
//
// FORMAT, which defaults to "tree", specifes the format of output to produce:
//
//   tree   All modules in tree form
//   proto  All directory nodes as proto structures
//   types  All type definitions
//
// THIS PROGRAM IS STILL JUST A DEVELOPMENT TOOL.
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/pborman/getopt"
	"github.com/openconfig/goyang/pkg/indent"
	"github.com/openconfig/goyang/pkg/yang"
)

// Types keeps track of all the YangTypes defined.
type Types map[*yang.YangType]struct{}

// AddEntry adds all types defined in e and its decendents to t.
func (t Types) AddEntry(e *yang.Entry) {
	if e == nil {
		return
	}
	if e.Type != nil {
		t[e.Type.Root] = struct{}{}
	}
	for _, d := range e.Dir {
		t.AddEntry(d)
	}
	t.AddEntry(e.Choice)
	t.AddEntry(e.Case)
}

// exitIfError writes errs to standard error and exits with an exit status of 1.
// If errs is empty then exitIfError does nothing and simply returns.
func exitIfError(errs []error) {
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func main() {
	format := "tree"
	getopt.CommandLine.ListVarLong(&yang.Path, "path", 0, "comma separated list of directories to add to PATH")
	getopt.CommandLine.StringVarLong(&format, "format", 0, "format to display: tree, proto, types")

	getopt.Parse()
	files := getopt.Args()

	if len(files) > 0 && !strings.HasSuffix(files[0], ".yang") {
		e, errs := yang.GetModule(files[0], files[1:]...)
		exitIfError(errs)
		Write(os.Stdout, e)
		return
	}

	// Okay, either there are no arguments and we read stdin, or there
	// is one or more file names listed.  Read them in and display them.

	ms := yang.NewModules()

	if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err == nil {
			err = ms.Parse(string(data), "<STDIN>")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	for _, name := range files {
		if err := ms.Read(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
	}

	// Keep track of the top level modules we read in.
	// Those are the only modules we want to print below.
	mods := map[string]*yang.Module{}
	var names []string

	for _, m := range ms.Modules {
		if mods[m.Name] == nil {
			mods[m.Name] = m
			names = append(names, m.Name)
		}
	}
	sort.Strings(names)

	// Print any errors found in the tree.  This will return false if
	// there were no errors.
	exitIfError(ms.Process())

	switch format {
	case "tree":
		for _, n := range names {
			Write(os.Stdout, yang.ToEntry(mods[n]))
		}
	case "proto":
		for _, n := range names {
			for _, e := range flatten(yang.ToEntry(mods[n])) {
				FormatNode(os.Stdout, e)
			}
		}
	case "types":
		types := Types{}
		for _, n := range names {
			types.AddEntry(yang.ToEntry(mods[n]))
		}

		for t := range types {
			YTPrint(os.Stdout, t)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s\n", format)
		os.Exit(1)
	}
}

// fixedNames maps a fixed name back to its origial name.
var fixedNames = map[string]string{}

func fixName(s string) string {
	cc := yang.CamelCase(s)
	if cc != s {
		if o := fixedNames[cc]; o != "" && o != s {
			fmt.Printf("Collision on %s and %s\n", o, s)
		}
		fixedNames[cc] = s
	}
	return cc
}

func getTypeName(e *yang.Entry) string {
	if e == nil || e.Type == nil {
		return ""
	}
	// Return our root's type name.
	// This is should be the builtin type-name
	// for this entry.
	return e.Type.Root.Name
}

// Write writes e, formatted, and all of its children, to w.
func Write(w io.Writer, e *yang.Entry) {
	if e.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}
	if e.ReadOnly() {
		fmt.Fprintf(w, "RO: ")
	} else {
		fmt.Fprintf(w, "rw: ")
	}
	if e.Type != nil {
		fmt.Fprintf(w, "%s ", getTypeName(e))
	}
	switch {
	case e.Dir == nil && e.IsList:
		fmt.Fprintf(w, "[]%s\n", e.Name)
		return
	case e.Dir == nil:
		fmt.Fprintf(w, "%s\n", e.Name)
		return
	case e.IsList:
		fmt.Fprintf(w, "[%s]%s {\n", e.Key, e.Name) //}
	default:
		fmt.Fprintf(w, "%s {\n", e.Name) //}
	}
	var names []string
	for k := range e.Dir {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		Write(indent.NewWriter(w, "  "), e.Dir[k])
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}

// kind2proto maps base yang types to protocol buffer types.
// TODO(borman): do TODO types.
var kind2proto = map[yang.TypeKind]string{
	yang.Yint8:   "int32",  // int in range [-128, 127]
	yang.Yint16:  "int32",  // int in range [-32768, 32767]
	yang.Yint32:  "int32",  // int in range [-2147483648, 2147483647]
	yang.Yint64:  "int64",  // int in range [-9223372036854775808, 9223372036854775807]
	yang.Yuint8:  "uint32", // int in range [0, 255]
	yang.Yuint16: "uint32", // int in range [0, 65535]
	yang.Yuint32: "uint32", // int in range [0, 4294967295]
	yang.Yuint64: "uint64", // int in range [0, 18446744073709551615]

	yang.Ybinary:             "bytes",          // arbitrary data
	yang.Ybits:               "TODO-bits",      // set of bits or flags
	yang.Ybool:               "bool",           // true or false
	yang.Ydecimal64:          "TODO-decimal64", // signed decimal number
	yang.Yenum:               "TODO-enum",      // enumerated strings
	yang.Yidentityref:        "string",         // reference to abstrace identity
	yang.YinstanceIdentifier: "TODO-ii",        // reference of a data tree node
	yang.Yleafref:            "string",         // reference to a leaf instance
	yang.Ystring:             "string",         // human readable string
	yang.Yunion:              "TODO-union",     // choice of types
}

// FormatNode writes e, formatted almost like a protobuf message, to w.
func FormatNode(w io.Writer, e *yang.Entry) {
	fmt.Fprintln(w)
	if e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}
	fmt.Fprintf(w, "message %s {\n", fixName(e.Name))

	var names []string
	for k := range e.Dir {
		names = append(names, k)
	}
	sort.Strings(names)
	for x, k := range names {
		se := e.Dir[k]
		if se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		if se.IsList {
			fmt.Fprint(w, "  repeated ")
		} else {
			fmt.Fprint(w, "  optional ")
		}
		if len(se.Dir) == 0 && se.Type != nil {
			// TODO(borman): this is probably an empty container.
			kind := "UNKNOWN TYPE"
			if se.Type != nil {
				kind = kind2proto[se.Type.Kind]
			}
			fmt.Fprintf(w, "%s %s = %d; // %s\n", kind, fixName(k), x+1, yang.Source(se.Node))
			continue
		}
		fmt.Fprintf(w, "%s %s = %d; // %s\n", fixName(se.Name), fixName(k), x+1, yang.Source(se.Node))
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}

// flatten returns a slice of all directory entries in e and e's decendents.
func flatten(e *yang.Entry) []*yang.Entry {
	if e == nil || len(e.Dir) == 0 {
		return nil
	}
	f := []*yang.Entry{e}
	var names []string
	for n := range e.Dir {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f = append(f, flatten(e.Dir[n])...)
	}
	f = append(f, flatten(e.Choice)...)
	f = append(f, flatten(e.Case)...)
	return f
}

// YTPrint prints type t in a moderately human readable format to w.
func YTPrint(w io.Writer, t *yang.YangType) {
	if t.Base != nil {
		fmt.Fprintf(w, "%s: ", yang.Source(t.Base))
	}
	fmt.Fprintf(w, "%s", t.Root.Name)
	if t.Kind.String() != t.Root.Name {
		fmt.Fprintf(w, "(%s)", t.Kind)
	}
	if t.Units != "" {
		fmt.Fprintf(w, " units=%s", t.Units)
	}
	if t.Default != "" {
		fmt.Fprintf(w, " default=%q", t.Default)
	}
	if t.FractionDigits != 0 {
		fmt.Fprintf(w, " fraction-digits=%d", t.FractionDigits)
	}
	if len(t.Length) > 0 {
		fmt.Fprintf(w, " length=%s", t.Length)
	}
	if t.Kind == yang.YinstanceIdentifier && !t.OptionalInstance {
		fmt.Fprintf(w, " required")
	}
	if t.Kind == yang.Yleafref && t.Path != "" {
		fmt.Fprintf(w, " path=%q", t.Path)
	}
	if len(t.Pattern) > 0 {
		fmt.Fprintf(w, " pattern=%s", strings.Join(t.Pattern, "|"))
	}
	if len(t.Type) > 0 {
		fmt.Fprintf(w, " union...")
	}

	b := yang.BaseTypedefs[t.Kind.String()].YangType
	if len(t.Range) > 0 && !t.Range.Equal(b.Range) {
		fmt.Fprintf(w, " range=%s", t.Range)
	}
	fmt.Fprintf(w, ";\n")
}
