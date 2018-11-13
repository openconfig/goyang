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
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/openconfig/gnmi/errdiff"
)

func TestNilEntry(t *testing.T) {
	e := ToEntry(nil)
	_, ok := e.Node.(*ErrorNode)
	if !ok {
		t.Fatalf("ToEntry(nil) did not return an error node")
	}
	errs := e.GetErrors()
	switch len(errs) {
	case 0:
		t.Fatalf("GetErrors returned no error")
	default:
		t.Errorf("got %d errors, wanted 1", len(errs))
		fallthrough
	case 1:
		got := errs[0].Error()
		want := "ToEntry called with nil"
		if got != want {
			t.Fatalf("got error %q, want %q", got, want)
		}
	}
}

var badInputs = []struct {
	name   string
	in     string
	errors []string
}{
	{
		name: "bad.yang",
		in: `
// Base test yang module.
// This module is syntactally correct (we can build an AST) but it is has
// invalid parameters in many statements.
module base {
  namespace "urn:mod";
  prefix "base";

  container c {
    // bad config value in a container
    config bad;
  }
  container d {
    leaf bob {
      // bad config value
      config incorrect;
      type unknown;
    }
    // duplicate leaf entry bob
    leaf bob { type string; }
    // unknown grouping to uses
    uses the-beatles;
  }
  grouping the-group {
    leaf one { type string; }
    // duplicate leaf in unused grouping.
    leaf one { type int; }
  }
  uses the-group;
}
`,
		errors: []string{
			`bad.yang:9:3: invalid config value: bad`,
			`bad.yang:13:3: duplicate key from bad.yang:20:5: bob`,
			`bad.yang:14:5: invalid config value: incorrect`,
			`bad.yang:17:7: unknown type: base:unknown`,
			`bad.yang:22:5: unknown group: the-beatles`,
			`bad.yang:24:3: duplicate key from bad.yang:27:5: one`,
		},
	},
	{
		name: "bad-augment.yang",
		in: `
module base {
  namespace "urn:mod";
  prefix "base";
  // augmentation of unknown element
  augment erewhon {
    leaf bob {
      type string;
      // bad config value in unused augment
      config wrong;
    }
  }
}
`,
		errors: []string{
			`bad-augment.yang:6:3: augment erewhon not found`,
		},
	},
}

func TestBadYang(t *testing.T) {
	for _, tt := range badInputs {
		typeDict = typeDictionary{dict: map[Node]map[string]*Typedef{}}
		ms := NewModules()
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		errs := ms.Process()
		if len(errs) != len(tt.errors) {
			t.Errorf("got %d errors, want %d", len(errs), len(tt.errors))
		} else {
			ok := true
			for x, err := range errs {
				if err.Error() != tt.errors[x] {
					ok = false
					break
				}
			}
			if ok {
				continue
			}
		}

		var b bytes.Buffer
		fmt.Fprint(&b, "got errors:\n")
		for _, err := range errs {
			fmt.Fprintf(&b, "\t%v\n", err)
		}
		fmt.Fprint(&b, "want errors:\n")
		for _, err := range tt.errors {
			fmt.Fprintf(&b, "\t%s\n", err)
		}
		t.Error(b.String())
	}
}

var parentTestModules = []struct {
	name string
	in   string
}{
	{
		name: "foo.yang",
		in: `
module foo {
  namespace "urn:foo";
  prefix "foo";

  import bar { prefix "temp-bar"; }
  container foo-c {
    leaf zzz { type string; }
    leaf-list foo-list { type string; }
    uses temp-bar:common;
  }
  uses temp-bar:common;
}
`,
	},
	{
		name: "bar.yang",
		in: `
module bar {
  namespace "urn:bar";
  prefix "bar";

  grouping common {
    container test1 { leaf str { type string; } }
    container test2 { leaf str { type string; } }
  }

  container bar-local {
    leaf test1 { type string; }
  }

}
`,
	},
	{
		name: "baz.yang",
		in: `
module baz {
  namespace "urn:baz";
  prefix "baz";

  import foo { prefix "f"; }

  grouping baz-common {
    leaf baz-common-leaf { type string; }
    container baz-dir {
      leaf aardvark { type string; }
    }
  }

  augment /f:foo-c {
    uses baz-common;
    leaf baz-direct-leaf { type string; }
  }
}
`,
	},
	{
		name: "baz-augment.yang",
		in: `
		submodule baz-augment {
		  belongs-to baz {
		    prefix "baz";
		  }

		  import foo { prefix "f"; }

		  augment "/f:foo-c" {
		    leaf baz-submod-leaf { type string; }
		  }
		}
		`,
	},
}

func TestUsesParent(t *testing.T) {
	ms := NewModules()
	for _, tt := range parentTestModules {
		_ = ms.Parse(tt.in, tt.name)
	}

	efoo, _ := ms.GetModule("foo")
	used := efoo.Dir["foo-c"].Dir["test1"]
	expected := "/foo/foo-c/test1"
	if used.Path() != expected {
		t.Errorf("want %s, got %s", expected, used.Path())
	}

	used = efoo.Dir["test1"]
	expected = "/foo/test1"
	if used.Path() != expected {
		t.Errorf("want %s, got %s", expected, used.Path())
	}
}

func TestPrefixes(t *testing.T) {
	ms := NewModules()
	for _, tt := range parentTestModules {
		_ = ms.Parse(tt.in, tt.name)
	}

	efoo, _ := ms.GetModule("foo")
	if efoo.Prefix.Name != "foo" {
		t.Errorf(`want prefix "foo", got %q`, efoo.Prefix.Name)
	}

	used := efoo.Dir["foo-c"].Dir["zzz"]
	if used.Prefix == nil || used.Prefix.Name != "foo" {
		t.Errorf(`want prefix named "foo", got %#v`, used.Prefix)
	}

	used = efoo.Dir["foo-c"].Dir["foo-list"]
	if used.Prefix == nil || used.Prefix.Name != "foo" {
		t.Errorf(`want prefix named "foo", got %#v`, used.Prefix)
	}
	used = efoo.Dir["foo-c"].Dir["test1"]
	if used.Prefix.Name != "bar" {
		t.Errorf(`want prefix "bar", got %q`, used.Prefix.Name)
	}

	used = efoo.Dir["foo-c"].Dir["test1"].Dir["str"]
	if used.Prefix == nil || used.Prefix.Name != "bar" {
		t.Errorf(`want prefix named "bar", got %#v`, used.Prefix)
	}

}

func TestEntryNamespace(t *testing.T) {
	ms := NewModules()
	for _, tt := range parentTestModules {
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("could not parse module %s: %v", tt.name, err)
		}
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	foo, _ := ms.GetModule("foo")
	bar, _ := ms.GetModule("bar")

	for _, tc := range []struct {
		descr   string
		entry   *Entry
		ns      string
		wantMod string
	}{
		{
			descr:   "grouping used in foo always have foo's namespace, even if it was defined in bar",
			entry:   foo.Dir["foo-c"].Dir["test1"],
			ns:      "urn:foo",
			wantMod: "foo",
		},
		{
			descr:   "grouping defined and used in foo has foo's namespace",
			entry:   foo.Dir["foo-c"].Dir["zzz"],
			ns:      "urn:foo",
			wantMod: "foo",
		},
		{
			descr:   "grouping defined and used in bar has bar's namespace",
			entry:   bar.Dir["bar-local"].Dir["test1"],
			ns:      "urn:bar",
			wantMod: "bar",
		},
		{
			descr:   "leaf within a used grouping in baz augmented into foo has baz's namespace",
			entry:   foo.Dir["foo-c"].Dir["baz-common-leaf"],
			ns:      "urn:baz",
			wantMod: "baz",
		},
		{
			descr:   "leaf directly defined within an augment to foo from baz has baz's namespace",
			entry:   foo.Dir["foo-c"].Dir["baz-direct-leaf"],
			ns:      "urn:baz",
			wantMod: "baz",
		},
		{
			descr:   "children of a container within an augment to from baz have baz's namespace",
			entry:   foo.Dir["foo-c"].Dir["baz-dir"].Dir["aardvark"],
			ns:      "urn:baz",
			wantMod: "baz",
		},
	} {
		nsValue := tc.entry.Namespace()
		if nsValue == nil {
			t.Errorf("%s: want namespace %s, got nil", tc.descr, tc.ns)
		} else if tc.ns != nsValue.Name {
			t.Errorf("%s: want namespace %s, got %s", tc.descr, tc.ns, nsValue.Name)
		}

		m, err := tc.entry.InstantiatingModule()
		if err != nil {
			t.Errorf("%s: %s.InstantiatingModule(): got unexpected error: %v", tc.descr, tc.entry.Path(), err)
			continue
		}

		if m != tc.wantMod {
			t.Errorf("%s: %s.InstantiatingModule(): did not get expected name, got: %v, want: %v",
				tc.descr, tc.entry.Path(), m, tc.wantMod)
		}
	}
}

func TestIgnoreCircularDependencies(t *testing.T) {
	tests := []struct {
		name            string
		inModules       map[string]string
		inIgnoreCircDep bool
		wantErrs        bool
	}{{
		name: "validation that non-circular dependencies are correct",
		inModules: map[string]string{
			"mod-a": `
			module mod-a {
				namespace "urn:a";
				prefix "a";

				include subm-x;
				include subm-y;

				leaf marker { type string; }
			}
			`,
			"subm-x": `
				submodule subm-x {
					belongs-to mod-a { prefix a; }
				}
			`,
			"subm-y": `
			submodule subm-y {
				belongs-to mod-a { prefix a; }
				// Not circular.
				include subm-x;
			}
      `},
	}, {
		name: "circular dependency error identified",
		inModules: map[string]string{
			"mod-a": `
    module mod-a {
      namespace "urn:a";
      prefix "a";

      include subm-x;
      include subm-y;

      leaf marker { type string; }
    }
    `,
			"subm-x": `
      submodule subm-x {
        belongs-to mod-a { prefix a; }
        // Circular
        include subm-y;
      }
    `,
			"subm-y": `
      submodule subm-y {
        belongs-to mod-a { prefix a; }
        // Circular
        include subm-x;
      }
    `},
		wantErrs: true,
	}, {
		name: "circular dependency error skipped",
		inModules: map[string]string{
			"mod-a": `
    module mod-a {
      namespace "urn:a";
      prefix "a";

      include subm-x;
      include subm-y;

      leaf marker { type string; }
    }
    `,
			"subm-x": `
      submodule subm-x {
        belongs-to mod-a { prefix a; }
        // Circular
        include subm-y;
      }
    `,
			"subm-y": `
      submodule subm-y {
        belongs-to mod-a { prefix a; }
        // Circular
        include subm-x;
      }
    `},
		inIgnoreCircDep: true,
	}}

	for _, tt := range tests {
		ms := NewModules()
		ParseOptions.IgnoreSubmoduleCircularDependencies = tt.inIgnoreCircDep
		for n, m := range tt.inModules {
			if err := ms.Parse(m, n); err != nil {
				if !tt.wantErrs {
					t.Errorf("%s: could not parse modules, got: %v, want: nil", tt.name, err)
				}
				continue
			}
		}
	}
}

func TestEntryDefaultValue(t *testing.T) {
	getdir := func(e *Entry, elements ...string) (*Entry, error) {
		for _, elem := range elements {
			next := e.Dir[elem]
			if next == nil {
				return nil, fmt.Errorf("%s missing directory %q", e.Path(), elem)
			}
			e = next
		}
		return e, nil
	}

	modtext := `
module defaults {
  namespace "urn:defaults";
  prefix "defaults";

  typedef string-default {
    type string;
    default "typedef default value";
  }

  grouping common {
    container common-nodefault {
      leaf string {
        type string;
      }
    }
    container common-withdefault {
      leaf string {
        type string;
        default "default value";
      }
    }
    container common-typedef-withdefault {
      leaf string {
        type string-default;
      }
    }
  }

  container defaults {
    leaf mandatory-default {
      type string-default;
      mandatory true;
    }
    leaf uint32-withdefault {
      type uint32;
      default 13;
    }
    leaf string-withdefault {
      type string-default;
    }
    leaf nodefault {
      type string;
    }
    uses common;
  }

}
`

	ms := NewModules()
	if err := ms.Parse(modtext, "defaults.yang"); err != nil {
		t.Fatal(err)
	}

	for i, tc := range []struct {
		want string
		path []string
	}{
		{
			path: []string{"defaults", "string-withdefault"},
			want: "typedef default value",
		},
		{
			path: []string{"defaults", "uint32-withdefault"},
			want: "13",
		},
		{
			path: []string{"defaults", "nodefault"},
			want: "",
		},
		{
			path: []string{"defaults", "common-withdefault", "string"},
			want: "default value",
		},
		{
			path: []string{"defaults", "common-typedef-withdefault", "string"},
			want: "typedef default value",
		},
		{
			path: []string{"defaults", "common-nodefault", "string"},
			want: "",
		},
		{
			path: []string{"defaults", "mandatory-default"},
			want: "",
		},
	} {
		tname := strings.Join(tc.path, "/")

		mod, err := ms.FindModuleByPrefix("defaults")
		if err != nil {
			t.Fatalf("[%d_%s] module not found: %v", i, tname, err)
		}
		defaults := ToEntry(mod)
		dir, err := getdir(defaults, tc.path...)
		if err != nil {
			t.Fatalf("[%d_%s] could not retrieve path: %v", i, tname, err)
		}
		if got := dir.DefaultValue(); tc.want != got {
			t.Errorf("[%d_%s] want DefaultValue %q, got %q", i, tname, tc.want, got)
		}
	}
}

func TestFullModuleProcess(t *testing.T) {
	tests := []struct {
		name             string
		inModules        map[string]string
		inIgnoreCircDeps bool
		wantLeaves       map[string][]string
		wantErr          bool
	}{{
		name: "circular import via child",
		inModules: map[string]string{
			"test": `
			      module test {
				      	prefix "t";
					namespace "urn:t";

				      	include test-router;
				   	include test-router-bgp;
				      	include test-router-isis;

				   	container configure {
						uses test-router;
					}
				}`,
			"test-router": `
				submodule test-router {
					belongs-to test { prefix "t"; }

					include test-router-bgp;
					include test-router-isis;
					include test-router-ldp;

					grouping test-router {
						uses test-router-ldp;
					}
				}`,
			"test-router-ldp": `
				submodule test-router-ldp {
					belongs-to test { prefix "t"; }

					grouping test-router-ldp { }
				}`,
			"test-router-isis": `
				 submodule test-router-isis {
					belongs-to test { prefix "t"; }

					include test-router;
				}`,
			"test-router-bgp": `
				submodule test-router-bgp {
					belongs-to test { prefix "t"; }
				}`,
		},
		inIgnoreCircDeps: true,
	}, {
		name: "non-circular via child",
		inModules: map[string]string{
			"bgp": `
			module bgp {
			  prefix "bgp";
			  namespace "urn:bgp";

			  include bgp-son;
			  include bgp-daughter;

			  leaf parent { type string; }
			}`,
			"bgp-son": `
			submodule bgp-son {
			  belongs-to bgp { prefix "bgp"; }

			  leaf son { type string; }
			}`,
			"bgp-daughter": `
			submodule bgp-daughter {
			  belongs-to bgp { prefix "bgp"; }
			  include bgp-son;

			  leaf daughter { type string; }
			}`,
		},
	}, {
		name: "simple circular via child",
		inModules: map[string]string{
			"parent": `
			module parent {
			  prefix "p";
			  namespace "urn:p";
				include son;
				include daughter;

			  leaf p { type string; }
			}
			`,
			"son": `
			submodule son {
			  belongs-to parent { prefix "p"; }
			  include daughter;

			  leaf s { type string; }
			}
			`,
			"daughter": `
			submodule daughter {
			  belongs-to parent { prefix "p"; }
			  include son;

			  leaf d { type string; }
			}
			`,
		},
		wantErr: true,
	}, {
		name: "merge via grandchild",
		inModules: map[string]string{
			"bgp": `
				module bgp {
				  prefix "bgp";
				  namespace "urn:bgp";

				  include bgp-son;

				  leaf parent { type string; }
				}`,
			"bgp-son": `
				submodule bgp-son {
				  belongs-to bgp { prefix "bgp"; }

					include bgp-grandson;

				  leaf son { type string; }
				}`,
			"bgp-grandson": `
				submodule bgp-grandson {
				  belongs-to bgp { prefix "bgp"; }

				  leaf grandson { type string; }
				}`,
		},
		wantLeaves: map[string][]string{
			"bgp": {"parent", "son", "grandson"},
		},
	}, {
		name: "parent to son and daughter with common grandchild",
		inModules: map[string]string{
			"parent": `
			module parent {
				prefix "p";
				namespace "urn:p";
				include son;
				include daughter;

				leaf p { type string; }
			}
			`,
			"son": `
			submodule son {
				belongs-to parent { prefix "p"; }
				include grandchild;

				leaf s { type string; }
			}
			`,
			"daughter": `
			submodule daughter {
				belongs-to parent { prefix "p"; }
				include grandchild;

				leaf d { type string; }
			}
			`,
			"grandchild": `
			submodule grandchild {
				belongs-to parent { prefix "p"; }

				leaf g { type string; }
			}
			`,
		},
		wantLeaves: map[string][]string{
			"parent": {"p", "s", "d", "g"},
		},
	}, {
		name: "parent to son and daughter, not a circdep",
		inModules: map[string]string{
			"parent": `
			module parent {
				prefix "p";
				namespace "urn:p";

				include son;
				include daughter;

				uses son-group;
			}
			`,
			"son": `
			submodule son {
				belongs-to parent { prefix "p"; }
				include daughter;

				grouping son-group {
					uses daughter-group;
				}
			}
			`,
			"daughter": `
			submodule daughter {
				belongs-to parent { prefix "p"; }

				grouping daughter-group {
					leaf s { type string; }
				}

				leaf d { type string; }
			}
			`,
		},
		wantLeaves: map[string][]string{
			"parent": {"s", "d"},
		},
	}}

	for _, tt := range tests {
		ms := NewModules()
		mergedSubmodule = map[string]bool{}

		ParseOptions.IgnoreSubmoduleCircularDependencies = tt.inIgnoreCircDeps
		for n, m := range tt.inModules {
			if err := ms.Parse(m, n); err != nil {
				t.Errorf("%s: error parsing module %s, got: %v, want: nil", tt.name, n, err)
			}
		}

		if errs := ms.Process(); len(errs) > 0 {
			if !tt.wantErr {
				t.Errorf("%s: error processing modules, got: %v, want: nil", tt.name, errs)
			}
			continue
		}

		if tt.wantErr {
			t.Errorf("%s: did not get expected errors", tt.name)
			continue
		}

		for m, l := range tt.wantLeaves {
			mod, errs := ms.GetModule(m)
			if len(errs) > 0 {
				t.Errorf("%s: cannot retrieve expected module %s, got: %v, want: nil", tt.name, m, errs)
				continue
			}

			var leaves []string
			for _, n := range mod.Dir {
				leaves = append(leaves, n.Name)
			}

			// Sort the two slices to ensure that we are comparing like with like.
			sort.Strings(l)
			sort.Strings(leaves)
			if !reflect.DeepEqual(l, leaves) {
				t.Errorf("%s: did not get expected leaves in %s, got: %v, want: %v", tt.name, m, leaves, l)
			}
		}
	}
}

func TestAnyDataAnyXML(t *testing.T) {
	tests := []struct {
		name          string
		inModule      string
		wantNodeKind  string
		wantEntryKind EntryKind
	}{
		{
			name:          "test anyxml",
			wantNodeKind:  "anyxml",
			wantEntryKind: AnyXMLEntry,
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  container c {
    anyxml data {
      description "anyxml";
    }
  }
}`,
		},
		{
			name:          "test anydata",
			wantNodeKind:  "anydata",
			wantEntryKind: AnyDataEntry,
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  container c {
    anydata data {
      description "anydata";
    }
  }
}`,
		},
	}
	for _, tt := range tests {
		ms := NewModules()
		if err := ms.Parse(tt.inModule, "test"); err != nil {
			t.Errorf("%s: error parsing module 'test', got: %v, want: nil", tt.name, err)
		}

		if errs := ms.Process(); len(errs) > 0 {
			t.Errorf("%s: got module parsing errors", tt.name)
			for i, err := range errs {
				t.Errorf("%s: error #%d: %v", tt.name, i, err)
			}
			continue
		}

		mod, ok := ms.Modules["test"]
		if !ok {
			t.Errorf("%s: did not find `test` module", tt.name)
			continue
		}
		e := ToEntry(mod)
		c := e.Dir["c"]
		if c == nil {
			t.Errorf("%s: did not find container c", tt.name)
			continue
		}
		data := c.Dir["data"]
		if data == nil {
			t.Errorf("%s: did not find leaf c/data", tt.name)
			continue
		}
		if got := data.Node.Kind(); got != tt.wantNodeKind {
			t.Errorf("%s: want Node.Kind(): %q, got: %q", tt.name, tt.wantNodeKind, got)
		}
		if got := data.Kind; got != tt.wantEntryKind {
			t.Errorf("%s: want Kind: %v, got: %v", tt.name, tt.wantEntryKind, got)
		}
		if got := data.Description; got != tt.wantNodeKind {
			t.Errorf("%s: want data.Description: %q, got: %q", tt.name, tt.wantNodeKind, got)
		}
	}
}

func getEntry(root *Entry, path []string) *Entry {
	for _, elem := range path {
		if root = root.Dir[elem]; root == nil {
			break
		}
	}
	return root
}

func TestActionRPC(t *testing.T) {
	tests := []struct {
		name          string
		inModule      string
		operationPath []string
		wantNodeKind  string
		wantError     string
	}{
		{
			name:          "test action in container",
			wantNodeKind:  "action",
			operationPath: []string{"c", "operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  container c {
    action operation {
      description "action";
      input { leaf string { type string; } }
      output { leaf string { type string; } }
    }
  }
}`,
		},

		{
			name:          "test action in list",
			wantNodeKind:  "action",
			operationPath: []string{"list", "operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  list list {
    action operation {
      description "action";
      input { leaf string { type string; } }
      output { leaf string { type string; } }
    }
  }
}`,
		},

		{
			name:          "test action in container via grouping",
			wantNodeKind:  "action",
			operationPath: []string{"c", "operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  grouping g {
    action operation {
      description "action";
      input { leaf string { type string; } }
      output { leaf string { type string; } }
    }
  }
  container c { uses g; }
}`,
		},

		{
			name:          "test action in list via grouping",
			wantNodeKind:  "action",
			operationPath: []string{"list", "operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  grouping g {
    action operation {
      description "action";
      input { leaf string { type string; } }
      output { leaf string { type string; } }
    }
  }
  list list { uses g; }
}`,
		},

		{
			name:          "test rpc",
			wantNodeKind:  "rpc",
			operationPath: []string{"operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  rpc operation {
    description "rpc";
    input {
      leaf string { type string; }
    }
    output {
      leaf string { type string; }
    }
  }
}`,
		},

		// test cases with errors (in module parsing)
		{
			name:      "rpc not module child",
			wantError: "test:6:5: unknown container field: rpc",
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  container c {
    // error: "rpc" is not a valid sub-statement to "container"
    rpc operation;
  }
}`,
		},

		{
			name:      "action not valid leaf child",
			wantError: "test:6:5: unknown leaf field: action",
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  leaf l {
    // error: "operation" is not a valid sub-statement to "leaf"
    action operation;
  }
}`,
		},

		{
			name:      "action not valid leaf-list child",
			wantError: "test:6:5: unknown leaf-list field: action",
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  leaf-list leaf-list {
    // error: "operation" is not a valid sub-statement to "leaf-list"
    action operation;
  }
}`,
		},
	}
	for _, tt := range tests {
		ms := NewModules()
		if err := ms.Parse(tt.inModule, "test"); err != nil {
			if got := err.Error(); got != tt.wantError {
				t.Errorf("%s: error parsing module 'test', got error: %q, want: %q", tt.name, got, tt.wantError)
			}
			continue
		}

		if errs := ms.Process(); len(errs) > 0 {
			t.Errorf("%s: got %d module parsing errors", tt.name, len(errs))
			for i, err := range errs {
				t.Errorf("%s: error #%d: %v", tt.name, i, err)
			}
			continue
		}

		mod := ms.Modules["test"]
		e := ToEntry(mod)
		if e = getEntry(e, tt.operationPath); e == nil {
			t.Errorf("%s: want child entry at: %v, got: nil", tt.name, tt.operationPath)
			continue
		}
		if got := e.Node.Kind(); got != tt.wantNodeKind {
			t.Errorf("%s: got `operation` node kind: %q, want: %q", tt.name, got, tt.wantNodeKind)
		} else if got := e.Description; got != tt.wantNodeKind {
			t.Errorf("%s: got `operation` Description: %q, want: %q", tt.name, got, tt.wantNodeKind)
		}
		// confirm the child RPCEntry was populated for the entry.
		if e.RPC == nil {
			t.Errorf("%s: entry at %v has nil RPC child, want: non-nil. Entry: %#v", tt.name, tt.operationPath, e)
		} else if e.RPC.Input == nil {
			t.Errorf("%s: RPCEntry has nil Input, want: non-nil. Entry: %#v", tt.name, e.RPC)
		} else if e.RPC.Output == nil {
			t.Errorf("%s: RPCEntry has nil Output, want: non-nil. Entry: %#v", tt.name, e.RPC)
		}
	}
}

// addTreeE takes an input Entry and appends it to a directory, keyed by path, to the Entry.
// If the Entry has children, they are appended to the directory recursively. Used in test
// cases where a path is to be referred to.
func addTreeE(e *Entry, dir map[string]*Entry) {
	for _, ch := range e.Dir {
		dir[ch.Path()] = ch
		if ch.Dir != nil {
			addTreeE(ch, dir)
		}
	}
}

func TestEntryFind(t *testing.T) {
	tests := []struct {
		name            string
		inModules       map[string]string
		inBaseEntryPath string
		wantEntryPath   map[string]string // keyed on path to find, with path expected as value.
		wantError       string
	}{{
		name: "intra module find",
		inModules: map[string]string{
			"test.yang": `
				module test {
					prefix "t";
					namespace "urn:t";

					leaf a { type string; }
					leaf b { type string; }

					container c { leaf d { type string; } }

                    rpc rpc1 {
                        input { leaf input1 { type string; } }
                    }

                    container e {
                        action operation {
                          description "action";
                          input { leaf input1 { type string; } }
                          output { leaf output1 { type string; } }
                        }
                    }

				}
			`,
		},
		inBaseEntryPath: "/test/a",
		wantEntryPath: map[string]string{
			// Absolute path with no prefixes.
			"/b": "/test/b",
			// Relative path with no prefixes.
			"../b": "/test/b",
			// Absolute path with prefixes.
			"/t:b": "/test/b",
			// Relative path with prefixes.
			"../t:b": "/test/b",
			// Find within a directory.
			"/c/d": "/test/c/d",
			// Find within a directory specified relatively.
			"../c/d": "/test/c/d",
			// Find within a relative directory with prefixes.
			"../t:c/t:d": "/test/c/d",
			"../t:c/d":   "/test/c/d",
			"../c/t:d":   "/test/c/d",
			// Find within an absolute directory with prefixes.
			"/t:c/d":                "/test/c/d",
			"/c/t:d":                "/test/c/d",
			"../t:rpc1/input":       "/test/rpc1/input",
			"/t:rpc1/input":         "/test/rpc1/input",
			"/t:e/operation/input":  "/test/e/operation/input",
			"/t:e/operation/output": "/test/e/operation/output",
		},
	}, {
		name: "inter-module find",
		inModules: map[string]string{
			"test.yang": `
				module test {
					prefix "t";
					namespace "urn:t";

					import foo { prefix foo; }
					import bar { prefix baz; }

					leaf ctx { type string; }
					leaf other { type string; }
					leaf conflict { type string; }
				}`,
			"foo.yang": `
				module foo {
					prefix "foo"; // matches the import above
					namespace "urn:foo";

					container bar {
						leaf baz { type string; }
					}

					leaf conflict { type string; }
				}`,
			"bar.yang": `
				module bar {
					prefix "bar"; // does not match import in test
					namespace "urn:b";

					container fish {
						leaf chips { type string; }
					}

					leaf conflict { type string; }
				}`,
		},
		inBaseEntryPath: "/test/ctx",
		wantEntryPath: map[string]string{
			// Check we can still do intra module lookups
			"../other":         "/test/other",
			"/other":           "/test/other",
			"/foo:bar/foo:baz": "/foo/bar/baz",
			// Technically partially prefixed paths to remote modules are
			// not legal - check whether we can resolve them.
			"/foo:bar/baz": "/foo/bar/baz",
			// With mismatched prefixes.
			"/baz:fish/baz:chips": "/bar/fish/chips",
			// With conflicting node names
			"/conflict":     "/test/conflict",
			"/foo:conflict": "/foo/conflict",
			"/baz:conflict": "/bar/conflict",
			"/t:conflict":   "/test/conflict",
		},
	}}

	for _, tt := range tests {
		ms := NewModules()
		var errs []error
		for n, m := range tt.inModules {
			if err := ms.Parse(m, n); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			t.Errorf("%s: ms.Parse(), got unexpected error parsing input modules: %v", tt.name, errs)
			continue
		}

		if errs := ms.Process(); len(errs) > 0 {
			t.Errorf("%s: ms.Process(), got unexpected error processing entries: %v", tt.name, errs)
			continue
		}

		dir := map[string]*Entry{}
		for _, m := range ms.Modules {
			addTreeE(ToEntry(m), dir)
		}

		if _, ok := dir[tt.inBaseEntryPath]; !ok {
			t.Errorf("%s: could not find entry %s within the dir: %v", tt.name, tt.inBaseEntryPath, dir)
		}

		for path, want := range tt.wantEntryPath {
			got := dir[tt.inBaseEntryPath].Find(path)
			if got.Path() != want {
				t.Errorf("%s: (entry %s).Find(%s), did not find path, got: %v, want: %v, errors: %v", tt.name, dir[tt.inBaseEntryPath].Path(), path, got.Path(), want, dir[tt.inBaseEntryPath].Errors)
			}
		}
	}
}

func TestEntryTypes(t *testing.T) {
	leafSchema := &Entry{Name: "leaf-schema", Kind: LeafEntry, Type: &YangType{Kind: Ystring}}

	containerSchema := &Entry{
		Name: "container-schema",
		Kind: DirectoryEntry,
		Dir: map[string]*Entry{
			"config": {
				Dir: map[string]*Entry{
					"leaf1": {
						Kind: LeafEntry,
						Name: "Leaf1Name",
						Type: &YangType{Kind: Ystring},
					},
				},
			},
		},
	}

	emptyContainerSchema := &Entry{
		Name: "empty-container-schema",
		Kind: DirectoryEntry,
	}

	leafListSchema := &Entry{
		Kind:     LeafEntry,
		ListAttr: &ListAttr{MinElements: &Value{Name: "0"}},
		Type:     &YangType{Kind: Ystring},
		Name:     "leaf-list-schema",
	}

	listSchema := &Entry{
		Name:     "list-schema",
		Kind:     DirectoryEntry,
		ListAttr: &ListAttr{MinElements: &Value{Name: "0"}},
		Dir: map[string]*Entry{
			"leaf-name": {
				Kind: LeafEntry,
				Name: "LeafName",
				Type: &YangType{Kind: Ystring},
			},
		},
	}

	choiceSchema := &Entry{
		Kind: ChoiceEntry,
		Name: "Choice1Name",
		Dir: map[string]*Entry{
			"case1": {
				Kind: CaseEntry,
				Name: "case1",
				Dir: map[string]*Entry{
					"case1-leaf1": &Entry{
						Kind: LeafEntry,
						Name: "Case1Leaf1",
						Type: &YangType{Kind: Ystring},
					},
				},
			},
		},
	}

	type SchemaType string
	const (
		Leaf      SchemaType = "Leaf"
		Container SchemaType = "Container"
		LeafList  SchemaType = "LeafList"
		List      SchemaType = "List"
		Choice    SchemaType = "Choice"
		Case      SchemaType = "Case"
	)

	tests := []struct {
		desc     string
		schema   *Entry
		wantType SchemaType
	}{
		{
			desc:     "leaf",
			schema:   leafSchema,
			wantType: Leaf,
		},
		{
			desc:     "container",
			schema:   containerSchema,
			wantType: Container,
		},
		{
			desc:     "empty container",
			schema:   emptyContainerSchema,
			wantType: Container,
		},
		{
			desc:     "leaf-list",
			schema:   leafListSchema,
			wantType: LeafList,
		},
		{
			desc:     "list",
			schema:   listSchema,
			wantType: List,
		},
		{
			desc:     "choice",
			schema:   choiceSchema,
			wantType: Choice,
		},
		{
			desc:     "case",
			schema:   choiceSchema.Dir["case1"],
			wantType: Case,
		},
	}

	for _, tt := range tests {
		gotm := map[SchemaType]bool{
			Leaf:      tt.schema.IsLeaf(),
			Container: tt.schema.IsContainer(),
			LeafList:  tt.schema.IsLeafList(),
			List:      tt.schema.IsList(),
			Choice:    tt.schema.IsChoice(),
			Case:      tt.schema.IsCase(),
		}

		for stype, got := range gotm {
			if want := (stype == tt.wantType); got != want {
				t.Errorf("%s: got Is%v? %t, want Is%v? %t", tt.desc, stype, got, stype, want)
			}
		}
	}
}

func mustReadFile(path string) string {
	s, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(s)
}

func TestDeviation(t *testing.T) {
	type deviationTest struct {
		path  string
		entry *Entry // entry is the entry that is wanted at a particular path, if a field is left as nil, it is not checked.
	}
	tests := []struct {
		desc                    string
		inFiles                 map[string]string
		wants                   map[string][]deviationTest
		wantParseErrSubstring   string
		wantProcessErrSubstring string
	}{{
		desc:    "deviation with add",
		inFiles: map[string]string{"deviate": mustReadFile(filepath.Join("testdata", "deviate.yang"))},
		wants: map[string][]deviationTest{
			"deviate": []deviationTest{{
				path: "/target/add/config",
				entry: &Entry{
					Config: TSFalse,
				},
			}, {
				path: "/target/add/mandatory",
				entry: &Entry{
					Mandatory: TSTrue,
				},
			}, {
				path: "/target/add/min-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MinElements: &Value{Name: "42"},
					},
				},
			}, {
				path: "/target/add/max-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MaxElements: &Value{Name: "42"},
					},
				},
			}, {
				path: "/target/add/max-and-min-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MinElements: &Value{Name: "42"},
						MaxElements: &Value{Name: "42"},
					},
				},
			}, {
				path: "/target/add/units",
				entry: &Entry{
					Units: "fish per second",
				},
			}},
		},
	}, {
		desc: "error case - deviation add max-element to non-list",
		inFiles: map[string]string{
			"deviate": `
				module deviate {
					prefix "d";
					namespace "urn:d";

					leaf a { type string; }

					deviation /a {
						deviate add {
							max-elements 42;
						}
					}
				}`,
		},
		wantProcessErrSubstring: "tried to deviate max-elements on a non-list type",
	}, {
		desc: "error case - deviation add min elements to non-list",
		inFiles: map[string]string{
			"deviate": `
				module deviate {
					prefix "d";
					namespace "urn:d";

					leaf a { type string; }

					deviation /a {
						deviate add {
							min-elements 42;
						}
					}
				}`,
		},
		wantProcessErrSubstring: "tried to deviate min-elements on a non-list type",
	}, {
		desc:    "deviation - not supported",
		inFiles: map[string]string{"deviate": mustReadFile(filepath.Join("testdata", "deviate-notsupported.yang"))},
		wants: map[string][]deviationTest{
			"deviate": []deviationTest{{
				path: "/target",
			}, {
				path: "/target-list",
			}, {
				path: "/a-leaf",
			}, {
				path: "/a-leaflist",
			}, {
				path:  "survivor",
				entry: &Entry{Name: "survivor"},
			}},
		},
	}, {
		desc: "deviation removing non-existent node",
		inFiles: map[string]string{
			"deviate": `
				module deviate {
					prefix "d";
					namespace "urn:d";

					deviation /a/b/c {
						deviate not-supported;
					}
				}
			`,
		},
		wantProcessErrSubstring: "cannot find target node to deviate",
	}, {
		desc: "deviation not supported across modules",
		inFiles: map[string]string{
			"source": `
				module source {
					prefix "s";
					namespace "urn:s";

					leaf a { type string; }
					leaf b { type string; }
				}`,
			"deviation": `
					module deviation {
						prefix "d";
						namespace "urn:d";

						import source { prefix s; }

						deviation /s:a {
							deviate not-supported;
						}
					}`,
		},
		wants: map[string][]deviationTest{
			"source": []deviationTest{{
				path: "/a",
			}, {
				path:  "/b",
				entry: &Entry{},
			}},
		},
	}, {
		desc:    "deviation with replace",
		inFiles: map[string]string{"deviate": mustReadFile(filepath.Join("testdata", "deviate-replace.yang"))},
		wants: map[string][]deviationTest{
			"deviate": []deviationTest{{
				path: "/target/replace/config",
				entry: &Entry{
					Config: TSFalse,
				},
			}, {
				path: "/target/replace/mandatory",
				entry: &Entry{
					Mandatory: TSTrue,
				},
			}, {
				path: "/target/replace/min-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MinElements: &Value{Name: "42"},
					},
				},
			}, {
				path: "/target/replace/max-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MaxElements: &Value{Name: "42"},
					},
				},
			}, {
				path: "/target/replace/max-and-min-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MinElements: &Value{Name: "42"},
						MaxElements: &Value{Name: "42"},
					},
				},
			}, {
				path: "/target/replace/units",
				entry: &Entry{
					Units: "fish per second",
				},
			}, {
				path: "/target/replace/type",
				entry: &Entry{
					Type: &YangType{
						Name: "uint16",
						Kind: Yuint16,
					},
				},
			}},
		},
	}, {
		desc:    "deviation with delete",
		inFiles: map[string]string{"deviate": mustReadFile(filepath.Join("testdata", "deviate-delete.yang"))},
		wants: map[string][]deviationTest{
			"deviate": []deviationTest{{
				path: "/target/delete/config",
				entry: &Entry{
					Config: TSUnset,
				},
			}, {
				path: "/target/delete/mandatory",
				entry: &Entry{
					Mandatory: TSUnset,
				},
			}, {
				path: "/target/delete/min-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MinElements: nil,
					},
				},
			}, {
				path: "/target/delete/max-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MaxElements: nil,
					},
				},
			}, {
				path: "/target/delete/max-and-min-elements",
				entry: &Entry{
					ListAttr: &ListAttr{
						MinElements: nil,
						MaxElements: nil,
					},
				},
			}, {
				path: "/target/delete/units",
				entry: &Entry{
					Units: "",
				},
			}},
		},
	}, {
		desc: "deviation using locally defined typedef",
		inFiles: map[string]string{
			"deviate": `
				module deviate {
					prefix "d";
					namespace "urn:d";

					import source { prefix s; }

					typedef rstr {
						type string {
							pattern "a.*";
						}
					}

					deviation /s:a {
						deviate replace {
							type rstr;
						}
					}
				}
			`,
			"source": `
				module source {
					prefix "s";
					namespace "urn:s";

					leaf a { type uint16; }
				}
			`,
		},
		wants: map[string][]deviationTest{
			"source": []deviationTest{{
				path: "/a",
				entry: &Entry{
					Type: &YangType{
						Name:    "rstr",
						Kind:    Ystring,
						Pattern: []string{"a.*"},
					},
				},
			}},
		},
	}, {
		desc: "complex deviation of multiple leaves",
		inFiles: map[string]string{
			"foo": `
			module foo {
				prefix "f";
				namespace "urn:f";

				container a { leaf b { type string; } }

				typedef abc { type boolean; }
				typedef abt { type uint32; }

				deviation /a/b {
					// typedef is not valid here.
					//typedef abc {
					//  type boolean;
					//}
					deviate replace { type abc; }
				}

				deviation /a/b {
					// typedef is not valid here.
					//typedef abt {
					//  type uint16;
					//}
					deviate replace { type abt; }
				}
			}`,
		},
		wants: map[string][]deviationTest{
			"foo": []deviationTest{{
				path: "/a/b",
				entry: &Entry{
					Type: &YangType{
						Name: "abt",
						Kind: Yuint32,
					},
				},
			}},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()
			mergedSubmodule = map[string]bool{}

			for name, mod := range tt.inFiles {
				if err := ms.Parse(mod, name); err != nil {
					if diff := errdiff.Substring(err, tt.wantParseErrSubstring); diff != "" {
						t.Fatalf("error parsing module %s, %s", name, diff)
					}
				}
			}

			if errs := ms.Process(); len(errs) > 0 {
				var match bool
				for _, err := range errs {
					if diff := errdiff.Substring(err, tt.wantProcessErrSubstring); diff == "" {
						match = true
						break
					}
				}
				if !match {
					t.Fatalf("got errs: %v, want: %v", errs, tt.wantProcessErrSubstring)
				}
			}

			for mod, tcs := range tt.wants {
				m, errs := ms.GetModule(mod)
				if errs != nil {
					t.Errorf("couldn't find module %s", mod)
					continue
				}

				for idx, want := range tcs {
					got := m.Find(want.path)
					switch {
					case got == nil && want.entry != nil:
						t.Errorf("%d: expected entry %s does not exist", idx, want.path)
						continue
					case got != nil && want.entry == nil:
						t.Errorf("%d: unexpected entry %s exists, got: %v", idx, want.path, got)
						continue
					case want.entry == nil:
						continue
					}

					if got.Config != want.entry.Config {
						t.Errorf("%d (%s): did not get expected config statement, got: %v, want: %v", idx, want.path, got.Config, want.entry.Config)
					}

					if got.Default != want.entry.Default {
						t.Errorf("%d (%s): did not get expected default statement, got: %v, want: %v", idx, want.path, got.Default, want.entry.Default)
					}

					if got.Mandatory != want.entry.Mandatory {
						t.Errorf("%d (%s): did not get expected mandatory statement, got: %v, want: %v", idx, want.path, got.Mandatory, want.entry.Mandatory)
					}

					if want.entry.ListAttr != nil {
						if got.ListAttr == nil {
							t.Errorf("%d (%s): listattr was nil for an entry expected to be a list at %s", idx, want.path, want.path)
							continue
						}
						if want.entry.ListAttr.MinElements != nil {
							if gotn, wantn := got.ListAttr.MinElements.Name, want.entry.ListAttr.MinElements.Name; gotn != wantn {
								t.Errorf("%d (%s): min-elements, got: %v, want: %v", idx, want.path, gotn, wantn)
							}
						}
					}

					if want.entry.Type != nil {
						if got.Type.Name != want.entry.Type.Name {
							t.Errorf("%d (%s): type name, got: %s, want: %s", idx, want.path, got.Type.Name, want.entry.Type.Name)
						}

						if got.Type.Kind != want.entry.Type.Kind {
							t.Errorf("%d (%s): type kind, got: %s, want: %s", idx, want.path, got.Type.Kind, want.entry.Type.Kind)
						}
					}

					if got.Units != want.entry.Units {
						t.Errorf("%d (%s): did not get expected units statement, got: %s, want: %s", idx, want.path, got.Units, want.entry.Units)
					}
				}
			}
		})
	}
}
