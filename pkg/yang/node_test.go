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

			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
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
		desc: "import description statement",
		inFn: func(ms *Modules) (Node, error) {

			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
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

			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}

			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}

			_, err = ms.FindModuleByPrefix("f")
			if err != nil {
				return nil, err
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

			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}

			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}

			_, err = ms.FindModuleByPrefix("f")
			if err != nil {
				return nil, err
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
