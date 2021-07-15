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
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/gnmi/errdiff"
)

func TestTypeResolve(t *testing.T) {
	for x, tt := range []struct {
		in  *Type
		err string
		out *YangType
	}{
		{
			in: &Type{
				Name: "int64",
			},
			out: &YangType{
				Name:  "int64",
				Kind:  Yint64,
				Range: Int64Range,
			},
		},
		{
			in: &Type{
				Name:           "boolean",
				FractionDigits: &Value{Name: "42"},
			},
			err: "unknown: fraction-digits only allowed for decimal64 values",
		},
		{
			in: &Type{
				Name: "decimal64",
			},
			err: "unknown: value is required in the range of [1..18]",
		},
		{
			in: &Type{
				Name: "identityref",
			},
			err: "unknown: an identityref must specify a base",
		},
		{
			in: &Type{
				Name:           "decimal64",
				FractionDigits: &Value{Name: "42"},
			},
			err: "unknown: value 42 out of range [1..18]",
		},
		{
			in: &Type{
				Name:           "decimal64",
				FractionDigits: &Value{Name: "7"},
			},
			out: &YangType{
				Name:           "decimal64",
				Kind:           Ydecimal64,
				FractionDigits: 7,
				Range:          Decimal64Range,
			},
		},
		// TODO(borman): Add in more tests as we honor more fields
		// in Type.
	} {
		// We can initialize a value to ourself, so to it here.
		errs := tt.in.resolve(&typeDict)

		// TODO(borman):  Do not hack out Root and Base.  These
		// are hacked out for now because they can be self-referential,
		// making construction of them difficult.
		tt.in.YangType.Root = nil
		tt.in.YangType.Base = nil

		switch {
		case tt.err == "" && len(errs) > 0:
			t.Errorf("#%d: unexpected errors: %v", x, errs)
		case tt.err != "" && len(errs) == 0:
			t.Errorf("#%d: did not get expected errors: %v", x, tt.err)
		case len(errs) > 1:
			t.Errorf("#%d: too many errors: %v", x, errs)
		case len(errs) == 1 && errs[0].Error() != tt.err:
			t.Errorf("#%d: got error %v, want %s", x, errs[0], tt.err)
		case len(errs) != 0:
		case !reflect.DeepEqual(tt.in.YangType, tt.out):
			t.Errorf("#%d: got %#v, want %#v", x, tt.in.YangType, tt.out)
		}
	}
}

func TestTypeResolveUnions(t *testing.T) {
	tests := []struct {
		desc          string
		leafNode      string
		wantType      *testTypeStruct
		wantErrSubstr string
	}{{
		desc: "simple union",
		leafNode: `
			typedef alpha {
				type union {
					type string;
					type uint32;
					type enumeration {
						enum zero;
						enum one;
						enum seven {
							value 7;
						}
					}
				}
			}

			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "alpha",
			Type: []*testTypeStruct{{
				Name: "string",
			}, {
				Name: "uint32",
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
			}},
		},
	}, {
		desc: "union with typedef",
		leafNode: `
			typedef alpha {
				type union {
					type string;
					type uint32;
					type enumeration {
						enum zero;
						enum one;
						enum seven {
							value 7;
						}
					}
					type bravo;
				}
			}

			typedef bravo {
				type union {
					type uint8;
					type uint16;
					type enumeration {
						enum two {
							value 2;
						}
						enum three;
						enum four;
					}
				}
			}

			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "alpha",
			Type: []*testTypeStruct{{
				Name: "string",
			}, {
				Name: "uint32",
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
			}, {
				Name: "bravo",
				Type: []*testTypeStruct{{
					Name: "uint8",
				}, {
					Name: "uint16",
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"two": 2, "three": 3, "four": 4},
				}},
			}},
		},
	}, {
		desc: "nested unions with typedef",
		leafNode: `
			typedef alpha {
				type union {
					type union {
						type uint32;
						type string;
						type enumeration {
							enum zero;
							enum one;
							enum seven {
								value 7;
							}
						}
					}
					type bravo;
				}
			}

			typedef bravo {
				type union {
					type uint8;
					type uint16;
					type enumeration {
						enum two {
							value 2;
						}
						enum three;
						enum four;
					}
				}
			}

			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "alpha",
			Type: []*testTypeStruct{{
				Name: "union",
				Type: []*testTypeStruct{{
					Name: "uint32",
				}, {
					Name: "string",
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
				}},
			}, {
				Name: "bravo",
				Type: []*testTypeStruct{{
					Name: "uint8",
				}, {
					Name: "uint16",
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"two": 2, "three": 3, "four": 4},
				}},
			}},
		},
	}, {
		desc: "simple union with multiple enumerations",
		leafNode: `
			leaf test-leaf {
				type union {
					type string;
					type uint32;
					type enumeration {
						enum zero;
						enum one;
						enum seven {
							value 7;
						}
					}
					type enumeration {
						enum two {
							value 2;
						}
						enum three;
						enum four;
					}
				}
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "union",
			Type: []*testTypeStruct{{
				Name: "string",
			}, {
				Name: "uint32",
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"two": 2, "three": 3, "four": 4},
			}},
		},
	}, {
		desc: "typedef union with multiple enumerations",
		leafNode: `
			typedef alpha {
				type union {
					type string;
					type uint32;
					type enumeration {
						enum zero;
						enum one;
						enum seven {
							value 7;
						}
					}
					type enumeration {
						enum two {
							value 2;
						}
						enum three;
						enum four;
					}
				}
			}

			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "alpha",
			Type: []*testTypeStruct{{
				Name: "string",
			}, {
				Name: "uint32",
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"two": 2, "three": 3, "four": 4},
			}},
		},
	}, {
		desc: "simple union containing typedef union, both with enumerations",
		leafNode: `
			typedef alpha {
				type union {
					type string;
					type uint32;
					type enumeration {
						enum zero;
						enum one;
						enum seven {
							value 7;
						}
					}
				}
			}

			leaf test-leaf {
				type union {
					type alpha;
					type enumeration {
						enum two {
							value 2;
						}
						enum three;
						enum four;
					}
				}
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "union",
			Type: []*testTypeStruct{{
				Name: "alpha",
				Type: []*testTypeStruct{{
					Name: "string",
				}, {
					Name: "uint32",
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
				}},
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"two": 2, "three": 3, "four": 4},
			}},
		},
	}, {
		desc: "simple union containing typedef union containing another typedef union, all with multiple simple and typedef enumerations",
		leafNode: `
			typedef a {
				type enumeration {
					enum un {
						value 1;
					}
					enum deux;
				}
			}

			typedef b {
				type enumeration {
					enum trois {
						value 3;
					}
					enum quatre;
				}
			}

			typedef c {
				type enumeration {
					enum cinq {
						value 5;
					}
					enum sept {
						value 7;
					}
				}
			}

			typedef d {
				type enumeration {
					enum huit {
						value 8;
					}
					enum neuf;
				}
			}

			typedef e {
				type enumeration {
					enum dix {
						value 10;
					}
					enum onze;
				}
			}

			typedef f {
				type enumeration {
					enum douze {
						value 12;
					}
					enum treize;
				}
			}

			typedef bravo {
				type union {
					type uint32;
					type enumeration {
						enum eight {
							value 8;
						}
						enum nine;
					}
					type enumeration {
						enum ten {
							value 10;
						}
						enum eleven;
					}
					type e;
					type f;
				}
			}

			typedef alpha {
				type union {
					type uint16;
					type enumeration {
						enum four {
							value 4;
						}
						enum five;
					}
					type enumeration {
						enum six {
							value 6;
						}
						enum seven;
					}
					type c;
					type d;
					type bravo;
				}
			}

			leaf test-leaf {
				type union {
					type uint8;
					type enumeration {
						enum zero;
						enum one;
					}
					type enumeration {
						enum two {
							value 2;
						}
						enum three;
					}
					type a;
					type b;
					type alpha;
				}
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name: "union",
			Type: []*testTypeStruct{{
				Name: "uint8",
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"zero": 0, "one": 1},
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"two": 2, "three": 3},
			}, {
				Name:  "a",
				ToInt: map[string]int64{"un": 1, "deux": 2},
			}, {
				Name:  "b",
				ToInt: map[string]int64{"trois": 3, "quatre": 4},
			}, {
				Name: "alpha",
				Type: []*testTypeStruct{{
					Name: "uint16",
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"four": 4, "five": 5},
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"six": 6, "seven": 7},
				}, {
					Name:  "c",
					ToInt: map[string]int64{"cinq": 5, "sept": 7},
				}, {
					Name:  "d",
					ToInt: map[string]int64{"huit": 8, "neuf": 9},
				}, {
					Name: "bravo",
					Type: []*testTypeStruct{{
						Name: "uint32",
					}, {
						Name:  "enumeration",
						ToInt: map[string]int64{"eight": 8, "nine": 9},
					}, {
						Name:  "enumeration",
						ToInt: map[string]int64{"ten": 10, "eleven": 11},
					}, {
						Name:  "e",
						ToInt: map[string]int64{"dix": 10, "onze": 11},
					}, {
						Name:  "f",
						ToInt: map[string]int64{"douze": 12, "treize": 13},
					}},
				}},
			}},
		},
	}}

	getTestLeaf := func(ms *Modules) (*YangType, error) {
		m, err := ms.FindModuleByPrefix("t")
		if err != nil {
			return nil, fmt.Errorf("can't find module in %v", ms)
		}
		if len(m.Leaf) == 0 {
			return nil, fmt.Errorf("node %v is missing imports", m)
		}
		e := ToEntry(m)
		return e.Dir["test-leaf"].Type, nil
	}

	for _, tt := range tests {
		inModules := map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					` + tt.leafNode,
		}

		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()
			for n, m := range inModules {
				if err := ms.Parse(m, n); err != nil {
					t.Fatalf("error parsing module %s, got: %v, want: nil", n, err)
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

			gotType, err := getTestLeaf(ms)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(filterTypeNames(gotType), tt.wantType); diff != "" {
				t.Errorf("Type.resolve() union types test (-got, +want):\n%s", diff)
			}
		})
	}
}

type testTypeStruct struct {
	Name string
	// ToInt is the toInt map representing the enum value (if present).
	ToInt map[string]int64
	Type  []*testTypeStruct
}

// filterTypeNames returns a testTypeStruct with only the
// YangType.Name fields of the given type, preserving
// the recursive structure of the type, to work around cmp not
// having an allowlist way of specifying which fields to
// compare and YangType having a custom Equal function.
func filterTypeNames(ytype *YangType) *testTypeStruct {
	filteredNames := &testTypeStruct{Name: ytype.Name}
	if ytype.Enum != nil {
		filteredNames.ToInt = ytype.Enum.toInt
	}
	for _, subtype := range ytype.Type {
		filteredNames.Type = append(filteredNames.Type, filterTypeNames(subtype))
	}
	return filteredNames
}

func TestPattern(t *testing.T) {
	tests := []struct {
		desc          string
		leafNode      string
		wantType      *YangType
		wantErrSubstr string
	}{{
		desc: "Only normal patterns",
		leafNode: `
			leaf test-leaf {
				type string {
					o:bar 'coo';
					o:bar 'foo';
					pattern 'charlie';
					o:bar 'goo';
				}
			}
		} // end module`,
		wantType: &YangType{
			Pattern: []string{"charlie"},
		},
	}, {
		desc: "Only posix patterns",
		leafNode: `
			leaf test-leaf {
				type string {
					o:bar 'coo';
					o:posix-pattern 'bravo';
					o:bar 'foo';
					o:posix-pattern 'charlie';
					o:bar 'goo';
				}
			}
		} // end module`,
		wantType: &YangType{
			POSIXPattern: []string{"bravo", "charlie"},
		},
	}, {
		desc: "No patterns",
		leafNode: `
			leaf test-leaf {
				type string;
			}
		}`,
		wantType: &YangType{
			Pattern:      nil,
			POSIXPattern: nil,
		},
	}, {
		desc: "Both patterns",
		leafNode: `
			leaf test-leaf {
				type string {
					pattern 'alpha';
					o:posix-pattern 'bravo';
					o:posix-pattern 'charlie';
					o:bar 'coo';
					o:posix-pattern 'delta';
				}
			}
		} // end module`,
		wantType: &YangType{
			Pattern:      []string{"alpha"},
			POSIXPattern: []string{"bravo", "charlie", "delta"},
		},
	}, {
		desc: "Both patterns, but with non-openconfig-extensions pretenders",
		leafNode: `
			leaf test-leaf {
				type string {
					pattern 'alpha';
					o:bar 'coo';
					o:posix-pattern 'delta';

					n:posix-pattern 'golf';

					pattern 'bravo';
					o:bar 'foo';
					o:posix-pattern 'echo';

					pattern 'charlie';
					o:bar 'goo';
					o:posix-pattern 'foxtrot';

					n:posix-pattern 'hotel';
				}
			}
		} // end module`,
		wantType: &YangType{
			Pattern:      []string{"alpha", "bravo", "charlie"},
			POSIXPattern: []string{"delta", "echo", "foxtrot"},
		},
	}, {
		desc: "Union type",
		leafNode: `
			leaf test-leaf {
				type union {
					type string {
						pattern 'alpha';
						o:bar 'coo';
						o:posix-pattern 'delta';

						pattern 'bravo';
						o:bar 'foo';
						o:posix-pattern 'echo';
						n:posix-pattern 'echo2';

						pattern 'charlie';
						o:bar 'goo';
						o:posix-pattern 'foxtrot';
					}
					type uint64;
				}
			}
		} // end module`,
		wantType: &YangType{
			Type: []*YangType{{
				Pattern:      []string{"alpha", "bravo", "charlie"},
				POSIXPattern: []string{"delta", "echo", "foxtrot"},
			}, {
				Pattern:      nil,
				POSIXPattern: nil,
			}},
		},
	}, {
		desc: "Union type -- de-duping string types",
		leafNode: `
			leaf test-leaf {
				type union {
					type string {
						pattern 'alpha';
						o:posix-pattern 'alpha';
					}
					type string {
						pattern 'alpha';
						o:posix-pattern 'alpha';
					}
				}
			}
		} // end module`,
		wantType: &YangType{
			Type: []*YangType{{
				Pattern:      []string{"alpha"},
				POSIXPattern: []string{"alpha"},
			}},
		},
	}, {
		desc: "Union type -- different string types due to different patterns",
		leafNode: `
			leaf test-leaf {
				type union {
					type string {
						pattern 'alpha';
					}
					type string {
						pattern 'bravo';
					}
				}
			}
		} // end module`,
		wantType: &YangType{
			Type: []*YangType{{
				Pattern: []string{"alpha"},
			}, {
				Pattern: []string{"bravo"},
			}},
		},
	}, {
		desc: "Union type -- different string types due to different posix-patterns",
		leafNode: `
			leaf test-leaf {
				type union {
					type string {
						o:posix-pattern 'alpha';
					}
					type string {
						o:posix-pattern 'bravo';
					}
				}
			}
		} // end module`,
		wantType: &YangType{
			Type: []*YangType{{
				POSIXPattern: []string{"alpha"},
			}, {
				POSIXPattern: []string{"bravo"},
			}},
		},
	}, {
		desc: "typedef",
		leafNode: `
			leaf test-leaf {
				type leaf-type;
			}

			typedef leaf-type {
				type string {
					pattern 'alpha';
					o:bar 'coo';
					o:posix-pattern 'delta';

					pattern 'bravo';
					o:bar 'foo';
					o:posix-pattern 'echo';

					pattern 'charlie';
					o:bar 'goo';
					o:posix-pattern 'foxtrot';
				}
			}
		} // end module`,
		wantType: &YangType{
			Pattern:      []string{"alpha", "bravo", "charlie"},
			POSIXPattern: []string{"delta", "echo", "foxtrot"},
		},
	}, {
		desc: "invalid POSIX pattern",
		leafNode: `
			leaf test-leaf {
				type leaf-type;
			}

			typedef leaf-type {
				type string {
					o:posix-pattern '?';
				}
			}
		} // end module`,
		wantErrSubstr: "bad pattern",
	}}

	getTestLeaf := func(ms *Modules) (*YangType, error) {
		m, err := ms.FindModuleByPrefix("t")
		if err != nil {
			return nil, fmt.Errorf("can't find module in %v", ms)
		}
		if len(m.Leaf) == 0 {
			return nil, fmt.Errorf("node %v is missing imports", m)
		}
		e := ToEntry(m)
		return e.Dir["test-leaf"].Type, nil
	}

	for _, tt := range tests {
		inModules := map[string]string{
			"test": `
				module test {
					prefix "t";
					namespace "urn:t";

					import non-openconfig-extensions {
						prefix "n";
						description "non-openconfig-extensions module";
					}
					import openconfig-extensions {
						prefix "o";
						description "openconfig-extensions module";
					}` + tt.leafNode,
			"openconfig-extensions": `
				module openconfig-extensions {
					prefix "o";
					namespace "urn:o";

					extension bar {
						argument "baz";
					}

					extension posix-pattern {
						argument "pattern";
					}
				}
			`,
			"non-openconfig-extensions": `
				module non-openconfig-extensions {
					prefix "n";
					namespace "urn:n";

					extension bar {
						argument "baz";
					}

					extension posix-pattern {
						argument "pattern";
					}
				}
			`,
		}

		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()
			for n, m := range inModules {
				if err := ms.Parse(m, n); err != nil {
					t.Fatalf("error parsing module %s, got: %v, want: nil", n, err)
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

			yangType, err := getTestLeaf(ms)
			if err != nil {
				t.Fatal(err)
			}

			gotType := &YangType{}
			populatePatterns(yangType, gotType)
			if diff := cmp.Diff(gotType, tt.wantType, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Type.resolve() pattern test (-got, +want):\n%s", diff)
			}
		})
	}
}

// populatePatterns populates targetType with only the
// Pattern/POSIXPattern fields of the given type, preserving
// the recursive structure of the type, to work around cmp not
// having an allowlist way of specifying which fields to
// compare.
func populatePatterns(ytype *YangType, targetType *YangType) {
	targetType.Pattern = ytype.Pattern
	targetType.POSIXPattern = ytype.POSIXPattern
	for _, subtype := range ytype.Type {
		targetSubtype := &YangType{}
		targetType.Type = append(targetType.Type, targetSubtype)
		populatePatterns(subtype, targetSubtype)
	}
}
