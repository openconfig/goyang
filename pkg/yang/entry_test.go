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
