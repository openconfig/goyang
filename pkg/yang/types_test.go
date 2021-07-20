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

func TestTypeLengthRange(t *testing.T) {
	tests := []struct {
		desc          string
		leafNode      string
		wantType      *testTypeStruct
		wantErrSubstr string
	}{{
		desc: "simple uint32",
		leafNode: `
			typedef alpha {
				type uint32 {
					range "1..4 | 10..20";
				}
			}
			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:  "alpha",
			Range: YangRange{R(1, 4), R(10, 20)},
		},
	}, {
		desc: "inherited uint32",
		leafNode: `
			typedef alpha {
				type uint32 {
					range "1..4 | 10..20";
				}
			}
			typedef bravo {
				type alpha {
					range "min..3 | 12..max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:  "bravo",
			Range: YangRange{R(1, 3), R(12, 20)},
		},
	}, {
		desc: "inherited uint32 range violation",
		leafNode: `
			typedef alpha {
				type uint32 {
					range "1..4 | 10..20";
				}
			}
			typedef bravo {
				type alpha {
					range "min..max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantErrSubstr: "not within",
	}, {
		desc: "simple decimal64",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 2;
					range "1 .. 3.14 | 10 | 20..max";
				}
			}
			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:  "alpha",
			Range: YangRange{Rf(100, 314, 2), Rf(1000, 1000, 2), Rf(2000, MaxInt64, 2)},
		},
	}, {
		desc: "simple decimal64 with inherited ranges",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 3;
					range "1 .. 3.14 | 10 | 20..max";
				}
			}
			typedef bravo {
				type alpha {
					range "min .. 2.72 | 42 .. max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:  "bravo",
			Range: YangRange{Rf(1000, 2720, 3), Rf(42000, MaxInt64, 3)},
		},
	}, {
		desc: "simple decimal64 with inherited ranges",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 2;
					range "1 .. 3.14 | 10 | 20..max";
				}
			}
			typedef bravo {
				type alpha {
					range "min..max";
				}
			}
			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantErrSubstr: "not within",
	}, {
		desc: "simple decimal64 with too few fractional digits",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 1;
					range "1 .. 3.14 | 10 | 20..max";
				}
			}
			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantErrSubstr: "has too much precision",
	}, {
		desc: "simple decimal64 fractional digit on inherited decimal64 type",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 2;
					range "1 .. 3.14 | 10 | 20..max";
				}
			}
			typedef bravo {
				type alpha {
					fraction-digits 2;
					range "25..max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantErrSubstr: "overriding of fraction-digits not allowed",
	}, {
		desc: "simple string with length",
		leafNode: `
			typedef alpha {
				type string {
					length "1..4 | 10..20 | 30..max";
				}
			}
			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:   "alpha",
			Length: YangRange{R(1, 4), R(10, 20), YRange{FromInt(30), FromUint(maxUint64)}},
		},
	}, {
		desc: "inherited string",
		leafNode: `
			typedef alpha {
				type string {
					length "1..4 | 10..20 | 30..max";
				}
			}
			typedef bravo {
				type alpha {
					length "min..3 | 42..max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:   "bravo",
			Length: YangRange{R(1, 3), YRange{FromInt(42), FromUint(maxUint64)}},
		},
	}, {
		desc: "inherited binary",
		leafNode: `
			typedef alpha {
				type binary {
					length "1..4 | 10..20 | 30..max";
				}
			}
			typedef bravo {
				type alpha {
					length "min..3 | 42..max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantType: &testTypeStruct{
			Name:   "bravo",
			Length: YangRange{R(1, 3), YRange{FromInt(42), FromUint(maxUint64)}},
		},
	}, {
		desc: "inherited string length violation",
		leafNode: `
			typedef alpha {
				type string {
					length "1..4 | 10..20 | 30..max";
				}
			}
			typedef bravo {
				type alpha {
					length "min..max";
				}
			}
			leaf test-leaf {
				type bravo;
			}
		} // end module`,
		wantErrSubstr: "not within",
	}, {
		desc: "simple union",
		leafNode: `
				typedef alpha {
					type union {
						type string;
						type binary {
							length "min..5|999..max";
						}
						type int8 {
							range "min..-42|42..max";
						}
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
				Name:   "binary",
				Length: YangRange{R(0, 5), YRange{FromInt(999), FromUint(maxUint64)}},
			}, {
				Name:  "int8",
				Range: YangRange{R(minInt8, -42), R(42, maxInt8)},
			}, {
				Name: "enumeration",
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

			if diff := cmp.Diff(filterRanges(gotType), tt.wantType); diff != "" {
				t.Errorf("Type.resolve() union types test (-got, +want):\n%s", diff)
			}
		})
	}
}

// testTypeStruct is a filtered-down version of YangType where only certain
// fields are preserved for targeted testing.
type testTypeStruct struct {
	Name   string
	Length YangRange
	Range  YangRange
	Type   []*testTypeStruct
}

// filterRanges returns a testTypeStruct with only the Name, Length, and Range
// fields of the given YangType, preserving the recursive structure of the
// type, to work around cmp not having an allowlist way of specifying which
// fields to compare and YangType having a custom Equal function.
func filterRanges(ytype *YangType) *testTypeStruct {
	filteredType := &testTypeStruct{Name: ytype.Name}
	filteredType.Length = ytype.Length
	filteredType.Range = ytype.Range
	for _, subtype := range ytype.Type {
		filteredType.Type = append(filteredType.Type, filterRanges(subtype))
	}
	return filteredType
}
