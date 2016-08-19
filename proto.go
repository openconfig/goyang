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

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openconfig/goyang/pkg/indent"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/pborman/getopt"
)

const (
	protoVersion  = "1"
	tagPrefix     = "// goyang-tag "
	versionPrefix = "// goyang-version "
)

var (
	proto2          bool
	protoDir        string
	protoNoComments bool
	protoNotNested  bool
	protoPreserve   string
	protoWithSource bool
)

func init() {
	flags := getopt.New()
	register(&formatter{
		name:  "proto",
		f:     doProto,
		help:  "display tree in a proto like format",
		flags: flags,
	})
	flags.BoolVarLong(&proto2, "proto2", 0, "produce proto2 protobufs")
	flags.StringVarLong(&protoDir, "proto_dir", 0, "write a .proto file for each module in DIR", "DIR")
	flags.BoolVarLong(&protoNoComments, "proto_no_comments", 0, "do not include comments in output")
	flags.BoolVarLong(&protoNotNested, "proto_not_nested", 0, "do not produce nested protobufs")
	flags.StringVarLong(&protoPreserve, "proto_preserve", 0, "preserve existing .proto files as filename.SUFFIX", "SUFFIX")
	flags.BoolVarLong(&protoWithSource, "proto_with_source", 0, "add source location comments")
}

// A protofile collects the produced proto along with meta information.
type protofile struct {
	buf        bytes.Buffer
	fixedNames map[string]string // maps a fixed name back to its origial name.
	errs       []error
	messages   map[string]*messageInfo
}

// A messageInfo contains tag information about fields in a message.
type messageInfo struct {
	last   int
	fields map[string]int
}

func doProto(w io.Writer, entries []*yang.Entry) {
	failed := false
	if protoPreserve != "" && protoPreserve[0] != '.' {
		protoPreserve = "." + protoPreserve
	}
	for _, e := range entries {
		if len(e.Dir) == 0 {
			// skip modules that have nothing in them
			continue
		}

		pf := &protofile{
			fixedNames: map[string]string{},
			messages:   map[string]*messageInfo{},
		}

		var out string
		// optionally read in tags from old proto
		if protoDir != "" {
			out = filepath.Join(protoDir, e.Name+".proto")
			if fd, err := os.Open(out); err == nil {
				err = pf.importTags(fd)
				fd.Close()
				if err != nil {
					failed = true
					fmt.Fprintln(os.Stderr, err)
					continue
				}
			}
		}

		pf.printHeader(&pf.buf, e)
		for _, e := range flatten(e) {
			pf.printService(&pf.buf, e)
		}
		for _, child := range children(e) {
			if protoNotNested {
				for _, e := range flatten(child) {
					pf.printFlatNode(&pf.buf, e)
				}
			} else {
				pf.printNextedNode(&pf.buf, child)
			}
		}
		pf.dumpMessageInfo()
		if len(pf.errs) != 0 {
			for _, err := range pf.errs {
				failed = true
				fmt.Fprintf(os.Stderr, "%s: %v\n", e.Name, err)
			}
			continue
		}
		var closeme io.Closer
		if out == "" {
			out = "stdout"
		} else {
			var old string
			if protoPreserve != "" {
				old = out + protoPreserve
				if _, err := os.Stat(out); err == nil {
					if err := os.Rename(out, old); err != nil {
						failed = true
						fmt.Fprintf(os.Stderr, "%s: %v\n", e.Name, err)
						continue
					}
				}
			}
			fd, err := os.Create(out)
			if err != nil {
				failed = true
				fmt.Fprintln(os.Stderr, err)
			}
			w = fd
			closeme = fd
		}
		if _, err := io.Copy(w, &pf.buf); err != nil {
			failed = true
			fmt.Fprintf(os.Stderr, "%s: %v\n", out, err)
		}
		if closeme != nil {
			if err := closeme.Close(); err != nil {
				failed = true
				fmt.Fprintf(os.Stderr, "%s: %v\n", out, err)
			}
		}
	}
	if failed {
		os.Exit(1)
	}
}

// Children returns all the children nodes of e that are not RPC nodes.
func children(e *yang.Entry) []*yang.Entry {
	var names []string
	for k, se := range e.Dir {
		if se.RPC == nil {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	children := make([]*yang.Entry, len(names))
	for x, n := range names {
		children[x] = e.Dir[n]
	}
	return children
}

func (pf *protofile) dumpMessageInfo() {
	w := &pf.buf
	fmt.Fprint(w, `
// Do not delete the lines below.
// They are used to preserve tag information by goyang.
`)
	names := make([]string, len(pf.messages))
	x := 0
	for name := range pf.messages {
		names[x] = name
		x++
	}
	sort.Strings(names)
	for _, name := range names {
		mi := pf.messages[name]
		mi.dump(name, &pf.buf)
	}
}

func (mi *messageInfo) dump(mname string, w io.Writer) {
	names := make([]string, len(mi.fields))
	x := 0
	for name := range mi.fields {
		names[x] = name
		x++
	}
	sort.Strings(names)
	for _, name := range names {
		tag := mi.fields[name]
		fmt.Fprintf(w, "%s%s %s %d\n", tagPrefix, mname, name, tag)
	}
}

func (pf *protofile) importTags(r io.Reader) error {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, versionPrefix) {
			version := strings.TrimSpace(line[len(versionPrefix):])
			if version != protoVersion {
				return fmt.Errorf("unsupported goyang proto version: %s", version)
			}
			continue
		}
		if !strings.HasPrefix(line, tagPrefix) {
			continue
		}
		line = line[len(tagPrefix):]
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return fmt.Errorf("invalid goyang-tag: %s", line)
		}
		tag, err := strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("invalid goyang-tag: %s", line)
		}
		mi := pf.messages[fields[0]]
		if mi == nil {
			mi = &messageInfo{
				fields: map[string]int{},
			}
			pf.messages[fields[0]] = mi
		}
		mi.fields[fields[1]] = tag
		if mi.last < tag {
			mi.last = tag
		}
	}
	return s.Err()
}

func (m *messageInfo) tag(name, kind string, isList bool) int {
	key := name + "/" + kind
	if isList {
		key = key + "[]"
	}
	if i := m.fields[key]; i != 0 {
		return i
	}
	m.last++
	m.fields[key] = m.last
	return m.last
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
	yang.Yunion:              "inline",         // handled inline
}

func isStream(e *yang.Entry) bool {
	for _, ext := range e.Exts {
		if ext.Kind() == "grpc:stream" {
			return true
		}
	}
	return false
}

func (pf *protofile) printHeader(w io.Writer, e *yang.Entry) {
	syntax := "proto3"
	if proto2 {
		syntax = "proto2"
	}
	fmt.Fprintf(w, `syntax = %q;
// Automatically generated by goyang https://github.com/openconfig/goyang
// compiled %s`, syntax, time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(w, `
// do not delete the next line
%s%s`, versionPrefix, protoVersion)
	fmt.Fprintf(w, `
// module %q
`, e.Name)

	if v := e.Extra["revision"]; len(v) > 0 {
		for _, rev := range v[0].([]*yang.Revision) {
			fmt.Fprintf(w, "// revision %q\n", rev.Name)
		}
	}
	if v := e.Extra["namespace"]; len(v) > 0 {
		fmt.Fprintf(w, "// namespace %q\n", v[0].(*yang.Value).Name)
	}

	fmt.Fprintln(w)
	if !protoNoComments && e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}
	fmt.Fprintf(w, "package %s;\n\n", pf.fieldName(e.Name))
}

// printFlatNode writes e, formatted almost like a protobuf message, to w.
func (pf *protofile) printService(w io.Writer, e *yang.Entry) {
	var names []string

	for k, se := range e.Dir {
		if se.RPC != nil {
			names = append(names, k)
		}
	}
	needEmpty := false
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	fmt.Fprintf(w, "service %s {\n", pf.fixName(e.Name))
	for _, k := range names {
		rpc := e.Dir[k].RPC
		k = pf.fixName(k)
		iName := "Empty"
		oName := "Empty"
		if rpc.Input != nil {
			iName = k + "Request"
			rpc.Input.Name = iName
			if isStream(rpc.Input) {
				iName = "stream " + iName
			}
		}
		if rpc.Output != nil {
			oName = k + "Response"
			rpc.Output.Name = oName
			if isStream(rpc.Output) {
				oName = "stream " + oName
			}
		}
		needEmpty = needEmpty || rpc.Input == nil || rpc.Output == nil
		fmt.Fprintf(w, "  rpc %s (%s) returns (%s);\n", k, iName, oName)
	}
	fmt.Fprintln(w, "}")
	for _, k := range names {
		rpc := e.Dir[k].RPC
		if rpc.Input != nil {
			pf.printFlatNode(w, rpc.Input)
		}
		if rpc.Output != nil {
			pf.printFlatNode(w, rpc.Output)
		}
	}

	if needEmpty {
		fmt.Fprintln(w, "\nmessage Empty { }")
	}
}

// printFlatNode writes e, formatted almost like a protobuf message, to w.
func (pf *protofile) printFlatNode(w io.Writer, e *yang.Entry) {
	nodes := children(e)
	if len(nodes) == 0 {
		return
	}

	fmt.Fprintln(w)
	if !protoNoComments && e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}
	messageName := pf.fullName(e.Name, e)
	mi := pf.messages[messageName]
	if mi == nil {
		mi = &messageInfo{
			fields: map[string]int{},
		}
		pf.messages[messageName] = mi
	}

	fmt.Fprintf(w, "message %s {\n", messageName) // matching brace }
	for _, se := range nodes {
		k := se.Name
		if !protoNoComments && se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		fmt.Fprint(w, "  ")
		if se.ListAttr != nil {
			fmt.Fprint(w, "repeated ")
		} else if proto2 {
			fmt.Fprint(w, "optional ")
		}
		name := pf.fixName(k)
		if len(se.Dir) > 0 || se.Type == nil {
			kind := pf.fullName(se.Name, se)
			fmt.Fprintf(w, "%s %s = %d;", kind, name, mi.tag(name, kind, se.ListAttr != nil))
		} else {
			kind := "UNKNOWN TYPE" // if se.Type is nil, this must be an empty container
			if se.Type != nil {
				kind = kind2proto[se.Type.Kind]
			}
			printed := false
			if len(se.Type.Type) > 0 {
				seen := map[string]bool{}
				var types []string
				for _, t := range se.Type.Type {
					kind = kind2proto[t.Kind]
					if seen[kind] {
						continue
					}
					types = append(types, kind)
					seen[kind] = true
				}
				if len(types) > 1 {
					fmt.Fprintf(w, "oneof %s {\n", name) // matching brace }
					for _, kind := range types {
						fmt.Fprintf(w, "    %s as_%s = %d;\n", kind, kind, mi.tag(name, kind, false))
					}
					// { to match the brace below to keep brace matching working
					fmt.Fprintf(w, "  }")
					printed = true
				}
				// kind is now the value of the only base type we found in the union.
			}
			if !printed {
				fmt.Fprintf(w, "%s %s = %d;", kind, name, mi.tag(name, kind, se.ListAttr != nil))
			}
		}
		if protoWithSource {
			fmt.Fprintf(w, " // %s", yang.Source(se.Node))
		}
		fmt.Fprintln(w)
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}

// printNextedNode writes e, formatted almost like a protobuf message, to w.
func (pf *protofile) printNextedNode(w io.Writer, e *yang.Entry) {
	nodes := children(e)
	if len(nodes) == 0 {
		return
	}

	if !protoNoComments && e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}

	messageName := pf.fullName(e.Name, e)
	mi := pf.messages[messageName]
	if mi == nil {
		mi = &messageInfo{
			fields: map[string]int{},
		}
		pf.messages[messageName] = mi
	}

	fmt.Fprintf(w, "message %s {\n", pf.fixName(e.Name)) // matching brace }

	for _, se := range nodes {
		k := se.Name
		if !protoNoComments && se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		if len(se.Dir) > 0 || se.Type == nil {
			pf.printNextedNode(indent.NewWriter(w, "  "), se)
		}
		fmt.Fprint(w, "  ")
		if se.ListAttr != nil {
			fmt.Fprint(w, "repeated ")
		} else if proto2 {
			fmt.Fprint(w, "optional ")
		}
		name := pf.fieldName(k)
		if len(se.Dir) > 0 || se.Type == nil {
			kind := pf.fixName(se.Name)
			fmt.Fprintf(w, "%s %s = %d;", kind, name, mi.tag(name, kind, se.ListAttr != nil))
		} else {
			kind := kind2proto[se.Type.Kind]
			printed := false
			if len(se.Type.Type) > 0 {
				seen := map[string]bool{}
				var types []string
				for _, t := range se.Type.Type {
					kind = kind2proto[t.Kind]
					if seen[kind] {
						continue
					}
					types = append(types, kind)
					seen[kind] = true
				}
				if len(types) > 1 {
					fmt.Fprintf(w, "oneof %s {\n", name) // matching brace }
					for _, kind := range types {
						fmt.Fprintf(w, "    %s as_%s = %d;\n", kind, kind, mi.tag(name, kind, false))
					}
					// { to match the brace below to keep brace matching working
					fmt.Fprintf(w, "  }")
					printed = true
				}
				// kind is now the value of the only base type we found in the union.
			}
			if !printed {
				fmt.Fprintf(w, "%s %s = %d;", kind, name, mi.tag(name, kind, se.ListAttr != nil))
			}
		}
		if protoWithSource {
			fmt.Fprintf(w, " // %s", yang.Source(se.Node))
		}
		fmt.Fprintln(w)
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}

func (pf *protofile) fullName(n string, e *yang.Entry) string {
	parts := []string{pf.fixName(n)}
	for p := e.Parent; p != nil; p = p.Parent {
		parts = append(parts, pf.fixName(p.Name))
	}
	for i := 0; i < len(parts)/2; i++ {
		parts[i], parts[len(parts)-i-1] = parts[len(parts)-i-1], parts[i]
	}
	return strings.Join(parts, "_")
}

// fieldName simply changes -'s to _'s.
func (pf *protofile) fieldName(s string) string {
	fn := strings.Replace(s, "-", "_", -1)
	if fn != s {
		if o := pf.fixedNames[fn]; o != "" && o != s {
			pf.errs = append(pf.errs, fmt.Errorf("collision on %s and %s\n", o, s))
		}
		pf.fixedNames[fn] = s
	}
	return fn
}

// fixName returns s in camel case
func (pf *protofile) fixName(s string) string {
	cc := yang.CamelCase(s)
	if cc != s {
		if o := pf.fixedNames[cc]; o != "" && o != s {
			pf.errs = append(pf.errs, fmt.Errorf("collision on %s and %s\n", o, s))
		}
		pf.fixedNames[cc] = s
	}
	return cc
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
	return f
}
