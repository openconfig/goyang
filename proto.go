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
	protoFlat       bool
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
	flags.BoolVarLong(&protoFlat, "proto_flat", 0, "do not produce nested protobufs")
	flags.StringVarLong(&protoPreserve, "proto_save", 0, "preserve existing .proto files as filename.SUFFIX", "SUFFIX")
	flags.BoolVarLong(&protoWithSource, "proto_with_source", 0, "add source location comments")
}

// A protofile collects the produced proto along with meta information.
type protofile struct {
	buf          bytes.Buffer
	fixedNames   map[string]string // maps a fixed name back to its origial name.
	errs         []error
	messages     map[string]*messageInfo
	hasDecimal64 bool
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
			if protoFlat {
				for _, e := range flatten(child) {
					fmt.Fprintln(&pf.buf)
					pf.printNode(&pf.buf, e, false)
				}
			} else {
				fmt.Fprintln(&pf.buf)
				pf.printNode(&pf.buf, child, true)
			}
		}
		if pf.hasDecimal64 {
			prefix := " "
			if proto2 {
				prefix = "  optional"
			}
			fmt.Fprintf(&pf.buf, `
// A Decimal64 is the YANG decimal64 type.
message Decimal64 {
%s int64  value = 1;            // integeral value
%s uint32 fraction_digits = 2;  // decimal point position [1..18]
}
`, prefix, prefix)
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
// Do not delete the lines below, they preserve tag information for goyang.
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

	yang.Ybinary:             "bytes",        // arbitrary data
	yang.Ybits:               "INLINE-bits",  // set of bits or flags
	yang.Ybool:               "bool",         // true or false
	yang.Ydecimal64:          "INLINE-d64",   // signed decimal number
	yang.Yempty:              "bool",         // value is its presence
	yang.Yenum:               "INLINE-enum",  // enumerated strings
	yang.Yidentityref:        "string",       // reference to abstract identity
	yang.YinstanceIdentifier: "string",       // reference of a data tree node
	yang.Yleafref:            "string",       // reference to a leaf instance
	yang.Ystring:             "string",       // human readable string
	yang.Yunion:              "INLINE-union", // handled inline
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
	fmt.Fprintf(w, "package %s;\n", pf.fieldName(e.Name))
}

// printService writes e, formatted almost like a protobuf message, to w.
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
			pf.printNode(w, rpc.Input, false)
		}
		if rpc.Output != nil {
			pf.printNode(w, rpc.Output, false)
		}
	}

	if needEmpty {
		fmt.Fprintln(w, "\nmessage Empty { }")
	}
}

// printNode writes e, formatted almost like a protobuf message, to w.
func (pf *protofile) printNode(w io.Writer, e *yang.Entry, nest bool) {
	nodes := children(e)

	if !protoNoComments && e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}

	messageName := pf.fullName(e)
	mi := pf.messages[messageName]
	if mi == nil {
		mi = &messageInfo{
			fields: map[string]int{},
		}
		pf.messages[messageName] = mi
	}

	fmt.Fprintf(w, "message %s {", pf.messageName(e)) // matching brace }
	if protoWithSource {
		fmt.Fprintf(w, " // %s", yang.Source(e.Node))
	}
	fmt.Fprintln(w)

	for i, se := range nodes {
		k := se.Name
		if !protoNoComments && se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		if nest && (len(se.Dir) > 0 || se.Type == nil) {
			pf.printNode(indent.NewWriter(w, "  "), se, true)
		}
		prefix := "  "
		if se.ListAttr != nil {
			prefix = "  repeated "
		} else if proto2 {
			prefix = "  optional "
		}
		name := pf.fieldName(k)
		printed := false
		var kind string
		if len(se.Dir) > 0 || se.Type == nil {
			kind = pf.messageName(se)
		} else if se.Type.Kind == yang.Ybits {
			values := dedup(se.Type.Bit.Values())
			asComment := false
			switch {
			case len(values) > 0 && values[len(values)-1] > 63:
				if i != 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "  // *WARNING* bitfield %s has more than 64 positions\n", name)
				kind = "uint64"
				asComment = true
			case len(values) > 0 && values[len(values)-1] > 31:
				if i != 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "  // bitfield %s to large for enum\n", name)
				kind = "uint64"
				asComment = true
			default:
				kind = pf.fixName(se.Name)
			}
			if !asComment {
				fmt.Fprintf(w, "  enum %s {\n", kind)
				fmt.Fprintf(w, "    %s_FIELD_NOT_SET = 0;\n", kind)
			} else {
				fmt.Fprintf(w, "  // Values:\n")
			}
			names := map[int64][]string{}
			for n, v := range se.Type.Bit.NameMap() {
				names[v] = append(names[v], n)
			}
			for _, v := range values {
				ns := names[v]
				sort.Strings(ns)
				if asComment {
					for _, n := range ns {
						fmt.Fprintf(w, "  //   %s = 1 << %d\n", n, v)
					}
				} else {
					n := strings.ToUpper(pf.fieldName(ns[0]))
					fmt.Fprintf(w, "    %s_%s = %d;\n", kind, n, 1<<uint(v))
					for _, n := range ns[1:] {
						n = strings.ToUpper(pf.fieldName(n))
						fmt.Fprintf(w, "    // %s = %d; (DUPLICATE VALUE)\n", n, 1<<uint(v))
					}
				}
			}
			if !asComment {
				fmt.Fprintf(w, "  };\n")
			}
		} else if se.Type.Kind == yang.Ydecimal64 {
			kind = "Decimal64"
			pf.hasDecimal64 = true
		} else if se.Type.Kind == yang.Yenum {
			kind = pf.fixName(se.Name)
			fmt.Fprintf(w, "  enum %s {", kind)
			if protoWithSource {
				fmt.Fprintf(w, " // %s", yang.Source(se.Node))
			}
			fmt.Fprintln(w)

			for i, n := range se.Type.Enum.Names() {
				fmt.Fprintf(w, "    %s_%s = %d;\n", kind, strings.ToUpper(pf.fieldName(n)), i)
			}
			fmt.Fprintf(w, "  };\n")
		} else if se.Type.Kind == yang.Yunion {
			types := pf.unionTypes(se.Type, map[string]bool{})
			switch len(types) {
			case 0:
				fmt.Fprintf(w, "    // *WARNING* union %s has no types\n", se.Name)
				printed = true
			case 1:
				kind = types[0]
			default:
				iw := w
				kind = pf.fixName(se.Name)
				if se.ListAttr != nil {
					fmt.Fprintf(w, "  message %s {\n", kind)
					iw = indent.NewWriter(w, "  ")
				}
				fmt.Fprintf(iw, "  oneof %s {", kind) // matching brace }
				if protoWithSource {
					fmt.Fprintf(w, " // %s", yang.Source(se.Node))
				}
				fmt.Fprintln(w)
				for _, tkind := range types {
					fmt.Fprintf(iw, "    %s %s_%s = %d;\n", tkind, kind, tkind, mi.tag(name, tkind, false))
				}
				// { to match the brace below to keep brace matching working
				fmt.Fprintf(iw, "  }\n")
				if se.ListAttr != nil {
					fmt.Fprintf(w, "  }\n")
				} else {
					printed = true
				}
			}
		} else {
			kind = kind2proto[se.Type.Kind]
		}
		if !printed {
			fmt.Fprintf(w, "%s%s %s = %d;", prefix, kind, name, mi.tag(name, kind, se.ListAttr != nil))
			if protoWithSource {
				fmt.Fprintf(w, " // %s", yang.Source(se.Node))
			}
			fmt.Fprintln(w)
		}
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}

// unionTypes returns a slice of all types in the union (and sub unions).
func (pf *protofile) unionTypes(ut *yang.YangType, seen map[string]bool) []string {
	var types []string
	for _, t := range ut.Type {
		k := t.Kind
		switch k {
		case yang.Yenum:
			// TODO(borman): this is not correct for enums that fit in 32 bits
			k = yang.Yuint64
		case yang.Ybits:
			k = yang.Yuint64
		case yang.Yunion:
			for _, st := range t.Type {
				types = append(types, pf.unionTypes(st, seen)...)
			}
			continue
		}
		kn := kind2proto[k]
		if k == yang.Ydecimal64 {
			kn = "Decimal64"
			pf.hasDecimal64 = true
		}
		if seen[kn] {
			continue
		}
		seen[kn] = true
		types = append(types, kn)
	}
	sort.Strings(types)
	return types
}

// messageName returns the name for the message defined by e.
func (pf *protofile) messageName(e *yang.Entry) string {
	if protoFlat {
		return pf.fullName(e)
	}
	return pf.fixName(e.Name)
}

// isPlural returns true if p is the plural of s.
func isPlural(s, p string) bool {
	if s == "" || p == "" {
		return false
	}
	n := len(p)
	return s == p[:n-1] && p[n-1] == 's'
}

// fullName always returns the full pathname of entry e.
func (pf *protofile) fullName(e *yang.Entry) string {
	parts := []string{pf.fixName(e.Name)}
	for p := e.Parent; p != nil && p.Parent != nil; p = p.Parent {
		parts = append(parts, pf.fixName(p.Name))
		// Don't output Foos_Foo_, just output Foo_
		if len(p.Parent.Dir) == 1 && isPlural(p.Name, p.Parent.Name) {
			p = p.Parent
		}
	}
	for i := 0; i < len(parts)/2; i++ {
		parts[i], parts[len(parts)-i-1] = parts[len(parts)-i-1], parts[i]
	}
	return strings.Join(parts, "_")
}

// fieldName simply changes -'s to _'s.
func (pf *protofile) fieldName(s string) string {
	if s == "" {
		return ""
	}
	fn := strings.Replace(s, "-", "_", -1)
	switch {
	case fn[0] >= 'a' && fn[0] <= 'z':
	case fn[0] >= 'A' && fn[0] <= 'Z':
	default:
		fn = "X_" + fn
	}
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

// flatten returns a slice of all directory entries in e and e's descendants.
func flatten(e *yang.Entry) []*yang.Entry {
	if e == nil || (len(e.Dir) == 0 && e.Type != nil) {
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

// dedump returns the sorted slice a with duplicate values removed.
func dedup(a []int64) []int64 {
	if len(a) == 0 {
		return a
	}
	b := []int64{a[0]}
	last := a[0]
	for _, v := range a[1:] {
		if v != last {
			b = append(b, v)
			last = v
		}
	}
	return b
}
