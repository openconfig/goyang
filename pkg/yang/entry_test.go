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
