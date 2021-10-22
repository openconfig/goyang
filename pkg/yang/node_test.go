// Copyright 2019 Google Inc.
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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/errdiff"
)

func TestNodePath(t *testing.T) {
	tests := []struct {
		desc string
		in   Node
		want string
	}{{
		desc: "basic",
		in: &Leaf{
			Name: "bar",
			Parent: &Container{
				Name: "c",
				Parent: &List{
					Name: "b",
					Parent: &Module{
						Name: "foo",
					},
				},
			},
		},
		want: "/foo/b/c/bar",
	}, {
		desc: "nil input node",
		in:   nil,
		want: "",
	}, {
		desc: "single node",
		in: &Module{
			Name: "foo",
		},
		want: "/foo",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if diff := cmp.Diff(NodePath(tt.in), tt.want); diff != "" {
				t.Errorf("(-got, +want):\n%s", diff)
			}
		})
	}
}

// TestNode provides a framework for processing tests that can check particular
// nodes being added to the grammar. It can be used to ensure that particular
// statement combinations are supported, especially where they are opaque to
// the YANG library.
func TestNode(t *testing.T) {
	tests := []struct {
		desc          string
		inFn          func(*Modules) (Node, error)
		inModules     map[string]string
		wantNode      func(Node) error
		wantErrSubstr string
	}{{
		desc: "import reference statement",
		inFn: func(ms *Modules) (Node, error) {

			const module = "test"
			m, ok := ms.Modules[module]
			if !ok {
				return nil, fmt.Errorf("can't find module %q", module)
			}

			if len(m.Import) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}

			return m.Import[0], nil
		},
		inModules: map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					import foo {
						prefix "f";
						reference "bar";
					}
				}
			`,
			"foo": `
				module foo {
					prefix "f";
					namespace "urn:f";
				}
			`,
		},
		wantNode: func(n Node) error {
			is, ok := n.(*Import)
			if !ok {
				return fmt.Errorf("got node: %v, want type: import", n)
			}

			switch {
			case is.Reference == nil:
				return errors.New("did not get expected reference, got: nil, want: *yang.Statement")
			case is.Reference.Statement().Argument != "bar":
				return fmt.Errorf("did not get expected reference, got: %v, want: 'bar'", is.Reference.Statement())
			}

			return nil
		},
	}, {
		desc: "get submodule from prefix in submodule",
		inFn: func(ms *Modules) (Node, error) {

			m, ok := ms.SubModules["foo"]
			if !ok {
				return nil, fmt.Errorf("can't find submodule in %v", ms)
			}

			if m.BelongsTo == nil {
				return nil, fmt.Errorf("node %v is missing belongs-to", m)
			}

			return m.BelongsTo, nil
		},
		inModules: map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					include foo {
						revision-date 2008-01-01;
					}
				}
			`,
			"foo": `
				submodule foo {
					belongs-to test {
					  prefix "t";
					}
				}
			`,
		},
		wantNode: func(n Node) error {
			is, ok := n.(*BelongsTo)
			if !ok {
				return fmt.Errorf("got node: %v, want type: belongs-to", n)
			}

			switch {
			case is.Prefix == nil:
				return errors.New("did not get expected reference, got: nil, want: *yang.Statement")
			case is.Prefix.Statement().Argument != "t":
				return fmt.Errorf("did not get expected reference, got: %v, want: 't'", is.Prefix.Statement())
			}

			m := FindModuleByPrefix(is, is.Prefix.Statement().Argument)
			if m == nil {
				return fmt.Errorf("can't find module from submodule's belongs-to prefix value")
			}
			if want := "foo"; m.Name != want {
				return fmt.Errorf("module from submodule's belongs-to prefix value doesn't match, got %q, want %q", m.Name, want)
			}

			return nil
		},
	}, {
		desc: "import statement from submodule",
		inFn: func(ms *Modules) (Node, error) {

			m, ok := ms.SubModules["foo"]
			if !ok {
				return nil, fmt.Errorf("can't find submodule in %v", ms)
			}

			if len(m.Import) == 0 {
				return nil, fmt.Errorf("node %v is missing import statement", m)
			}

			return m.Import[0], nil
		},
		inModules: map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					include foo {
						revision-date 2008-01-01;
					}

					typedef t {
						type string;
					}
				}
			`,
			"foo": `
				submodule foo {
					belongs-to test {
					  prefix "t";
					}

					import test2 {
						prefix "t2";
						description "test2 module";
					}
				}
			`,
			"test2": `
				module test2 {
					prefix "t2";
					namespace "urn:t2";
				}
			`,
		},
		wantNode: func(n Node) error {
			is, ok := n.(*Import)
			if !ok {
				return fmt.Errorf("got node: %v, want type: belongs-to", n)
			}

			switch {
			case is.Prefix == nil:
				return errors.New("did not get expected reference, got: nil, want: *yang.Statement")
			case is.Prefix.Statement().Argument != "t2":
				return fmt.Errorf("did not get expected reference, got: %v, want: 't'", is.Prefix.Statement())
			}

			m := FindModuleByPrefix(is, is.Prefix.Statement().Argument)
			if m == nil {
				return fmt.Errorf("can't find module from submodule's import prefix value")
			}
			if want := "test2"; m.Name != want {
				return fmt.Errorf("module from submodule's import prefix value doesn't match, got %q, want %q", m.Name, want)
			}

			return nil
		},
	}, {
		desc: "import description statement",
		inFn: func(ms *Modules) (Node, error) {

			const module = "test"
			m, ok := ms.Modules[module]
			if !ok {
				return nil, fmt.Errorf("can't find module %q", module)
			}

			if len(m.Import) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}

			return m.Import[0], nil
		},
		inModules: map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					import foo {
						prefix "f";
						description "foo module";
					}
				}
			`,
			"foo": `
				module foo {
					prefix "f";
					namespace "urn:f";
				}
			`,
		},
		wantNode: func(n Node) error {
			is, ok := n.(*Import)
			if !ok {
				return fmt.Errorf("got node: %v, want type: import", n)
			}

			switch {
			case is.Description == nil:
				return errors.New("did not get expected reference, got: nil, want: *yang.Statement")
			case is.Description.Statement().Argument != "foo module":
				return fmt.Errorf("did not get expected reference, got: '%v', want: 'foo module'", is.Description.Statement().Argument)
			}

			return nil
		},
	}, {
		desc: "Test matchingExtensions",
		inFn: func(ms *Modules) (Node, error) {

			module := "test"
			m, ok := ms.Modules[module]
			if !ok {
				return nil, fmt.Errorf("can't find module %q", module)
			}

			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}

			module = "foo"
			if _, ok := ms.Modules[module]; !ok {
				return nil, fmt.Errorf("can't find module %q", module)
			}

			return m.Leaf[0].Type, nil
		},
		inModules: map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					import foo {
						prefix "f";
						description "foo module";
					}

					import foo2 {
						prefix "f2";
						description "foo2 module";
					}

					leaf test-leaf {
						type string {
							pattern 'alpha';
							// Test different modules and different ext names.
							f:bar 'boo';
							f2:bar 'boo2';

							f:bar 'coo';
							f2:bar 'coo2';

							f:far 'doo';
							f2:far 'doo2';

							f:bar 'foo';
							f2:bar 'foo2';

							f:far 'goo';
							f2:far 'goo2';
						}
					}
				}
			`,
			"foo": `
				module foo {
					prefix "f";
					namespace "urn:f";

					extension bar {
						argument "baz";
					}

					extension far {
						argument "baz";
					}
				}
			`,
			"foo2": `
				module foo2 {
					prefix "f2";
					namespace "urn:f2";

					extension bar {
						argument "baz";
					}

					extension far {
						argument "baz";
					}
				}
			`,
		},
		wantNode: func(n Node) error {
			n, ok := n.(*Type)
			if !ok {
				return fmt.Errorf("got node: %v, want type: Leaf", n)
			}

			var bars []string
			matches, err := matchingExtensions(n, n.Exts(), "foo", "bar")
			if err != nil {
				return err
			}
			for _, ext := range matches {
				bars = append(bars, ext.Argument)
			}

			if diff := cmp.Diff(bars, []string{"boo", "coo", "foo"}); diff != "" {
				return fmt.Errorf("matchingExtensions (-got, +want):\n%s", diff)
			}

			return nil
		},
	}, {
		desc: "Test matchingExtensions when module is not found",
		inFn: func(ms *Modules) (Node, error) {

			module := "test"
			m, ok := ms.Modules[module]
			if !ok {
				return nil, fmt.Errorf("can't find module %q", module)
			}

			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}

			module = "foo"
			if _, ok := ms.Modules[module]; !ok {
				return nil, fmt.Errorf("can't find module %q", module)
			}

			return m.Leaf[0].Type, nil
		},
		inModules: map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					import foo {
						prefix "f";
						description "foo module";
					}

					leaf test-leaf {
						type string {
							pattern 'alpha';
							not-found:bar 'foo';
						}
					}
				}
			`,
			"foo": `
				module foo {
					prefix "f";
					namespace "urn:f";

					extension bar {
						argument "baz";
					}

					extension far {
						argument "baz";
					}
				}
			`,
		},
		wantNode: func(n Node) error {
			n, ok := n.(*Type)
			if !ok {
				return fmt.Errorf("got node: %v, want type: Leaf", n)
			}

			var bars []string
			matches, err := matchingExtensions(n, n.Exts(), "foo", "bar")
			if err != nil {
				return err
			}
			for _, ext := range matches {
				bars = append(bars, ext.Argument)
			}

			if diff := cmp.Diff(bars, []string{"boo", "coo", "foo"}); diff != "" {
				return fmt.Errorf("matchingExtensions (-got, +want):\n%s", diff)
			}

			return nil
		},
		wantErrSubstr: `module prefix "not-found" not found`,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()

			for n, m := range tt.inModules {
				if err := ms.Parse(m, n); err != nil {
					t.Errorf("error parsing module %s, got: %v, want: nil", n, err)
				}
			}

			errs := ms.Process()
			var err error
			if len(errs) > 1 {
				t.Fatalf("Got more than 1 error: %v", errs)
			} else if len(errs) == 1 {
				err = errs[0]
			}
			if diff := errdiff.Substring(err, tt.wantErrSubstr); diff != "" {
				t.Errorf("Did not get expected error: %s", diff)
			}
			if err != nil {
				return
			}

			node, err := tt.inFn(ms)
			if err != nil {
				t.Fatalf("cannot run in function, %v", err)
			}

			if err := tt.wantNode(node); err != nil {
				t.Fatalf("failed check function, %v", err)
			}
		})
	}
}

func TestModulesFindByPrefix(t *testing.T) {
	// Some examples of where prefixes might be used are in the following
	// YANG statements: extension, uses, augment, deviation, type, leafref.
	// Not all are put into the test here, since the logic is the same for
	// each.
	modules := map[string]string{
		"foo": `module foo { prefix "foo"; namespace "urn:foo"; include bar; leaf leafref { type leafref { path "../foo:leaf"; } } uses foo:lg; }`,
		"bar": `submodule bar { belongs-to foo { prefix "bar"; } container c { uses bar:lg; } grouping lg { leaf leaf { type string; } } }`,
		"baz": `module baz { prefix "foo"; namespace "urn:foo"; import foo { prefix f; } extension e; uses f:lg; foo:e; }`,
	}

	ms := NewModules()
	for name, modtext := range modules {
		if err := ms.Parse(modtext, name+".yang"); err != nil {
			t.Fatalf("error parsing module %q: %v", name, err)
		}
	}
	if errs := ms.Process(); errs != nil {
		for _, err := range errs {
			t.Errorf("error: %v", err)
		}
		t.Fatalf("fatal error(s) calling Process()")
	}

	for _, tt := range []struct {
		desc   string
		node   Node
		prefix string
		want   *Module
	}{
		{
			desc:   "nil node",
			node:   nil,
			prefix: "does-not-exist",
			want:   nil,
		},
		{
			desc:   "module foo",
			node:   ms.Modules["foo"],
			prefix: "foo",
			want:   ms.Modules["foo"],
		},
		{
			desc:   "submodule bar",
			node:   ms.SubModules["bar"],
			prefix: "bar",
			want:   ms.SubModules["bar"],
		},
		{
			desc:   "module baz",
			node:   ms.Modules["baz"],
			prefix: "foo",
			want:   ms.Modules["baz"],
		},
		{
			desc:   "foo leafref",
			node:   ms.Modules["foo"].Leaf[0].Type,
			prefix: "foo",
			want:   ms.Modules["foo"],
		},
		{
			desc:   "foo uses",
			node:   ms.Modules["foo"].Uses[0],
			prefix: "foo",
			want:   ms.Modules["foo"],
		},
		{
			desc:   "bar uses",
			node:   ms.SubModules["bar"].Container[0].Uses[0],
			prefix: "bar",
			want:   ms.SubModules["bar"],
		},
		{
			desc:   "baz uses",
			node:   ms.Modules["baz"].Uses[0],
			prefix: "f",
			want:   ms.Modules["foo"],
		},
		{
			desc:   "baz extension",
			node:   ms.Modules["baz"],
			prefix: "foo",
			want:   ms.Modules["baz"],
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			if got := FindModuleByPrefix(tt.node, tt.prefix); got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}
