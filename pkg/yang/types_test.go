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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/gnmi/errdiff"
)

func TestTypeResolve(t *testing.T) {
	tests := []struct {
		desc string
		in   *Type
		err  string
		out  *YangType
	}{{
		desc: "basic int64",
		in: &Type{
			Name: "int64",
		},
		out: &YangType{
			Name:  "int64",
			Kind:  Yint64,
			Range: Int64Range,
		},
	}, {
		desc: "basic int64 with a range",
		in: &Type{
			Name:  "int64",
			Range: &Range{Name: "-42..42"},
		},
		out: &YangType{
			Name:  "int64",
			Kind:  Yint64,
			Range: YangRange{{Min: FromInt(-42), Max: FromInt(42)}},
		},
	}, {
		desc: "basic uint64 with an invalid range",
		in: &Type{
			Name:  "uint64",
			Range: &Range{Name: "-42..42"},
		},
		err: "unknown: bad range: -42..42 not within 0..18446744073709551615",
	}, {
		desc: "basic uint64 with an unparseable range",
		in: &Type{
			Name:  "uint64",
			Range: &Range{Name: "-42..forty-two"},
		},
		err: `unknown: bad range: strconv.ParseUint: parsing "forty-two": invalid syntax`,
	}, {
		desc: "basic string with a length",
		in: &Type{
			Name:   "string",
			Length: &Length{Name: "24..42"},
		},
		out: &YangType{
			Name:   "string",
			Kind:   Ystring,
			Length: YangRange{{Min: FromInt(24), Max: FromInt(42)}},
		},
	}, {
		desc: "basic string with an invalid range",
		in: &Type{
			Name:   "string",
			Length: &Length{Name: "-42..42"},
		},
		err: `unknown: bad length: -42..42 not within 0..18446744073709551615`,
	}, {
		desc: "basic binary with a length",
		in: &Type{
			Name:   "binary",
			Length: &Length{Name: "24..42"},
		},
		out: &YangType{
			Name:   "binary",
			Kind:   Ybinary,
			Length: YangRange{{Min: FromInt(24), Max: FromInt(42)}},
		},
	}, {
		desc: "basic binary with an unparseable range",
		in: &Type{
			Name:   "binary",
			Length: &Length{Name: "42..forty-two"},
		},
		err: `unknown: bad length: strconv.ParseUint: parsing "forty-two": invalid syntax`,
	}, {
		desc: "invalid fraction-digits argument for boolean value",
		in: &Type{
			Name:           "boolean",
			FractionDigits: &Value{Name: "42"},
		},
		err: "unknown: fraction-digits only allowed for decimal64 values",
	}, {
		desc: "required field fraction-digits not supplied for decimal64",
		in: &Type{
			Name: "decimal64",
		},
		err: "unknown: value is required in the range of [1..18]",
	}, {
		desc: "invalid identityref that doesn't have a base identity name",
		in: &Type{
			Name: "identityref",
		},
		err: "unknown: an identityref must specify a base",
	}, {
		desc: "invalid decimal64 having an invalid fraction-digits value",
		in: &Type{
			Name:           "decimal64",
			FractionDigits: &Value{Name: "42"},
		},
		err: "unknown: value 42 out of range [1..18]",
	}, {
		desc: "decimal64",
		in: &Type{
			Name:           "decimal64",
			FractionDigits: &Value{Name: "7"},
		},
		out: &YangType{
			Name:           "decimal64",
			Kind:           Ydecimal64,
			FractionDigits: 7,
			Range:          YangRange{Rf(MinInt64, MaxInt64, 7)},
		},
	}, {
		desc: "instance-identifier with unspecified require-instance value (default true)",
		in: &Type{
			Name:            "instance-identifier",
			RequireInstance: nil,
		},
		out: &YangType{
			Name: "instance-identifier",
			Kind: YinstanceIdentifier,
			// https://tools.ietf.org/html/rfc7950#section-9.9.3
			// require-instance defaults to true.
			OptionalInstance: false,
		},
	}, {
		desc: "instance-identifier with true require-instance value",
		in: &Type{
			Name:            "instance-identifier",
			RequireInstance: &Value{Name: "true"},
		},
		out: &YangType{
			Name:             "instance-identifier",
			Kind:             YinstanceIdentifier,
			OptionalInstance: false,
		},
	}, {
		desc: "instance-identifier with false require-instance value",
		in: &Type{
			Name:            "instance-identifier",
			RequireInstance: &Value{Name: "false"},
		},
		out: &YangType{
			Name:             "instance-identifier",
			Kind:             YinstanceIdentifier,
			OptionalInstance: true,
		},
	}, {
		desc: "instance-identifier with invalid require-instance value",
		in: &Type{
			Name:            "instance-identifier",
			RequireInstance: &Value{Name: "foo"},
		},
		err: "invalid boolean: foo",
	}, {
		desc: "enum with unspecified values",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY"},
				{Name: "VENUS"},
				{Name: "EARTH"},
			},
		},
		out: &YangType{
			Name: "enumeration",
			Kind: Yenum,
			Enum: &EnumType{
				last:   2,
				min:    MinEnum,
				max:    MaxEnum,
				unique: true,
				ToString: map[int64]string{
					0: "MERCURY",
					1: "VENUS",
					2: "EARTH",
				},
				ToInt: map[string]int64{
					"MERCURY": 0,
					"VENUS":   1,
					"EARTH":   2,
				},
			},
		},
	}, {
		desc: "enum with specified values",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-1"}},
				{Name: "VENUS", Value: &Value{Name: "10"}},
				{Name: "EARTH", Value: &Value{Name: "30"}},
			},
		},
		out: &YangType{
			Name: "enumeration",
			Kind: Yenum,
			Enum: &EnumType{
				last:   30,
				min:    MinEnum,
				max:    MaxEnum,
				unique: true,
				ToString: map[int64]string{
					-1: "MERCURY",
					10: "VENUS",
					30: "EARTH",
				},
				ToInt: map[string]int64{
					"MERCURY": -1,
					"VENUS":   10,
					"EARTH":   30,
				},
			},
		},
	}, {
		desc: "enum with some values specified",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-1"}},
				{Name: "VENUS", Value: &Value{Name: "10"}},
				{Name: "EARTH"},
			},
		},
		out: &YangType{
			Name: "enumeration",
			Kind: Yenum,
			Enum: &EnumType{
				last:   11,
				min:    MinEnum,
				max:    MaxEnum,
				unique: true,
				ToString: map[int64]string{
					-1: "MERCURY",
					10: "VENUS",
					11: "EARTH",
				},
				ToInt: map[string]int64{
					"MERCURY": -1,
					"VENUS":   10,
					"EARTH":   11,
				},
			},
		},
	}, {
		desc: "enum with repeated specified values",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "1"}},
				{Name: "VENUS", Value: &Value{Name: "10"}},
				{Name: "EARTH", Value: &Value{Name: "1"}},
			},
		},
		err: "unknown: fields EARTH and MERCURY conflict on value 1",
	}, {
		desc: "enum with repeated specified names",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-1"}},
				{Name: "VENUS", Value: &Value{Name: "10"}},
				{Name: "MERCURY", Value: &Value{Name: "30"}},
			},
		},
		err: "unknown: field MERCURY already assigned",
	}, {
		desc: "enum with last specified value equal to the max enum value",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-2147483648"}},
				{Name: "VENUS", Value: &Value{Name: "2147483647"}},
				{Name: "EARTH"},
			},
		},
		err: `unknown: enum "EARTH" must specify a value since previous enum is the maximum value allowed`,
	}, {
		desc: "enum value too small",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-2147483649"}},
				{Name: "VENUS", Value: &Value{Name: "0"}},
				{Name: "EARTH"},
			},
		},
		err: `unknown: value -2147483649 for MERCURY too small (minimum is -2147483648)`,
	}, {
		desc: "enum value too large",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-2147483648"}},
				{Name: "VENUS", Value: &Value{Name: "2147483648"}},
				{Name: "EARTH"},
			},
		},
		err: `unknown: value 2147483648 for VENUS too large (maximum is 2147483647)`,
	}, {
		desc: "enum with an unparseable value",
		in: &Type{
			Name: "enumeration",
			Enum: []*Enum{
				{Name: "MERCURY", Value: &Value{Name: "-1"}},
				{Name: "VENUS", Value: &Value{Name: "10"}},
				{Name: "EARTH", Value: &Value{Name: "five"}},
			},
		},
		err: `unknown: strconv.ParseUint: parsing "five": invalid syntax`,
		// TODO(borman): Add in more tests as we honor more fields
		// in Type.
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// We can initialize a value to ourself, so to it here.
			errs := tt.in.resolve(newTypeDictionary())

			// TODO(borman):  Do not hack out Root and Base.  These
			// are hacked out for now because they can be self-referential,
			// making construction of them difficult.
			tt.in.YangType.Root = nil
			tt.in.YangType.Base = nil

			switch {
			case tt.err == "" && len(errs) > 0:
				t.Fatalf("unexpected errors: %v", errs)
			case tt.err != "" && len(errs) == 0:
				t.Fatalf("did not get expected errors: %v", tt.err)
			case len(errs) > 1:
				t.Fatalf("too many errors: %v", errs)
			case len(errs) == 1 && errs[0].Error() != tt.err:
				t.Fatalf("got error %v, want %s", errs[0], tt.err)
			case len(errs) != 0:
				return
			}

			if diff := cmp.Diff(tt.in.YangType, tt.out); diff != "" {
				t.Errorf("YangType (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestTypedefResolve(t *testing.T) {
	tests := []struct {
		desc string
		in   *Typedef
		err  string
		out  *YangType
	}{{
		desc: "basic int64",
		in: &Typedef{
			Name:    "time",
			Parent:  baseTypes["int64"].typedef(),
			Default: &Value{Name: "42"},
			Type: &Type{
				Name: "int64",
			},
			Units: &Value{Name: "nanoseconds"},
		},
		out: &YangType{
			Name: "time",
			Kind: Yint64,
			Base: &Type{
				Name: "int64",
			},
			Units:      "nanoseconds",
			Default:    "42",
			HasDefault: true,
			Range:      Int64Range,
		},
	}, {
		desc: "uint32 with more specific range",
		in: &Typedef{
			Name: "another-counter",
			Parent: &Typedef{
				Name:   "counter",
				Parent: baseTypes["uint32"].typedef(),
				Type: &Type{
					Name:  "uint32",
					Range: &Range{Name: "0..42"},
				},
			},
			Type: &Type{
				Name:  "uint32",
				Range: &Range{Name: "10..20"},
			},
		},
		out: &YangType{
			Name: "another-counter",
			Kind: Yuint32,
			Base: &Type{
				Name: "uint32",
			},
			Range: YangRange{{Min: FromInt(10), Max: FromInt(20)}},
		},
		// TODO(wenovus): Add tests on range and length inheritance once those are fixed.
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// We can initialize a value to ourself, so to it here.
			errs := tt.in.resolve(newTypeDictionary())

			switch {
			case tt.err == "" && len(errs) > 0:
				t.Fatalf("unexpected errors: %v", errs)
			case tt.err != "" && len(errs) == 0:
				t.Fatalf("did not get expected errors: %v", tt.err)
			case len(errs) > 1:
				t.Fatalf("too many errors: %v", errs)
			case len(errs) == 1 && errs[0].Error() != tt.err:
				t.Fatalf("got error %v, want %s", errs[0], tt.err)
			case len(errs) != 0:
				return
			}

			if diff := cmp.Diff(tt.in.YangType, tt.out); diff != "" {
				t.Errorf("YangType (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestTypeResolveUnions(t *testing.T) {
	tests := []struct {
		desc          string
		leafNode      string
		wantType      *testEnumTypeStruct
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
		wantType: &testEnumTypeStruct{
			Name: "alpha",
			Type: []*testEnumTypeStruct{{
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
		wantType: &testEnumTypeStruct{
			Name: "alpha",
			Type: []*testEnumTypeStruct{{
				Name: "string",
			}, {
				Name: "uint32",
			}, {
				Name:  "enumeration",
				ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
			}, {
				Name: "bravo",
				Type: []*testEnumTypeStruct{{
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
		wantType: &testEnumTypeStruct{
			Name: "alpha",
			Type: []*testEnumTypeStruct{{
				Name: "union",
				Type: []*testEnumTypeStruct{{
					Name: "uint32",
				}, {
					Name: "string",
				}, {
					Name:  "enumeration",
					ToInt: map[string]int64{"one": 1, "seven": 7, "zero": 0},
				}},
			}, {
				Name: "bravo",
				Type: []*testEnumTypeStruct{{
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
		wantType: &testEnumTypeStruct{
			Name: "union",
			Type: []*testEnumTypeStruct{{
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
		wantType: &testEnumTypeStruct{
			Name: "alpha",
			Type: []*testEnumTypeStruct{{
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
		wantType: &testEnumTypeStruct{
			Name: "union",
			Type: []*testEnumTypeStruct{{
				Name: "alpha",
				Type: []*testEnumTypeStruct{{
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
		wantType: &testEnumTypeStruct{
			Name: "union",
			Type: []*testEnumTypeStruct{{
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
				Type: []*testEnumTypeStruct{{
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
					Type: []*testEnumTypeStruct{{
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
		const module = "test"
		m, ok := ms.Modules[module]
		if !ok {
			return nil, fmt.Errorf("can't find module %q", module)
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

type testEnumTypeStruct struct {
	Name string
	// ToInt is the ToInt map representing the enum value (if present).
	ToInt map[string]int64
	Type  []*testEnumTypeStruct
}

// filterTypeNames returns a testEnumTypeStruct with only the
// YangType.Name fields of the given type, preserving
// the recursive structure of the type, to work around cmp not
// having an allowlist way of specifying which fields to
// compare and YangType having a custom Equal function.
func filterTypeNames(ytype *YangType) *testEnumTypeStruct {
	filteredNames := &testEnumTypeStruct{Name: ytype.Name}
	if ytype.Enum != nil {
		filteredNames.ToInt = ytype.Enum.ToInt
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
		const module = "test"
		m, ok := ms.Modules[module]
		if !ok {
			return nil, fmt.Errorf("can't find module %q", module)
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
		wantType      *testRangeTypeStruct
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
		wantType: &testRangeTypeStruct{
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
		wantType: &testRangeTypeStruct{
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
		desc: "unrestricted decimal64",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 2;
				}
			}
			leaf test-leaf {
				type alpha;
			}
		} // end module`,
		wantType: &testRangeTypeStruct{
			Name:  "alpha",
			Range: YangRange{Rf(MinInt64, MaxInt64, 2)},
		},
	}, {
		desc: "simple restricted decimal64",
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
		wantType: &testRangeTypeStruct{
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
		wantType: &testRangeTypeStruct{
			Name:  "bravo",
			Range: YangRange{Rf(1000, 2720, 3), Rf(42000, MaxInt64, 3)},
		},
	}, {
		desc: "triple-inherited decimal64",
		leafNode: `
			typedef alpha {
				type decimal64 {
					fraction-digits 2;
				}
			}
			typedef bravo {
				type alpha {
					range "1 .. 3.14 | 10 | 20..max";
				}
			}
			typedef charlie {
				type bravo {
					range "min .. 2.72 | 42 .. max";
				}
			}
			leaf test-leaf {
				type charlie;
			}
		} // end module`,
		wantType: &testRangeTypeStruct{
			Name:  "charlie",
			Range: YangRange{Rf(100, 272, 2), Rf(4200, MaxInt64, 2)},
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
		wantType: &testRangeTypeStruct{
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
		wantType: &testRangeTypeStruct{
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
		wantType: &testRangeTypeStruct{
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
		wantType: &testRangeTypeStruct{
			Name: "alpha",
			Type: []*testRangeTypeStruct{{
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
		const moduleName = "test"
		m, ok := ms.Modules[moduleName]
		if !ok {
			return nil, fmt.Errorf("module not found: %q", moduleName)
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

// testRangeTypeStruct is a filtered-down version of YangType where only certain
// fields are preserved for targeted testing.
type testRangeTypeStruct struct {
	Name   string
	Length YangRange
	Range  YangRange
	Type   []*testRangeTypeStruct
}

// filterRanges returns a testRangeTypeStruct with only the Name, Length, and Range
// fields of the given YangType, preserving the recursive structure of the
// type, to work around cmp not having an allowlist way of specifying which
// fields to compare and YangType having a custom Equal function.
func filterRanges(ytype *YangType) *testRangeTypeStruct {
	filteredType := &testRangeTypeStruct{Name: ytype.Name}
	filteredType.Length = ytype.Length
	filteredType.Range = ytype.Range
	for _, subtype := range ytype.Type {
		filteredType.Type = append(filteredType.Type, filterRanges(subtype))
	}
	return filteredType
}
