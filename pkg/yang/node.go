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
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/openconfig/goyang/pkg/indent"
)

// A Node contains a yang statement and all attributes and sub-statements.
// Only pointers to structures should implement Node.
type Node interface {
	// Kind returns the kind of yang statement (the keyword).
	Kind() string
	// NName returns the node's name (the argument)
	NName() string
	// Statement returns the original Statement of this Node.
	Statement() *Statement
	// ParentNode returns the parent of this Node, or nil if the
	// Node has no parent.
	ParentNode() Node
	// Exts returns the list of extension statements found.
	Exts() []*Statement
}

// A Typedefer is a Node that defines typedefs.
type Typedefer interface {
	Node
	Typedefs() []*Typedef
}

// An ErrorNode is a node that only contains an error.
type ErrorNode struct {
	Parent Node `yang:"Parent,nomerge"`

	Error error
}

func (ErrorNode) Kind() string             { return "error" }
func (s *ErrorNode) ParentNode() Node      { return s.Parent }
func (s *ErrorNode) NName() string         { return "error" }
func (s *ErrorNode) Statement() *Statement { return &Statement{} }
func (s *ErrorNode) Exts() []*Statement    { return nil }

// isRPCNode is a terrible hack to return back that a path points into
// an RPC and we should ignore it.
var isRPCNode = &ErrorNode{Error: errors.New("rpc is unsupported")}

// Source returns the location of the source where n was defined.
func Source(n Node) string {
	if n != nil && n.Statement() != nil {
		return n.Statement().Location()
	}
	return "unknown"
}

// getPrefix returns the prefix and base name of s.  If s has no prefix
// then the returned prefix is "".
func getPrefix(s string) (string, string) {
	f := strings.SplitN(s, ":", 2)
	if len(f) == 1 {
		return "", s
	}
	return f[0], f[1]
}

// Prefix notes for types:
//
// If there is prefix, look in nodes ancestors.
//
// If prefix matches the module's prefix statement, look in nodes ancestors.
//
// If prefix matches the submodule's belongs-t statement, look in nodes
// ancestors.
//
// Finally, look in the module imported with prefix.

// FindModuleByPrefix finds the module or submodule with the provided prefix
// relative to where n was defined.  If the prefix cannot be resolved then nil
// is returned.
func FindModuleByPrefix(n Node, prefix string) *Module {
	n = RootNode(n)

	mod := n.(*Module)
	if prefix == "" {
		return mod
	}

	switch mod.Kind() {
	case "module":
		if mod.Prefix.Name == prefix {
			return mod
		}
	case "submodule":
		if prefix == mod.BelongsTo.Prefix.Name {
			return mod
		}
	default:
		panic("root that is not a module or submodule")
	}

	for _, i := range mod.Import {
		if prefix == i.Prefix.Name {
			return i.Module
		}
	}
	return nil
}

// RootNode returns the submodule or module that n was defined in.
func RootNode(n Node) *Module {
	for ; n.ParentNode() != nil; n = n.ParentNode() {
	}
	if mod, ok := n.(*Module); ok {
		return mod
	}
	return nil
}

// FindNode finds the node referenced by path relative to n.  If path does not
// reference a node then nil is returned (i.e. path not found).  The path looks
// similar to an XPath but curently has no wildcarding.  For example:
// "/if:interfaces/if:interface" and "../config".
func FindNode(n Node, path string) (Node, error) {
	if path == "" {
		return n, nil
	}
	// / is not a valid path, it needs a module name
	if path == "/" {
		return nil, fmt.Errorf("invalid path %q", path)
	}
	// Paths do not end in /'s
	if path[len(path)-1] == '/' {
		return nil, fmt.Errorf("invalid path %q", path)
	}

	parts := strings.Split(path, "/")

	// An absolute path has a leading component of "".
	// We need to discover which module they are part of
	// based on our imports.
	if parts[0] == "" {
		parts = parts[1:]

		// TODO(borman): merge this with FindModuleByPrefix?
		n = RootNode(n)
		// The base is always a module
		mod := n.(*Module)
		prefix, _ := getPrefix(parts[0])
		if mod.Kind() == "submodule" {
			m := mod.modules.Modules[mod.BelongsTo.Name]
			if m == nil {
				return nil, fmt.Errorf("%s: unknown module %s", m.Name, mod.BelongsTo.Name)
			}
			if prefix == "" || prefix == mod.BelongsTo.Prefix.Name {
				mod = m
				goto processing
			}
			mod = m
		}

		if prefix == "" || prefix == mod.Prefix.Name {
			goto processing
		}

		for _, i := range mod.Import {
			if prefix == i.Prefix.Name {
				n = i.Module
				goto processing
			}
		}
		// We didn't find a matching prefix.
		return nil, fmt.Errorf("unknown prefix: %q", prefix)
	processing:
		// At this point, n should be pointing to the Module node
		// of module we are rooted in
	}

	for _, part := range parts {
		// If we encounter an RPC node in our search then we
		// return the magic isRPCNode Node which just contains
		// an error that it is an RPC node.  isRPCNode is a singleton
		// and can be checked against.
		if n.Kind() == "rpc" {
			return isRPCNode, nil
		}
		if part == ".." {
		Loop:
			for {
				n = n.ParentNode()
				if n == nil {
					return nil, fmt.Errorf(".. with no parent")
				}
				// choice, leaf, and case nodes
				// are "invisible" when doing ".."
				// up the tree.
				switch n.Kind() {
				case "choice", "leaf", "case":
				default:
					break Loop
				}
			}
			continue
		}
		// For now just strip off any prefix
		// TODO(borman): fix this
		_, spart := getPrefix(part)
		n = ChildNode(n, spart)
		if n == nil {
			return nil, fmt.Errorf("%s: no such element", part)
		}
	}
	return n, nil
}

// ChildNode finds n's child node named name.  It returns nil if the node
// could not be found.  ChildNode looks at every direct Node pointer in
// n as well as every node in all slices of Node pointers.  Names must
// be non-ambiguous, otherwise ChildNode has a non-deterministic result.
func ChildNode(n Node, name string) Node {
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()

Loop:
	for i := 0; i < nf; i++ {
		ft := t.Field(i)
		yang := ft.Tag.Get("yang")
		if yang == "" {
			continue
		}
		parts := strings.Split(yang, ",")
		for _, p := range parts[1:] {
			if p == "nomerge" {
				continue Loop
			}
		}

		f := v.Field(i)
		if !f.IsValid() || f.IsNil() {
			continue
		}

		check := func(n Node) Node {
			if n.NName() == name {
				return n
			}
			return nil
		}
		if parts[0] == "uses" {
			check = func(n Node) Node {
				uname := n.NName()
				// unrooted uses are rooted at root
				if !strings.HasPrefix(uname, "/") {
					uname = "/" + uname
				}
				if n, _ = FindNode(n, uname); n != nil {
					return ChildNode(n, name)
				}
				return nil
			}
		}

		switch ft.Type.Kind() {
		case reflect.Ptr:
			if n = check(f.Interface().(Node)); n != nil {
				return n
			}
		case reflect.Slice:
			sl := f.Len()
			for i := 0; i < sl; i++ {
				n = f.Index(i).Interface().(Node)
				if n = check(n); n != nil {
					return n
				}
			}
		}
	}
	return nil
}

// PrintNode prints node n to w, recursively.
// TODO(borman): display more information
func PrintNode(w io.Writer, n Node) {
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	fmt.Fprintf(w, "%s [%s]\n", n.NName(), n.Kind())
Loop:
	for i := 0; i < nf; i++ {
		ft := t.Field(i)
		yang := ft.Tag.Get("yang")
		if yang == "" {
			continue
		}
		parts := strings.Split(yang, ",")
		for _, p := range parts[1:] {
			if p == "nomerge" {
				continue Loop
			}
		}

		// Skip uppercase elements.
		if parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
			continue
		}

		f := v.Field(i)
		if !f.IsValid() || f.IsNil() {
			continue
		}

		switch ft.Type.Kind() {
		case reflect.Ptr:
			n = f.Interface().(Node)
			if v, ok := n.(*Value); ok {
				fmt.Fprintf(w, "%s = %s\n", ft.Name, v.Name)
			} else {
				PrintNode(indent.NewWriter(w, "    "), n)
			}
		case reflect.Slice:
			sl := f.Len()
			for i := 0; i < sl; i++ {
				n = f.Index(i).Interface().(Node)
				if v, ok := n.(*Value); ok {
					fmt.Fprintf(w, "%s[%d] = %s\n", ft.Name, i, v.Name)
				} else {
					PrintNode(indent.NewWriter(w, "    "), n)
				}
			}
		}
	}
}
