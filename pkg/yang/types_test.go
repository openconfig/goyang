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
		err: "unknown: negative length: -42..42",
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
			Range:          Decimal64Range,
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
				toString: map[int64]string{
					-1: "MERCURY",
					10: "VENUS",
					30: "EARTH",
				},
				toInt: map[string]int64{
					"MERCURY": -1,
					"VENUS":   10,
					"EARTH":   30,
				},
			},
		},
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
				last:   30,
				min:    MinEnum,
				max:    MaxEnum,
				unique: true,
				toString: map[int64]string{
					1: "MERCURY",
					2: "VENUS",
					3: "EARTH",
				},
				toInt: map[string]int64{
					"MERCURY": 1,
					"VENUS":   2,
					"EARTH":   3,
				},
			},
		},
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
			errs := tt.in.resolve()

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
			Units:   "nanoseconds",
			Default: "42",
			Range:   Int64Range,
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
			errs := tt.in.resolve()

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
