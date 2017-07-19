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

// This file contains source related to constructing an AST of Nodes from a
// Statement tree.  It also include code that builds up the basic YANG parser
// based on the various Node types (see yang.go).
//
// The initTypes function generates the functions that actually fill in the
// various Node structuers defined in yang.go.  BuildAST uses those functions to
// convert generic Statments into an AST.

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// A yangStatement contains all information needed to parse a particular
// type of statement.
//
// funcs is the map of YANG field names to the function that handle them.
// required is a list of fields that must be present in the statement.
// sRequired is maps statement type to required fields
//    If a field is required by statement type foo, then only foo should
//    have the field.
// addext is the function to handle possible extensions.
type yangStatement struct {
	funcs     map[string]func(*Statement, reflect.Value, reflect.Value) error
	required  []string
	sRequired map[string][]string
	addext    func(*Statement, reflect.Value, reflect.Value) error
}

var (
	// The following maps are built up at init time.
	// nameMap provides a lookup from a keyword string to the related
	// yangStatement in the typeMap to prevent redundant processing.
	typeMap = map[reflect.Type]*yangStatement{}
	nameMap = map[string]reflect.Type{}

	statementType = reflect.TypeOf(&Statement{})
	nilValue      = reflect.ValueOf(nil)
	// There has to be a better way to do this
	nodeType = reflect.TypeOf(struct{ Node }{}).Field(0).Type
)

// meta is a collection of possible top level statements.  There is no actual
// statement named "meta".  All other statements are a sub-statement of one
// of the meta statements.
type meta struct {
	Module []*Module `yang:"module"`
}

func init() {
	initTypes(reflect.TypeOf(&meta{}))
}

// aliases is a map of "aliased" names, that is, two types of statements
// that parse (nearly) the same.
var aliases = map[string]string{
	"submodule": "module",
}

// BuildAST builds an abstract syntax tree based on the yang statement s.
// Normally it should return a *Module.
func BuildAST(s *Statement) (Node, error) {
	v, err := build(s, nilValue)
	if err != nil {
		return nil, err
	}
	return v.Interface().(Node), nil
}

// build builds and returns an AST from the statement s, with parent p, or
// returns an error.  The type of value returned depends on the keyword in s.
func build(s *Statement, p reflect.Value) (v reflect.Value, err error) {
	defer func() {
		// If we are returning a real Node then call addTypedefs
		// if the node possibly contains typedefs.
		if err != nil || v == nilValue {
			return
		}
		if t, ok := v.Interface().(Typedefer); ok {
			addTypedefs(t)
		}
	}()
	kind := s.Keyword
	if k := aliases[s.Keyword]; k != "" {
		kind = k
	}
	t := nameMap[kind]
	if t == nil {
		// It is not an error if this is an extension.
		// TODO(borman): what do we do with them?
		if strings.Index(s.Keyword, ":") > 0 {
			return nilValue, nil
		}
		return nilValue, fmt.Errorf("%s: unknown statement: %s", s.Location(), s.Keyword)
	}
	y := typeMap[t]
	found := map[string]bool{}

	t = t.Elem()       // Get the struct type we are pointing to
	v = reflect.New(t) // v is a pointer to the structure we are building

	// Handle special cases that are not actually substatements:

	if fn := y.funcs["Name"]; fn != nil {
		// Name uses s directly.
		if err := fn(s, v, p); err != nil {
			return nilValue, err
		}
	}
	if fn := y.funcs["Statement"]; fn != nil {
		// Statement uses s directly.
		if err := fn(s, v, p); err != nil {
			return nilValue, err
		}
	}
	if fn := y.funcs["Parent"]; fn != nil {
		// p is the parent.  If there is no parent then p is nilValue,
		// which is the reflect.Value of nil.
		// p.IsValid will return false when p is a nil interface
		// p.IsValid will true if p is references a concrete type
		// (even if it is nil).
		if p.IsValid() {
			if err := fn(s, v, p); err != nil {
				return nilValue, err
			}
		}
	}

	// Now handle the substatements

	for _, ss := range s.statements {
		found[ss.Keyword] = true
		fn := y.funcs[ss.Keyword]
		parts := strings.Split(ss.Keyword, ":")
		switch {
		case fn != nil:
			// Normal case, the keyword is known.
			if err := fn(ss, v, p); err != nil {
				return nilValue, err
			}
		case len(parts) == 2:
			// Keyword is not known but it has a prefix so it might
			// be an extension.
			if y.addext == nil {
				return nilValue, fmt.Errorf("%s: no extension function", ss.Location())
			}
			y.addext(ss, v, p)
		default:
			return nilValue, fmt.Errorf("%s: unknown %s field: %s", ss.Location(), s.Keyword, ss.Keyword)
		}
	}

	// Make sure all of our required field are there.
	for _, r := range y.required {
		if !found[r] {
			return nilValue, fmt.Errorf("%s: missing required %s field: %s", s.Location(), s.Keyword, r)
		}
	}

	// Make sure required fields based on our keyword are there (module vs submodule)
	for _, r := range y.sRequired[s.Keyword] {
		if !found[r] {
			return nilValue, fmt.Errorf("%s: missing required %s field: %s", s.Location(), s.Keyword, r)
		}
	}

	// Make sure we don't have any field set that is required by a different keyword.
	for n, or := range y.sRequired {
		if n == s.Keyword {
			continue
		}
		for _, r := range or {
			if found[r] {
				return nilValue, fmt.Errorf("%s: unknown %s field: %s", s.Location(), s.Keyword, r)
			}
		}
	}
	return v, nil
}

// initTypes builds up the functions necessary to parse a Statement into the
// type at.  at must be a of type pointer to structure and that structure should
// implement Node.  For each field of the structure with a yang tag (e.g.,
// `yang:"command"`), a function is created and "command" is mapped to it.  The
// complete map is then added to the typeMap map with at as the key.
//
// The functions have the form:
//
//	 func fn(ss *Statement, v, p reflect.Value) error
//
// Given s as a statement of type at, ss is a substatement of s (in a few
// exceptional cases, ss is the Statement itself).  v must have the type at and
// is the structure being filled in.  p is the parent Node, or nil.  p is only
// used to set the Parent field of a Node.  For example, given the following
// structure and variables:
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
// ds, of s that has the keyword "revision-date" along with the value of
// vInc and its parent:
//
//	typeMap[tInc]["revision-date"](ss, vInc, parent)
//
// Normal fields are all processed this same way.
//
// The other 4 fields are special.  In the case of Name, Statement, and Parent,
// the function is passed s, rather than ss, as these fields are not filled in
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
	if typeMap[at] != nil {
		return // we already defined this type
	}
	if at.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("interface not a pointer, is %v", at))
	}
	t := at.Elem()
	if t.Kind() != reflect.Struct {
		panic("interface not a pointer to struct")
	}
	n := t.NumField()

	y := &yangStatement{
		funcs:     make(map[string]func(*Statement, reflect.Value, reflect.Value) error, n),
		sRequired: make(map[string][]string),
	}
	typeMap[at] = y

	for i := 0; i < n; i++ {
		i := i
		f := t.Field(i)
		yang := f.Tag.Get("yang")
		if yang == "" {
			continue
		}
		parts := strings.Split(yang, ",")
		name := parts[0]
		if a := aliases[name]; a != "" {
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
			// s is the extension to put into v at for field f.
			y.addext = func(s *Statement, v, _ reflect.Value) error {
				if v.Type() != at {
					panic(fmt.Sprintf("given type %s, need type %s", v.Type(), at))
				}
				fv := v.Elem().Field(i)
				fv.Set(reflect.Append(fv, reflect.ValueOf(s)))
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
		var fn func(*Statement, reflect.Value, reflect.Value) error

		// The field can be a pointer, a slice or a string
		switch f.Type.Kind() {
		default:
			panic(fmt.Sprintf("invalid type: %v", f.Type.Kind()))

		case reflect.Interface:
			// The only case of this should be the "Parent" field.
			if name != "Parent" {
				panic(fmt.Sprintf("interface field is %s, not Parent", name))
			}
			fn = func(s *Statement, v, p reflect.Value) error {
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
			fn = func(s *Statement, v, _ reflect.Value) error {
				if v.Type() != at {
					panic(fmt.Sprintf("got type %v, want %v", v.Type(), at))
				}
				fv := v.Elem().Field(i)
				if fv.String() != "" {
					return errors.New(s.Keyword + ": already set")
				}

				v.Elem().Field(i).SetString(s.Argument)
				return nil
			}

		case reflect.Ptr:
			if f.Type == statementType {
				// The only case of this should be the
				// "Statement" field
				if name != "Statement" {
					panic(fmt.Sprintf("string field is %s, not Statement", name))
				}
				fn = func(s *Statement, v, _ reflect.Value) error {
					if v.Type() != at {
						panic(fmt.Sprintf("got type %v, want %v", v.Type(), at))
					}
					v.Elem().Field(i).Set(reflect.ValueOf(s))
					return nil
				}
				break
			}

			// Make sure our field type is also setup.
			descend(name, f.Type)

			fn = func(s *Statement, v, p reflect.Value) error {
				if v.Type() != at {
					panic(fmt.Sprintf("given type %s, need type %s", v.Type(), at))
				}
				fv := v.Elem().Field(i)
				if !fv.IsNil() {
					return errors.New(s.Keyword + ": already set")
				}

				// Use build to build the value for this field.
				sv, err := build(s, v)
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
				fn = func(s *Statement, v, p reflect.Value) error {
					if v.Type() != at {
						panic(fmt.Sprintf("given type %s, need type %s", v.Type(), at))
					}
					sv, err := build(s, v)
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
