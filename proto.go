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
	"fmt"
	"io"
	"sort"

	"github.com/openconfig/goyang/pkg/indent"
	"github.com/openconfig/goyang/pkg/yang"
)

func init() {
	register(&formatter {
		name: "proto",
		f: doProto,
		help: "display tree in a proto like format",
	})
}

func doProto(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
		for _, e := range flatten(e) {
			FormatNode(w, e)
		}
	}
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

func isStream(e *yang.Entry) bool {
	for _, ext := range e.Exts {
		if ext.Kind() == "grpc:stream" {
			return true
		}
	}
	return false
}

// FormatNode writes e, formatted almost like a protobuf message, to w.
func FormatNode(w io.Writer, e *yang.Entry) {
	var names []string

	for k, se := range e.Dir {
		if se.RPC != nil {
			names = append(names, k)
		}
	}
	needEmpty := false
	if len(names) > 0 {
		sort.Strings(names)
		fmt.Fprintf(w, "service %s {\n", fixName(e.Name))
		for _, k := range names {
			rpc := e.Dir[k].RPC
			k = fixName(k)
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
				FormatNode(w, rpc.Input)
			}
			if rpc.Output != nil {
				FormatNode(w, rpc.Output)
			}
		}
	}

	if needEmpty {
		fmt.Fprintln(w, "\nmessage Empty { }")
	}

	names = nil
	for k, se := range e.Dir {
		if se.RPC == nil {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		return
	}

	fmt.Fprintln(w)
	if e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}
	fmt.Fprintf(w, "message %s {\n", fixName(e.Name))

	sort.Strings(names)
	for x, k := range names {
		se := e.Dir[k]
		if se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		if se.ListAttr != nil {
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
