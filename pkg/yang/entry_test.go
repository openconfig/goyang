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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	default:
		t.Errorf("got %d errors, wanted 1", len(errs))
		fallthrough
	case 1:
		got := errs[0].Error()
		want := "ToEntry called with nil"
		if got != want {
			t.Fatalf("got error %q, want %q", got, want)
		}
	case 0:
		t.Fatalf("GetErrors returned no error")
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

var testWhenModules = []struct {
	name string
	in   string
}{
	{
		name: "when.yang",
		in: `
module when {
  namespace "urn:when";
  prefix "when";

  leaf condition { type string; }

  container alpha {
    when "../condition = 'alpha'";
  }

  leaf beta {
    when "../condition = 'beta'";
    type string;
  }

  leaf-list gamma {
    when "../condition = 'gamma'";
    type string;
  }

  list delta {
    when "../condition = 'delta'";
  }

  choice epsilon {
    when "../condition = 'epsilon'";

    case zeta {
      when "../condition = 'zeta'";
    }
  }

  anyxml eta {
    when "../condition = 'eta'";
  }

  anydata theta {
    when "../condition = 'theta'";
  }

  uses iota {
    when "../condition = 'iota'";
  }

  grouping iota {
  }

  augment "../alpha" {
    when "../condition = 'kappa'";
  }
}
`,
	},
}

func TestGetWhenXPath(t *testing.T) {
	ParseOptions.StoreUses = true
	defer func() { ParseOptions.StoreUses = false }()

	ms := NewModules()
	for _, tt := range testWhenModules {
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("could not parse module %s: %v", tt.name, err)
		}
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	when, _ := ms.GetModule("when")

	testcases := []struct {
		descr         string
		childName     string
		isCase        bool
		choiceName    string
		isAugment     bool
		augmentTarget string
	}{
		{
			descr:     "extract when statement from *Container",
			childName: "alpha",
		}, {
			descr:     "extract when statement from *Leaf",
			childName: "beta",
		}, {
			descr:     "extract when statement from *LeafList",
			childName: "gamma",
		}, {
			descr:     "extract when statement from *List",
			childName: "delta",
		}, {
			descr:     "extract when statement from *Choice",
			childName: "epsilon",
		}, {
			descr:      "extract when statement from *Case",
			childName:  "zeta",
			isCase:     true,
			choiceName: "epsilon",
		}, {
			descr:     "extract when statement from *AnyXML",
			childName: "eta",
		}, {
			descr:     "extract when statement from *AnyData",
			childName: "theta",
		}, {
			descr:         "extract when statement from *Augment",
			childName:     "kappa",
			isAugment:     true,
			augmentTarget: "alpha",
		},
	}

	for _, tc := range testcases {
		parentEntry := when
		t.Run(tc.descr, func(t *testing.T) {
			var child *Entry

			if tc.isAugment {
				child = parentEntry.Dir[tc.augmentTarget].Augmented[0]
			} else {
				if tc.isCase {
					parentEntry = parentEntry.Dir[tc.choiceName]
				}
				child = parentEntry.Dir[tc.childName]
			}

			expectedWhen := "../condition = '" + tc.childName + "'"

			if gotWhen, ok := child.GetWhenXPath(); !ok {
				t.Errorf("Cannot get when statement of child entry %v", tc.childName)
			} else if gotWhen != expectedWhen {
				t.Errorf("Expected when XPath %v, but got %v", expectedWhen, gotWhen)
			}
		})
	}
}

var testAugmentAndUsesModules = []struct {
	name string
	in   string
}{
	{
		name: "original.yang",
		in: `
module original {
  namespace "urn:original";
  prefix "orig";

  import groupings {
    prefix grp;
  }

  container alpha {
    leaf beta {
      type string;
    }
    leaf psi {
      type string;
    }
    leaf omega {
      type string;
    }
    uses grp:nestedLevel0 {
      when "beta = 'holaWorld'";
    }
  }
}
`,
	},
	{
		name: "augments.yang",
		in: `
module augments {
  namespace "urn:augments";
  prefix "aug";

  import original {
    prefix orig;
  }

  import groupings {
    prefix grp;
  }

  augment "/orig:alpha" {
    when "orig:beta = 'helloWorld'";

    container charlie {
      leaf charlieLeaf {
        type string;
      }
    }
  }

  grouping delta {
    container echo {
      leaf echoLeaf {
        type string;
      }
    }
  }

  augment "/orig:alpha" {
    when "orig:omega = 'privetWorld'";
    uses delta {
      when "current()/orig:beta = 'nihaoWorld'";
    }
  }
}
`,
	},
	{
		name: "groupings.yang",
		in: `
module groupings {
  namespace "urn:groupings";
  prefix "grp";

  import "original" {
    prefix orig;
  }

  grouping nestedLevel0 {
    leaf leafAtLevel0 {
      type string;
    }
    uses nestedLevel1 {
      when "orig:psi = 'geiasouWorld'";
    }
  }

  grouping nestedLevel1 {
    leaf leafAtLevel1 {
      type string;
    }
    uses nestedLevel2 {
      when "orig:omega = 'salveWorld'";
    }
  }

  grouping nestedLevel2 {
    leaf leafAtLevel2 {
      type string;
    }
  }
}
`,
	},
}

func TestAugmentedEntry(t *testing.T) {
	ms := NewModules()
	for _, tt := range testAugmentAndUsesModules {
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("could not parse module %s: %v", tt.name, err)
		}
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	orig, _ := ms.GetModule("original")

	testcases := []struct {
		descr             string
		augmentEntry      *Entry
		augmentWhenStmt   string
		augmentChildNames map[string]bool
	}{
		{
			descr:           "leaf charlie is augmented to container alpha",
			augmentEntry:    orig.Dir["alpha"].Augmented[0],
			augmentWhenStmt: "orig:beta = 'helloWorld'",
			augmentChildNames: map[string]bool{
				"charlie": false,
			},
		}, {
			descr:           "grouping delta is augmented to container alpha",
			augmentEntry:    orig.Dir["alpha"].Augmented[1],
			augmentWhenStmt: "orig:omega = 'privetWorld'",
			augmentChildNames: map[string]bool{
				"echo": false,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.descr, func(t *testing.T) {
			augment := tc.augmentEntry

			if tc.augmentWhenStmt != "" {
				if gotAugmentWhenStmt, ok := augment.GetWhenXPath(); !ok {
					t.Errorf("Expected augment when statement %v, but not present",
						tc.augmentWhenStmt)
				} else if gotAugmentWhenStmt != tc.augmentWhenStmt {
					t.Errorf("Expected augment when statement %v, but got %v",
						tc.augmentWhenStmt, gotAugmentWhenStmt)
				}
			}

			for name, entry := range augment.Dir {
				if _, ok := tc.augmentChildNames[name]; ok {
					tc.augmentChildNames[name] = true
				} else {
					t.Errorf("Got unexpected child name %v in augment", name)
				}

				if entry.Dir != nil {
					t.Errorf("Expected augment's child entry %v have nil dir, but got %v",
						name, entry.Dir)
				}
			}

			for name, matched := range tc.augmentChildNames {
				if !matched {
					t.Errorf("Expected child name %v in augment, but not present", name)
				}
			}

		})
	}
}

func TestUsesEntry(t *testing.T) {
	ParseOptions.StoreUses = true
	defer func() { ParseOptions.StoreUses = false }()

	ms := NewModules()
	for _, tt := range testAugmentAndUsesModules {
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("could not parse module %s: %v", tt.name, err)
		}
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	orig, _ := ms.GetModule("original")

	testcases := []struct {
		descr              string
		usesParentEntry    *Entry
		usesWhenStmts      []string
		groupingChildNames []map[string]bool
		nestedLevel        int
	}{
		{
			descr:              "second augment in augments.yang uses grouping delta",
			usesParentEntry:    orig.Dir["alpha"].Augmented[1],
			usesWhenStmts:      []string{"current()/orig:beta = 'nihaoWorld'"},
			groupingChildNames: []map[string]bool{{"echo": false}},
		}, {
			descr:           "container alpha uses nested grouping nestedLevel0",
			usesParentEntry: orig.Dir["alpha"],
			usesWhenStmts: []string{
				"beta = 'holaWorld'",
				"orig:psi = 'geiasouWorld'",
				"orig:omega = 'salveWorld'",
			},
			groupingChildNames: []map[string]bool{
				{"leafAtLevel0": false, "leafAtLevel1": false, "leafAtLevel2": false},
				{"leafAtLevel1": false, "leafAtLevel2": false},
				{"leafAtLevel2": false},
			},
			nestedLevel: 2,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.descr, func(t *testing.T) {
			usesParentEntry := tc.usesParentEntry
			for i := 0; i <= tc.nestedLevel; i++ {
				usesStmts := usesParentEntry.Uses
				// want the usesStmts to have length 1, otherwise also need to verify
				// every usesStmt slice element is expected.
				if len(usesStmts) != 1 {
					t.Errorf("Expected usesStmts to have length 1, but got %v",
						len(usesStmts))
				}

				usesNode := usesStmts[0].Uses
				grouping := usesStmts[0].Grouping

				if tc.usesWhenStmts[i] != "" {
					if gotUsesWhenStmt, ok := usesNode.When.Statement().Arg(); !ok {
						t.Errorf("Expected uses when statement %v, but not present",
							tc.usesWhenStmts[i])
					} else if gotUsesWhenStmt != tc.usesWhenStmts[i] {
						t.Errorf("Expected uses when statement %v, but got %v",
							tc.usesWhenStmts[i], gotUsesWhenStmt)
					}
				}

				for name, entry := range grouping.Dir {
					if _, ok := tc.groupingChildNames[i][name]; ok {
						tc.groupingChildNames[i][name] = true
					} else {
						t.Errorf("Got unexpected child name %v in uses", name)
					}

					if entry.Dir != nil {
						t.Errorf("Expected uses's child entry %v have nil dir, but got %v",
							name, entry.Dir)
					}
				}

				for name, matched := range tc.groupingChildNames[i] {
					if !matched {
						t.Errorf("Expected child name %v in grouping %v, but not present",
							name, grouping.Name)
					}
				}
				usesParentEntry = grouping
			}

		})
	}
}

func TestShallowDup(t *testing.T) {
	testModule := struct {
		name string
		in   string
	}{

		name: "mod.yang",
		in: `
module mod {
  namespace "urn:mod";
  prefix "mod";

  container level0 {
    container level1-1 {
      leaf level2-1 { type string;}
    }

    container level1-2 {
      leaf level2-2 { type string;}
    }

    container level1-3{
      container level2-3 {
        leaf level3-1 { type string;}
      }
    }
  }
}
`,
	}

	ms := NewModules()

	if err := ms.Parse(testModule.in, testModule.name); err != nil {
		t.Fatalf("could not parse module %s: %v", testModule.name, err)
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	mod, _ := ms.GetModule("mod")
	level0 := mod.Dir["level0"]
	level0ShallowDup := level0.shallowDup()

	for name, entry := range level0.Dir {
		shallowDupedEntry, ok := level0ShallowDup.Dir[name]
		if !ok {
			t.Errorf("Expect shallowDup() to duplicate direct child %v, but did not", name)
		}
		if len(entry.Dir) != 1 {
			t.Errorf("Expect original entry's direct child have length 1 dir")
		}
		if shallowDupedEntry.Dir != nil {
			t.Errorf("Expect shallowDup()'ed entry's direct child to have nil dir")
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
		customVerify     func(t *testing.T, module *Entry)
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
	}, {
		name: "parent with grouping and with extension",
		inModules: map[string]string{
			"parent": `
			module parent {
				prefix "p";
				namespace "urn:p";

				import extensions {
					prefix "ext";
				}

				container c {
					ext:c-define "c's extension";
					uses daughter-group {
						ext:u-define "uses's extension";
					}
				}

				grouping daughter-group {
					ext:g-define "daughter-group's extension";

					leaf l {
						ext:l-define "l's extension";
						type string;
					}

					container c2 {
						leaf l2 {
							type string;
						}
					}

					// test nested grouping extensions.
					uses son-group {
						ext:sg-define "son-group's extension";
					}
				}

				grouping son-group {
					leaf s {
						ext:s-define "s's extension";
						type string;
					}

				}
			}
			`,
			"extension": `
			module extensions {
				prefix "q";
				namespace "urn:q";

				extension c-define {
					description
					"Takes as an argument a name string.
					c's extension.";
					argument "name";
			       }
				extension g-define {
					description
					"Takes as an argument a name string.
					daughter-group's extension.";
					argument "name";
			       }
				extension sg-define {
					description
					"Takes as an argument a name string.
					son-groups's extension.";
					argument "name";
			       }
				extension s-define {
					description
					"Takes as an argument a name string.
					s's extension.";
					argument "name";
			       }
				extension l-define {
					description
					"Takes as an argument a name string.
					l's extension.";
					argument "name";
			       }
				extension u-define {
					description
					"Takes as an argument a name string.
					uses's extension.";
					argument "name";
			       }
			}
			`,
		},
		wantLeaves: map[string][]string{
			"parent": {"c"},
		},
		customVerify: func(t *testing.T, module *Entry) {
			// Verify that an extension within the uses statement
			// and within a grouping's definition is copied to each
			// of the top-level nodes within the grouping, and no
			// one else above or below.
			less := cmpopts.SortSlices(func(l, r *Statement) bool { return l.Keyword < r.Keyword })

			if diff := cmp.Diff([]*Statement{
				{Keyword: "ext:c-define", HasArgument: true, Argument: "c's extension"},
			}, module.Dir["c"].Exts, cmpopts.IgnoreUnexported(Statement{}), less); diff != "" {
				t.Errorf("container c Exts (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff([]*Statement{
				{Keyword: "ext:g-define", HasArgument: true, Argument: "daughter-group's extension"},
				{Keyword: "ext:l-define", HasArgument: true, Argument: "l's extension"},
				{Keyword: "ext:u-define", HasArgument: true, Argument: "uses's extension"},
			}, module.Dir["c"].Dir["l"].Exts, cmpopts.IgnoreUnexported(Statement{}), less); diff != "" {
				t.Errorf("leaf l Exts (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff([]*Statement{
				{Keyword: "ext:g-define", HasArgument: true, Argument: "daughter-group's extension"},
				{Keyword: "ext:sg-define", HasArgument: true, Argument: "son-group's extension"},
				{Keyword: "ext:s-define", HasArgument: true, Argument: "s's extension"},
				{Keyword: "ext:u-define", HasArgument: true, Argument: "uses's extension"},
			}, module.Dir["c"].Dir["s"].Exts, cmpopts.IgnoreUnexported(Statement{}), less); diff != "" {
				t.Errorf("leaf s Exts (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff([]*Statement{
				{Keyword: "ext:g-define", HasArgument: true, Argument: "daughter-group's extension"},
				{Keyword: "ext:u-define", HasArgument: true, Argument: "uses's extension"},
			}, module.Dir["c"].Dir["c2"].Exts, cmpopts.IgnoreUnexported(Statement{}), less); diff != "" {
				t.Errorf("container c2 Exts (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff([]*Statement{}, module.Dir["c"].Dir["c2"].Dir["l2"].Exts, cmpopts.IgnoreUnexported(Statement{}), less, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("leaf l2 Exts (-want, +got):\n%s", diff)
			}
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

			if tt.customVerify != nil {
				tt.customVerify(t, mod)
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
		noInput       bool
		noOutput      bool
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

		{
			name:          "minimal rpc",
			wantNodeKind:  "rpc",
			operationPath: []string{"operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  rpc operation {
    description "rpc";
  }
}`,
			noInput:  true,
			noOutput: true,
		},

		{
			name:          "input-only rpc",
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
  }
}`,
			noOutput: true,
		},

		{
			name:          "output-only rpc",
			wantNodeKind:  "rpc",
			operationPath: []string{"operation"},
			inModule: `module test {
  namespace "urn:test";
  prefix "test";
  rpc operation {
    description "rpc";
    output {
      leaf string { type string; }
    }
  }
}`,
			noInput: true,
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
		} else if !tt.noInput && e.RPC.Input == nil {
			t.Errorf("%s: RPCEntry has nil Input, want: non-nil. Entry: %#v", tt.name, e.RPC)
		} else if !tt.noOutput && e.RPC.Output == nil {
			t.Errorf("%s: RPCEntry has nil Output, want: non-nil. Entry: %#v", tt.name, e.RPC)
		}
	}
}

var testIfFeatureModules = []struct {
	name string
	in   string
}{
	{
		name: "if-feature.yang",
		in: `module if-feature {
  namespace "urn:if-feature";
  prefix "feat";

  feature ft-container;
  feature ft-action;
  feature ft-anydata1;
  feature ft-anydata2;
  feature ft-anyxml;
  feature ft-choice;
  feature ft-case;
  feature ft-feature;
  feature ft-leaf;
  feature ft-bit;
  feature ft-leaf-list;
  feature ft-enum;
  feature ft-list;
  feature ft-notification;
  feature ft-rpc;
  feature ft-augment;
  feature ft-identity;
  feature ft-uses;
  feature ft-refine;

  container cont {
    if-feature ft-container;
    action act {
      if-feature ft-action;
    }
  }

  anydata data {
    if-feature ft-anydata1;
    if-feature ft-anydata2;
  }

  anyxml xml {
    if-feature ft-anyxml;
  }

  choice ch {
    if-feature ft-choice;
    case cs {
      if-feature ft-case;
    }
  }

  feature f {
    if-feature ft-feature;
  }

  leaf l {
    if-feature ft-leaf;
    type bits {
      bit A {
        if-feature ft-bit;
      }
    }
  }

  leaf-list ll {
    if-feature ft-leaf-list;
    type enumeration {
      enum zero {
        if-feature ft-enum;
      }
    }
  }

  list ls {
    if-feature ft-list;
  }

  notification n {
    if-feature ft-notification;
  }

  rpc r {
    if-feature ft-rpc;
  }

  augment "/cont" {
    if-feature ft-augment;
  }

  identity id {
    if-feature ft-identity;
  }

  uses g {
    if-feature ft-uses;
    refine rf {
      if-feature ft-refine;
    }
  }
  grouping g {}
}
`,
	},
}

func TestIfFeature(t *testing.T) {
	entryIfFeatures := func(e *Entry) []*Value {
		extra := e.Extra["if-feature"]
		if len(extra) == 0 {
			return nil
		}
		return extra[0].([]*Value)
	}

	featureByName := func(e *Entry, name string) *Feature {
		for _, f := range e.Extra["feature"][0].([]*Feature) {
			if f.Name == name {
				return f
			}
		}
		return nil
	}

	ms := NewModules()
	for _, tt := range testIfFeatureModules {
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("could not parse module %s: %v", tt.name, err)
		}
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	mod, _ := ms.GetModule("if-feature")

	testcases := []struct {
		name           string
		inIfFeatures   []*Value
		wantIfFeatures []string
	}{
		// Node statements
		{
			name:           "action",
			inIfFeatures:   entryIfFeatures(mod.Dir["cont"].Dir["act"]),
			wantIfFeatures: []string{"ft-action"},
		},
		{
			name:           "anydata",
			inIfFeatures:   entryIfFeatures(mod.Dir["data"]),
			wantIfFeatures: []string{"ft-anydata1", "ft-anydata2"},
		},
		{
			name:           "anyxml",
			inIfFeatures:   entryIfFeatures(mod.Dir["xml"]),
			wantIfFeatures: []string{"ft-anyxml"},
		},
		{
			name:           "case",
			inIfFeatures:   entryIfFeatures(mod.Dir["ch"].Dir["cs"]),
			wantIfFeatures: []string{"ft-case"},
		},
		{
			name:           "choice",
			inIfFeatures:   entryIfFeatures(mod.Dir["ch"]),
			wantIfFeatures: []string{"ft-choice"},
		},
		{
			name:           "container",
			inIfFeatures:   entryIfFeatures(mod.Dir["cont"]),
			wantIfFeatures: []string{"ft-container"},
		},
		{
			name:           "feature",
			inIfFeatures:   featureByName(mod, "f").IfFeature,
			wantIfFeatures: []string{"ft-feature"},
		},
		{
			name:           "leaf",
			inIfFeatures:   entryIfFeatures(mod.Dir["l"]),
			wantIfFeatures: []string{"ft-leaf"},
		},
		{
			name:           "leaf-list",
			inIfFeatures:   entryIfFeatures(mod.Dir["ll"]),
			wantIfFeatures: []string{"ft-leaf-list"},
		},
		{
			name:           "list",
			inIfFeatures:   entryIfFeatures(mod.Dir["ls"]),
			wantIfFeatures: []string{"ft-list"},
		},
		{
			name:           "notification",
			inIfFeatures:   entryIfFeatures(mod.Dir["n"]),
			wantIfFeatures: []string{"ft-notification"},
		},
		{
			name:           "rpc",
			inIfFeatures:   entryIfFeatures(mod.Dir["r"]),
			wantIfFeatures: []string{"ft-rpc"},
		},
		// Other statements
		{
			name:           "augment",
			inIfFeatures:   entryIfFeatures(mod.Dir["cont"].Augmented[0]),
			wantIfFeatures: []string{"ft-augment"},
		},
		{
			name:           "bit",
			inIfFeatures:   mod.Dir["l"].Node.(*Leaf).Type.Bit[0].IfFeature,
			wantIfFeatures: []string{"ft-bit"},
		},
		{
			name:           "enum",
			inIfFeatures:   mod.Dir["ll"].Node.(*Leaf).Type.Enum[0].IfFeature,
			wantIfFeatures: []string{"ft-enum"},
		},
		{
			name:           "identity",
			inIfFeatures:   mod.Identities[0].IfFeature,
			wantIfFeatures: []string{"ft-identity"},
		},
		{
			name:           "refine",
			inIfFeatures:   ms.Modules["if-feature"].Uses[0].Refine[0].IfFeature,
			wantIfFeatures: []string{"ft-refine"},
		},
		{
			name:           "uses",
			inIfFeatures:   ms.Modules["if-feature"].Uses[0].IfFeature,
			wantIfFeatures: []string{"ft-uses"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var names []string
			for _, f := range tc.inIfFeatures {
				names = append(names, f.Name)
			}

			if !reflect.DeepEqual(names, tc.wantIfFeatures) {
				t.Errorf("%s: did not get expected if-features, got %v, want %v", tc.name, names, tc.wantIfFeatures)
			}
		})
	}
}

var testNotificationModules = []struct {
	name string
	in   string
}{
	{
		name: "notification.yang",
		in: `module notification {
  namespace "urn:notification";
  prefix "n";

  notification n {}

  grouping g {
    notification g-n {}
  }

  container cont {
    notification cont-n {}
  }

  list ls {
    notification ls-n {}
    uses g;
  }

  augment "/cont" {
    notification aug-n {}
  }
}
`,
	},
}

func TestNotification(t *testing.T) {
	ms := NewModules()
	for _, tt := range testNotificationModules {
		if err := ms.Parse(tt.in, tt.name); err != nil {
			t.Fatalf("could not parse module %s: %v", tt.name, err)
		}
	}

	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("could not process modules: %v", errs)
	}

	mod, _ := ms.GetModule("notification")

	testcases := []struct {
		name     string
		wantPath []string
	}{
		{
			name:     "module",
			wantPath: []string{"n"},
		},
		{
			name:     "container",
			wantPath: []string{"cont", "cont-n"},
		},
		{
			name:     "list",
			wantPath: []string{"ls", "ls-n"},
		},
		{
			name:     "grouping",
			wantPath: []string{"ls", "g-n"},
		},
		{
			name:     "augment",
			wantPath: []string{"cont", "aug-n"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if e := getEntry(mod, tc.wantPath); e == nil || e.Node.Kind() != "notification" {
				t.Errorf("%s: want notification entry at: %v, got: %+v", tc.name, tc.wantPath, e)
			}
		})
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
			"/t:c/d":                    "/test/c/d",
			"/c/t:d":                    "/test/c/d",
			"../t:rpc1/input":           "/test/rpc1/input",
			"/t:rpc1/input":             "/test/rpc1/input",
			"/t:rpc1/t:input":           "/test/rpc1/input",
			"/t:e/operation/input":      "/test/e/operation/input",
			"/t:e/operation/output":     "/test/e/operation/output",
			"/t:e/t:operation/t:input":  "/test/e/operation/input",
			"/t:e/t:operation/t:output": "/test/e/operation/output",
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
					"case1-leaf1": {
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

func TestFixChoice(t *testing.T) {
	choiceEntry := &Entry{
		Name: "choiceEntry",
		Kind: ChoiceEntry,
		Dir: map[string]*Entry{
			"unnamedAnyDataCase": {
				Name: "unnamedAnyDataCase",
				Kind: AnyDataEntry,
				Node: &AnyData{
					Parent: &Container{
						Name: "AnyDataParentNode",
					},
					Name: "unnamedAnyDataCase",
					Source: &Statement{
						Keyword:     "anyData-keyword",
						HasArgument: true,
						Argument:    "anyData-argument",
						statements:  nil,
					},
					Extensions: []*Statement{
						{
							Keyword:     "anyData-extension",
							HasArgument: true,
							Argument:    "anyData-extension-arg",
							statements:  nil,
						},
					},
				},
			},
			"unnamedAnyXMLCase": {
				Name: "unnamedAnyXMLCase",
				Kind: AnyXMLEntry,
				Node: &AnyXML{
					Parent: &Container{
						Name: "AnyXMLParentNode",
					},
					Name: "unnamedAnyXMLCase",
					Source: &Statement{
						Keyword:     "anyXML-keyword",
						HasArgument: true,
						Argument:    "anyXML-argument",
						statements:  nil,
					},
					Extensions: []*Statement{
						{
							Keyword:     "anyXML-extension",
							HasArgument: true,
							Argument:    "anyXML-extension-arg",
							statements:  nil,
						},
					},
				},
			},
			"unnamedContainerCase": {
				Name: "unnamedContainerCase",
				Kind: DirectoryEntry,
				Node: &Container{
					Parent: &Container{
						Name: "AnyContainerNode",
					},
					Name: "unnamedContainerCase",
					Source: &Statement{
						Keyword:     "container-keyword",
						HasArgument: true,
						Argument:    "container-argument",
						statements:  nil,
					},
					Extensions: []*Statement{
						{
							Keyword:     "container-extension",
							HasArgument: true,
							Argument:    "container-extension-arg",
							statements:  nil,
						},
					},
				},
			},
			"unnamedLeafCase": {
				Name: "unnamedLeafCase",
				Kind: LeafEntry,
				Node: &Leaf{
					Parent: &Container{
						Name: "leafParentNode",
					},
					Name: "unnamedLeafCase",
					Source: &Statement{
						Keyword:     "leaf-keyword",
						HasArgument: true,
						Argument:    "leaf-argument",
						statements:  nil,
					},
					Extensions: []*Statement{
						{
							Keyword:     "leaf-extension",
							HasArgument: true,
							Argument:    "leaf-extension-arg",
							statements:  nil,
						},
					},
				},
			},
			"unnamedLeaf-ListCase": {
				Name: "unnamedLeaf-ListCase",
				Kind: LeafEntry,
				Node: &LeafList{
					Parent: &Container{
						Name: "LeafListNode",
					},
					Name: "unnamedLeaf-ListCase",
					Source: &Statement{
						Keyword:     "leaflist-keyword",
						HasArgument: true,
						Argument:    "leaflist-argument",
						statements:  nil,
					},
					Extensions: []*Statement{
						{
							Keyword:     "leaflist-extension",
							HasArgument: true,
							Argument:    "leaflist-extension-arg",
							statements:  nil,
						},
					},
				},
			},
			"unnamedListCase": {
				Name: "unnamedListCase",
				Kind: DirectoryEntry,
				Node: &List{
					Parent: &Container{
						Name: "ListNode",
					},
					Name: "unnamedListCase",
					Source: &Statement{
						Keyword:     "list-keyword",
						HasArgument: true,
						Argument:    "list-argument",
						statements:  nil,
					},
					Extensions: []*Statement{
						{
							Keyword:     "list-extension",
							HasArgument: true,
							Argument:    "list-extension-arg",
							statements:  nil,
						},
					},
				},
			},
		},
	}

	choiceEntry.FixChoice()

	for _, e := range []string{"AnyData", "AnyXML", "Container",
		"Leaf", "Leaf-List", "List"} {
		entryName := "unnamed" + e + "Case"
		t.Run(entryName, func(t *testing.T) {

			insertedCase := choiceEntry.Dir[entryName]
			originalCase := insertedCase.Dir[entryName]

			insertedNode := insertedCase.Node
			if insertedNode.Kind() != "case" {
				t.Errorf("Got inserted node type %s, expected case",
					insertedNode.Kind())
			}

			originalNode := originalCase.Node
			if originalNode.Kind() != strings.ToLower(e) {
				t.Errorf("Got original node type %s, expected %s",
					originalNode.Kind(), strings.ToLower(e))
			}

			if insertedNode.ParentNode() != originalNode.ParentNode() {
				t.Errorf("Got inserted node's parent node %v, expected %v",
					insertedNode.ParentNode(), originalNode.ParentNode())
			}

			if insertedNode.NName() != originalNode.NName() {
				t.Errorf("Got inserted node's name %s, expected %s",
					insertedNode.NName(), originalNode.NName())
			}

			if insertedNode.Statement() != originalNode.Statement() {
				t.Errorf("Got inserted node's statement %v, expected %s",
					insertedNode.Statement(), originalNode.Statement())
			}

			if len(insertedNode.Exts()) != len(originalNode.Exts()) {
				t.Errorf("Got inserted node extensions slice len %d, expected %v",
					len(insertedNode.Exts()), len(originalNode.Exts()))
			}

			for i, e := range insertedNode.Exts() {
				if e != originalNode.Exts()[i] {
					t.Errorf("Got inserted node's extension %v at index %d, expected %v",
						e, i, originalNode.Exts()[i])
				}
			}
		})
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
			"deviate": {{
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
			"deviate": {{
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
			"source": {{
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
			"deviate": {{
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
			"deviate": {{
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
			"source": {{
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
			"foo": {{
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

func TestLeafEntryTypes(t *testing.T) {
	tests := []struct {
		name                string
		inModules           map[string]string
		wantEntryPath       string
		wantEntryCustomTest func(t *testing.T, e *Entry)
		wantErrSubstr       string
	}{{
		name: "direct decimal64 type",
		inModules: map[string]string{
			"test.yang": `
			module test {
				prefix "t";
				namespace "urn:t";

				leaf "gain-adjustment" {
					type "decimal64" {
						fraction-digits "1";
						range "-12.0..12.0";
					}
					default "0.0";
				}
			}
			`,
		},
		wantEntryPath: "/test/gain-adjustment",
		wantEntryCustomTest: func(t *testing.T, e *Entry) {
			if got, want := e.Type.FractionDigits, 1; got != want {
				t.Errorf("got %d, want %d", got, want)
			}
			if diff := cmp.Diff(e.Type.Range, YangRange{Rf(-120, 120, 1)}); diff != "" {
				t.Errorf("Range (-got, +want):\n%s", diff)
			}
		},
	}, {
		name: "typedef decimal64 type",
		inModules: map[string]string{
			"test.yang": `
			module test {
				prefix "t";
				namespace "urn:t";

				typedef "optical-dB" {
					type "decimal64" {
						fraction-digits "1";
					}
				}

				leaf "gain-adjustment" {
					type "optical-dB" {
						range "-12.0..12.0";
					}
					default "0.0";
				}
			}
			`,
		},
		wantEntryPath: "/test/gain-adjustment",
		wantEntryCustomTest: func(t *testing.T, e *Entry) {
			if got, want := e.Type.FractionDigits, 1; got != want {
				t.Errorf("got %d, want %d", got, want)
			}
			if diff := cmp.Diff(e.Type.Range, YangRange{Rf(-120, 120, 1)}); diff != "" {
				t.Errorf("Range (-got, +want):\n%s", diff)
			}
		},
	}, {
		name: "typedef decimal64 type with overriding fraction-digits",
		inModules: map[string]string{
			"test.yang": `
			module test {
				prefix "t";
				namespace "urn:t";

				typedef "optical-dB" {
					type "decimal64" {
						fraction-digits "1";
					}
				}

				leaf "gain-adjustment" {
					type "optical-dB" {
						fraction-digits "2";
						range "-12.0..12.0";
					}
					default "0.0";
				}
			}
			`,
		},
		wantErrSubstr: "overriding of fraction-digits not allowed",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewModules()
			var errs []error
			for n, m := range tt.inModules {
				if err := ms.Parse(m, n); err != nil {
					errs = append(errs, err)
				}
			}

			if len(errs) > 0 {
				t.Fatalf("ms.Parse(), got unexpected error parsing input modules: %v", errs)
			}

			if errs := ms.Process(); len(errs) > 0 {
				if len(errs) == 1 {
					if diff := errdiff.Substring(errs[0], tt.wantErrSubstr); diff != "" {
						t.Fatalf("did not get expected error, %s", diff)
					}
					return
				}
				t.Fatalf("ms.Process(), got too many errors processing entries: %v", errs)
			}

			dir := map[string]*Entry{}
			for _, m := range ms.Modules {
				addTreeE(ToEntry(m), dir)
			}

			e, ok := dir[tt.wantEntryPath]
			if !ok {
				t.Fatalf("could not find entry %s within the dir: %v", tt.wantEntryPath, dir)
			}
			tt.wantEntryCustomTest(t, e)
		})
	}
}
