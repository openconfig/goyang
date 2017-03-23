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
	"reflect"
	"sort"
	"strings"
	"testing"
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
	used := efoo.Dir["foo-c"].Dir["test1"]
	if used.Prefix.Name != "bar" {
		t.Errorf(`want prefix "bar", got %q`, used.Prefix.Name)
	}
}

func TestEntryNamespace(t *testing.T) {
	ms := NewModules()
	for _, tt := range parentTestModules {
		_ = ms.Parse(tt.in, tt.name)
	}

	foo, _ := ms.GetModule("foo")
	bar, _ := ms.GetModule("bar")

	for _, tc := range []struct {
		entry *Entry
		ns    string
	}{
		// grouping used in foo always have foo's namespace, even if
		// it was defined in bar as here.
		{
			entry: foo.Dir["foo-c"].Dir["test1"],
			ns:    "urn:foo",
		},

		// grouping defined and used in foo has foo's namespace
		{
			entry: foo.Dir["foo-c"].Dir["zzz"],
			ns:    "urn:foo",
		},

		// grouping defined and used in bar has bar's namespace
		{
			entry: bar.Dir["bar-local"].Dir["test1"],
			ns:    "urn:bar",
		},
	} {
		nsValue := tc.entry.Namespace()
		if nsValue == nil {
			t.Errorf("want namespace %s, got nil", tc.ns)
		} else if tc.ns != nsValue.Name {
			t.Errorf("want namespace %s, got %s", tc.ns, nsValue.Name)
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
