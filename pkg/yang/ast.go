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

// This file implements BuildAST() and its associated helper structs and
// functions for constructing an AST of Nodes from a Statement tree. This
// function also populates all typedefs into a type cache.
//
// The initTypes function generates the helper struct and functions that
// recursively fill in the various Node structures defined in yang.go.
// BuildAST() then uses those functions to convert raw parsed Statements into
// an AST.

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func init() {
	// Initialize the global variables `typeMap` and `nameMap`.
	// By doing this, we are making the assumption that all modules will be
	// parsed according to the type hierarchy rooted at `meta`, and thus
	// all input YANG modules will be parsed in this manner.
	initTypes(reflect.TypeOf(&meta{}))
}

// A yangStatement contains all information needed to build a particular
// type of statement into an AST node.
type yangStatement struct {
	// funcs is the map of YANG field names to the function that populates
	// the statement into the AST node.
	funcs map[string]func(*Statement, reflect.Value, reflect.Value, *typeDictionary) error
	// required is a list of fields that must be present in the statement.
	required []string
	// sRequired maps a statement name to a list of required sub-field
	// names. The statement name can be an alias of the primary field type.
	//    e.g. If a field is required by statement type foo, then only foo
	//    should have the field. If bar is an alias of foo, it must not
	//    have this field.
	sRequired map[string][]string
	// addext is the function to handle possible extensions.
	addext func(*Statement, reflect.Value, reflect.Value) error
}

// newYangStatement creates a new yangStatement.
func newYangStatement() *yangStatement {
	return &yangStatement{
		funcs:     make(map[string]func(*Statement, reflect.Value, reflect.Value, *typeDictionary) error),
		sRequired: make(map[string][]string),
	}
}

var (
	// The following maps are built up at init time.
	// typeMap provides a lookup from a Node type to the corresponding
	// yangStatement.
	typeMap = map[reflect.Type]*yangStatement{}
	// nameMap provides a lookup from a keyword string to the corresponding
	// concrete type implementing the Node interface (see yang.go).
	nameMap = map[string]reflect.Type{}

	// The following are helper types used by the implementation.
	statementType = reflect.TypeOf(&Statement{})
	nilValue      = reflect.ValueOf(nil)
	// nodeType is the reflect.Type of the Node interface.
	nodeType = reflect.TypeOf((*Node)(nil)).Elem()
)

// meta is a collection of top-level statements.  There is no actual
// statement named "meta".  All other statements are a sub-statement of one
// of the meta statements.
type meta struct {
	Module []*Module `yang:"module"`
}

// aliases is a map of "aliased" names, that is, two types of statements
// that parse (nearly) the same.
// NOTE: This only works for root-level aliasing for now, which is good enough
// for module/submodule. This is because yangStatement.funcs doesn't store the
// handler function for aliased fields, and sRequired also may only store the
// correct values when processing a root-level statement due to aliasing. These
// issues would need to be fixed in order to support aliasing for non-top-level
// statements.
var aliases = map[string]string{
	"submodule": "module",
}

// buildASTWithTypeDict creates an AST for the input statement, and returns its
// root node. It also takes as input a type dictionary into which any
// encountered typedefs within the statement are cached.
func buildASTWithTypeDict(stmt *Statement, types *typeDictionary) (Node, error) {
	v, err := build(stmt, nilValue, types)
	if err != nil {
		return nil, err
	}
	return v.Interface().(Node), nil
}

// build builds and returns an AST from the statement stmt and with parent node
// parent. It also takes as input a type dictionary types into which any
// encountered typedefs within the statement are cached. The type of value
// returned depends on the keyword in stmt (see yang.go). It returns an error
// if it cannot build the statement into its corresponding Node type.
func build(stmt *Statement, parent reflect.Value, types *typeDictionary) (v reflect.Value, err error) {
	defer func() {
		// If we are returning a real Node then call addTypedefs
		// if the node possibly contains typedefs.
		// Cache these in the typedef cache for look-ups.
		if err != nil || v == nilValue {
			return
		}
		if t, ok := v.Interface().(Typedefer); ok {
			types.addTypedefs(t)
		}
	}()
	keyword := stmt.Keyword
	if k, ok := aliases[stmt.Keyword]; ok {
		keyword = k
	}
	t := nameMap[keyword]
	y := typeMap[t]
	// Keep track of which substatements are present in the statement.
	found := map[string]bool{}

	// Get the struct type we are pointing to.
	t = t.Elem()
	// v is a pointer to the instantiated structure we are building.
	v = reflect.New(t)

	// Handle special cases that are not actually substatements:

	if fn := y.funcs["Name"]; fn != nil {
		// Name uses stmt directly.
		if err := fn(stmt, v, parent, types); err != nil {
			return nilValue, err
		}
	}
	if fn := y.funcs["Statement"]; fn != nil {
		// Statement uses stmt directly.
		if err := fn(stmt, v, parent, types); err != nil {
			return nilValue, err
		}
	}
	if fn := y.funcs["Parent"]; fn != nil {
		// parent is the parent node, which is nilValue (reflect.ValueOf(nil)) if there is none.
		// parent.IsValid will return false when parent is a nil interface
		// parent.IsValid will true if parent references a concrete type
		// (even if it is nil).
		if parent.IsValid() {
			if err := fn(stmt, v, parent, types); err != nil {
				return nilValue, err
			}
		}
	}

	// Now handle the substatements

	for _, ss := range stmt.statements {
		found[ss.Keyword] = true
		fn := y.funcs[ss.Keyword]
		switch {
		case fn != nil:
			// Normal case, the keyword is known.
			if err := fn(ss, v, parent, types); err != nil {
				return nilValue, err
			}
		case len(strings.Split(ss.Keyword, ":")) == 2:
			// Keyword is not known but it has a prefix so it might
			// be an extension.
			if y.addext == nil {
				return nilValue, fmt.Errorf("%s: no extension function", ss.Location())
			}
			y.addext(ss, v, parent)
		default:
			return nilValue, fmt.Errorf("%s: unknown %s field: %s", ss.Location(), stmt.Keyword, ss.Keyword)
		}
	}

	// Make sure all of our required field are there.
	for _, r := range y.required {
		if !found[r] {
			return nilValue, fmt.Errorf("%s: missing required %s field: %s", stmt.Location(), stmt.Keyword, r)
		}
	}

	// Make sure required fields based on our keyword are there (module vs submodule)
	for _, r := range y.sRequired[stmt.Keyword] {
		if !found[r] {
			return nilValue, fmt.Errorf("%s: missing required %s field: %s", stmt.Location(), stmt.Keyword, r)
		}
	}

	// Make sure we don't have any field set that is required by a different keyword.
	for n, or := range y.sRequired {
		if n == stmt.Keyword {
			continue
		}
		for _, r := range or {
			if found[r] {
				return nilValue, fmt.Errorf("%s: unknown %s field: %s", stmt.Location(), stmt.Keyword, r)
			}
		}
	}
	return v, nil
}

// initTypes creates the functions necessary to build a Statement into the
// given the type "at" based on its possible substatements. at must implement
// Node, with its concrete type being a pointer to a struct defined in yang.go.
//
// This function also builds up the functions to populate the input type
// dictionary types with any encountered typedefs within the statement.
//
// For each field of the struct with a yang tag (e.g., `yang:"command"`), a
// function is created with "command" as its unique ID.  The complete map of
// builder functions for at is then added to the typeMap map with at as the
// key. The idea is to call these builder functions for each substatement
// encountered.
//
// The functions have the form:
//
//	 func fn(ss *Statement, v, p reflect.Value, types *typeDictionary) error
//
// Given stmt as a Statement of type at, ss is a substatement of stmt (in a few
// exceptional cases, ss is the Statement itself).  v must have the same type
// as at and is the structure being filled in.  p is the parent Node, or nil.
// types is the type dictionary cache of the current set of modules being parsed,
// which is used for looking up typedefs.  p is only used to set the Parent
// field of a Node.  For example, given the following structure and variables:
//
//	type Include struct {
//		Name         string       `yang:"Name"`
//		Source       *Statement   `yang:"Statement"`
//		Parent       Node         `yang:"Parent"`
//		Extensions   []*Statement `yang:"Ext"`
//		RevisionDate *Value       `yang:"revision-date"`
//	}
//
//	var inc = &Include{}
//	var vInc = reflect.ValueOf(inc)
//	var tInc = reflect.TypeOf(inc)
//
// Functions are created for each fields and named Name, Statement, Parent, Ext,
// and revision-date.
//
// The function built for RevisionDate will be called for any substatement,
// ds, of stmt that has the keyword "revision-date" along with the value of
// vInc and its parent:
//
//	typeMap[tInc]["revision-date"](ss, vInc, parent, types)
//
// Normal fields are all processed this same way.
//
// The other 4 fields are special.  In the case of Name, Statement, and Parent,
// the function is passed stmt, rather than ss, as these fields are not filled in
// by substatements.
//
// The Name command must set its field to the Statement's argument.  The
// Statement command must set its field to the Statement itself.  The
// Parent command must set its field with the Node of its parent (the
// parent parameter).
//
// The Ext command is unique and must decode into a []*Statement.  This is a
// slice of all statements that use unknown keywords with a prefix (in a valid
// .yang file these should be the extensions).
//
// The Field can have attributes delimited by a ','.  The only
// supported attributes are:
//
//    nomerge:       Do not merge this field
//    required:      This field must be populated
//    required=KIND: This field must be populated if the keyword is KIND
//                   otherwise this field must not be present.
//                   (This is to support merging Module and SubModule).
//
// If at contains substructures, initTypes recurses on the substructures.
func initTypes(at reflect.Type) {
	if at.Kind() != reflect.Ptr || at.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("interface not a struct pointer, is %v", at))
	}
	if typeMap[at] != nil {
		return // we already defined this type
	}

	y := newYangStatement()
	typeMap[at] = y
	t := at.Elem()
	for i := 0; i != t.NumField(); i++ {
		i := i
		f := t.Field(i)
		yang := f.Tag.Get("yang")
		if yang == "" {
			continue
		}
		parts := strings.Split(yang, ",")
		name := parts[0]
		if a, ok := aliases[name]; ok {
			name = a
		}

		const reqe = "required="
		for _, p := range parts[1:] {
			switch {
			case p == "nomerge":
			case p == "required":
				y.required = append(y.required, name)
			case strings.HasPrefix(p, reqe):
				p = p[len(reqe):]
				y.sRequired[p] = append(y.sRequired[p], name)
			default:
				panic(f.Name + ": unknown tag: " + p)
			}
		}

		// Ext means this is where we squirrel away extensions
		if name == "Ext" {
			// stmt is the extension to put into v at for field f.
			y.addext = func(stmt *Statement, v, _ reflect.Value) error {
				if v.Type() != at {
					panic(fmt.Sprintf("given type %s, need type %s", v.Type(), at))
				}
				fv := v.Elem().Field(i)
				fv.Set(reflect.Append(fv, reflect.ValueOf(stmt)))
				return nil
			}
			continue
		}

		// descend runs initType on dt if it has not already done so.
		descend := func(name string, dt reflect.Type) {
			switch nameMap[name] {
			case nil:
				nameMap[name] = dt
				initTypes(dt) // Make sure that structure type is included
			case dt:
			default:
				panic("redeclared type " + name)
			}
		}

		// Create a function, fn, that will build the field from a
		// Statement.  These functions are used when actually making
		// an AST from a Statement Tree.
		var fn func(*Statement, reflect.Value, reflect.Value, *typeDictionary) error

		// The field can be a pointer, a slice or a string
		switch f.Type.Kind() {
		default:
			panic(fmt.Sprintf("invalid type: %v", f.Type.Kind()))

		case reflect.Interface:
			// The only case of this should be the "Parent" field.
			if name != "Parent" {
				panic(fmt.Sprintf("interface field is %s, not Parent", name))
			}
			fn = func(stmt *Statement, v, p reflect.Value, types *typeDictionary) error {
				if !p.Type().Implements(nodeType) {
					panic(fmt.Sprintf("invalid interface: %v", f.Type.Kind()))
				}
				v.Elem().Field(i).Set(p)
				return nil
			}
		case reflect.String:
			// The only case of this should be the "Name" field
			if name != "Name" {
				panic(fmt.Sprintf("string field is %s, not Name", name))
			}
			fn = func(stmt *Statement, v, _ reflect.Value, types *typeDictionary) error {
				if v.Type() != at {
					panic(fmt.Sprintf("got type %v, want %v", v.Type(), at))
				}
				fv := v.Elem().Field(i)
				if fv.String() != "" {
					return errors.New(stmt.Keyword + ": already set")
				}

				v.Elem().Field(i).SetString(stmt.Argument)
				return nil
			}

		case reflect.Ptr:
			if f.Type == statementType {
				// The only case of this should be the
				// "Statement" field
				if name != "Statement" {
					panic(fmt.Sprintf("string field is %s, not Statement", name))
				}
				fn = func(stmt *Statement, v, _ reflect.Value, types *typeDictionary) error {
					if v.Type() != at {
						panic(fmt.Sprintf("got type %v, want %v", v.Type(), at))
					}
					v.Elem().Field(i).Set(reflect.ValueOf(stmt))
					return nil
				}
				break
			}

			// Make sure our field type is also setup.
			descend(name, f.Type)

			fn = func(stmt *Statement, v, p reflect.Value, types *typeDictionary) error {
				if v.Type() != at {
					panic(fmt.Sprintf("given type %s, need type %s", v.Type(), at))
				}
				fv := v.Elem().Field(i)
				if !fv.IsNil() {
					return errors.New(stmt.Keyword + ": already set")
				}

				// Use build to build the value for this field.
				sv, err := build(stmt, v, types)
				if err != nil {
					return err
				}
				v.Elem().Field(i).Set(sv)
				return nil
			}

		case reflect.Slice:
			// A slice at this point is always a slice of
			// substructures.  We may see the same keyword multiple
			// times, each time we see it we just append to the
			// slice.
			st := f.Type.Elem()
			switch st.Kind() {
			default:
				panic(fmt.Sprintf("invalid type: %v", st.Kind()))
			case reflect.Ptr:
				descend(name, st)
				fn = func(stmt *Statement, v, p reflect.Value, types *typeDictionary) error {
					if v.Type() != at {
						panic(fmt.Sprintf("given type %s, need type %s", v.Type(), at))
					}
					sv, err := build(stmt, v, types)
					if err != nil {
						return err
					}

					fv := v.Elem().Field(i)
					fv.Set(reflect.Append(fv, sv))
					return nil
				}
			}
		}
		y.funcs[name] = fn
	}
}
