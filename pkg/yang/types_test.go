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
	"sort"
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
		errs := tt.in.resolve()

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
		desc                string
		inGetFn             func(*Modules) (*YangType, error)
		leafNode            string
		wantPatternsRegular []string
		wantPatternsPosix   []string
		wantPosixErrSubstr  string
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
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type, nil
		},
		wantPatternsRegular: []string{"charlie"},
		wantPosixErrSubstr:  "expecting equal",
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
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type, nil
		},
		wantPatternsRegular: nil,
		wantPosixErrSubstr:  "expecting equal",
	}, {
		desc: "unequal number of patterns",
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
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type, nil
		},
		wantPatternsRegular: []string{"alpha"},
		wantPosixErrSubstr:  "expecting equal",
	}, {
		desc: "Equal number of patterns",
		leafNode: `
			leaf test-leaf {
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
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type, nil
		},
		wantPatternsRegular: []string{"alpha", "bravo", "charlie"},
		wantPatternsPosix:   []string{"delta", "echo", "foxtrot"},
	}, {
		desc: "No patterns",
		leafNode: `
			leaf test-leaf {
				type string;
			}
		}`,
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type, nil
		},
		wantPatternsRegular: nil,
		wantPatternsPosix:   nil,
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

						pattern 'charlie';
						o:bar 'goo';
						o:posix-pattern 'foxtrot';
					}
					type uint64;
				}
			}
		} // end module`,
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type.Type[0], nil
		},
		wantPatternsRegular: []string{"alpha", "bravo", "charlie"},
		wantPatternsPosix:   []string{"delta", "echo", "foxtrot"},
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
		inGetFn: func(ms *Modules) (*YangType, error) {
			m, err := ms.FindModuleByPrefix("t")
			if err != nil {
				return nil, fmt.Errorf("can't find module in %v", ms)
			}
			if len(m.Leaf) == 0 {
				return nil, fmt.Errorf("node %v is missing imports", m)
			}
			e := ToEntry(m)
			return e.Dir["test-leaf"].Type, nil
		},
		wantPatternsRegular: []string{"alpha", "bravo", "charlie"},
		wantPatternsPosix:   []string{"delta", "echo", "foxtrot"},
	}}

	for _, tt := range tests {
		for _, inUsePosixPatternExt := range []bool{false, true} {
			wantPatterns := tt.wantPatternsRegular
			patternType := "regular"
			if inUsePosixPatternExt {
				wantPatterns = tt.wantPatternsPosix
				patternType = "posix"
			}
			UsePosixPatternExt = inUsePosixPatternExt
			inModules := map[string]string{
				"test": `
				module test {
					prefix "t";
					namespace "urn:t";

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
			}

			t.Run(tt.desc+patternType, func(t *testing.T) {
				ms := NewModules()
				for n, m := range inModules {
					if err := ms.Parse(m, n); err != nil {
						t.Fatalf("error parsing module %s, got: %v, want: nil", n, err)
					}
				}
				errs := ms.Process()
				if inUsePosixPatternExt {
					var err error
					if len(errs) > 1 {
						t.Fatalf("Got more than 1 error: %v", errs)
					} else if len(errs) == 1 {
						err = errs[0]
					}
					if diff := errdiff.Substring(err, tt.wantPosixErrSubstr); diff != "" {
						t.Errorf("Did not get expected error: %s", diff)
					}
					if err != nil {
						return
					}
				} else if errs != nil {
					t.Fatal(errs)
				}

				yangType, err := tt.inGetFn(ms)
				if err != nil {
					t.Fatal(err)
				}

				gotPatterns := yangType.Pattern
				sort.Strings(gotPatterns)
				sort.Strings(wantPatterns)
				if diff := cmp.Diff(gotPatterns, wantPatterns, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("%s Pattern (-got, +want):\n%s", patternType, diff)
				}
			})
		}
	}
}
