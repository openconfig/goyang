// Copyright 2016 Google Inc.
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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/errdiff"
)

// inputModule is a mock input YANG module.
type inputModule struct {
	name    string // The filename of the YANG module.
	content string // The contents of the YANG module.
}

type idrefOut struct {
	module string   // The module that the identityref is within.
	name   string   // The name of the identityref.
	values []string // Names of the identities that the identityref relates to.
}

// identityOut is the output for a particular identity within the test case.
type identityOut struct {
	module    string   // The module that the identity is within.
	name      string   // The name of the identity.
	baseNames []string // The base(s) of the identity as string(s).
	values    []string // The string names of derived identities.
}

// identityTestCase is a test case for a module which contains identities.
type identityTestCase struct {
	name          string
	in            []inputModule // The set of input modules for the test
	identities    []identityOut // Slice of the identity values expected
	idrefs        []idrefOut    // Slice of identityref results expected
	wantErrSubstr string        // wanErrSubstr is a substring of the wanted error.
}

// getBaseNamesFrom is a utility function for getting the base name(s) of an identity
func getBaseNamesFrom(i *Identity) []string {
	baseNames := []string{}
	for _, base := range i.Base {
		baseNames = append(baseNames, base.Name)
	}
	return baseNames
}

// Test cases for basic identity extraction.
var basicTestCases = []identityTestCase{
	{
		name: "basic-test-case-1: Check identity is found in module.",
		in: []inputModule{
			{
				name: "idtest-one",
				content: `
					module idtest-one {
					  namespace "urn:idone";
					  prefix "idone";

					  identity TEST_ID;
					}
				`},
		},
		identities: []identityOut{
			{module: "idtest-one", name: "TEST_ID"},
		},
	},
	{
		name: "basic-test-case-2: Check identity with base is found in module.",
		in: []inputModule{
			{
				name: "idtest-two",
				content: `
					module idtest-two {
					  namespace "urn:idtwo";
					  prefix "idone";

					  identity TEST_ID;
					  identity TEST_ID_TWO;
					  identity TEST_CHILD {
					    base TEST_ID;
					  }
					}
				`},
		},
		identities: []identityOut{
			{module: "idtest-two", name: "TEST_ID"},
			{module: "idtest-two", name: "TEST_ID_TWO"},
			{module: "idtest-two", name: "TEST_CHILD", baseNames: []string{"TEST_ID"}},
		},
	},
	{
		name: "basic-test-case-3: Check identity with multiple bases.",
		in: []inputModule{
			{
				name: "idtest-three",
				content: `
					module idtest-three {
					  namespace "urn:idthree";
					  prefix "idthree";

					  identity BASE_ONE;
					  identity BASE_TWO;
					  identity TEST_CHILD_WITH_MULTIPLE_BASES {
						base BASE_ONE;
						base BASE_TWO;
					  }
					}
				`},
		},
		identities: []identityOut{
			{module: "idtest-three", name: "BASE_ONE"},
			{module: "idtest-three", name: "BASE_TWO"},
			{module: "idtest-three", name: "TEST_CHILD_WITH_MULTIPLE_BASES", baseNames: []string{"BASE_ONE", "BASE_TWO"}},
		},
	},
	{
		name: "basic-test-case-4: Check identity base is found from submodule.",
		in: []inputModule{
			{
				name: "idtest-one",
				content: `
					module idtest-one {
					  namespace "urn:idone";
					  prefix "idone";

					  include "idtest-one-sub";

					  identity TEST_ID_DERIVED {
					    base TEST_ID;
					  }
					}
				`},
			{
				name: "idtest-one-sub",
				content: `
					submodule idtest-one-sub {
					  belongs-to idtest-one {
					    prefix "idone";
					  }

					  identity TEST_ID;
					}
				`},
		},
		identities: []identityOut{
			{module: "idtest-one", name: "TEST_ID"},
			{module: "idtest-one", name: "TEST_ID_DERIVED", baseNames: []string{"TEST_ID"}},
		},
	},
	{
		name: "basic-test-case-5: Check identity base is found from module.",
		in: []inputModule{
			{
				name: "idtest-one",
				content: `
					module idtest-one {
					  namespace "urn:idone";
					  prefix "idone";

					  include "idtest-one-sub";

					  identity TEST_ID;
					}
				`},
			{
				name: "idtest-one-sub",
				content: `
					submodule idtest-one-sub {
					  belongs-to idtest-one {
					    prefix "idone";
					  }

					  identity TEST_ID_DERIVED {
					    base TEST_ID;
					  }
					}
				`},
		},
		identities: []identityOut{
			{module: "idtest-one", name: "TEST_ID_DERIVED", baseNames: []string{"TEST_ID"}},
			{module: "idtest-one", name: "TEST_ID"},
		},
	},
}

// Test the ability to extract identities from a module with the correct base
// statements.
func TestIdentityExtract(t *testing.T) {
	for _, tt := range basicTestCases {
		ms := NewModules()
		for _, mod := range tt.in {
			_ = ms.Parse(mod.content, mod.name)
		}

		for _, ti := range tt.identities {
			_, err := ms.GetModule(ti.module)

			if err != nil {
				t.Errorf("Could not parse module : %s", ti.module)
				continue
			}

			foundIdentity := false
			var thisID *Identity
			for _, ri := range ms.typeDict.identities.dict {
				moduleName := module(ri.Module).Name
				if ri.Identity.Name == ti.name && moduleName == ti.module {
					foundIdentity = true
					thisID = ri.Identity
					break
				}
			}

			if !foundIdentity {
				t.Errorf("Could not find identity %s in module %s, identity dict:\n%+v", ti.name, ti.module, ms.typeDict.identities.dict)
				continue
			}

			actualBaseNames := getBaseNamesFrom(thisID)
			if len(ti.baseNames) > 0 {
				if diff := cmp.Diff(actualBaseNames, ti.baseNames); diff != "" {
					t.Errorf("(-got, +want):\n%s", diff)
				}
			} else {
				if thisID.Base != nil {
					t.Errorf("Identity %s had unexpected base(s) %s", thisID.Name,
						actualBaseNames)
				}
			}
		}
	}
}

// Test cases for validating that identities can be resolved correctly.
var treeTestCases = []identityTestCase{
	{
		name: "tree-test-case-0: Validate identity resolution across submodules",
		in: []inputModule{
			{
				name: "base.yang",
				content: `
				  module base {
				    namespace "urn:base";
				    prefix "base";

				    include side;

				    identity REMOTE_BASE;
				  }
				`},
			{
				name: "remote.yang",
				content: `
				  submodule side {
				    belongs-to base {
				      prefix "r";
				    }

				    identity LOCAL_REMOTE_BASE {
				      base r:REMOTE_BASE;
				    }
				  }
				`},
		},
		identities: []identityOut{
			{
				module: "base",
				name:   "REMOTE_BASE",
				values: []string{"LOCAL_REMOTE_BASE"},
			},
		},
	},
	{
		name: "tree-test-case-1: Validate identity resolution across modules",
		in: []inputModule{
			{
				name: "base.yang",
				content: `
				  module base {
				    namespace "urn:base";
				    prefix "base";

				    import remote { prefix "r"; }
				    import remote2 { prefix "r2"; }

				    identity LOCAL_REMOTE_BASE {
				      base r:REMOTE_BASE;
				    }

				    identity LOCAL_REMOTE_BASE2 {
				      base r2:REMOTE_BASE2;
				    }
				  }
				`},
			{
				name: "remote.yang",
				content: `
				  module remote {
				    namespace "urn:remote";
				    prefix "r";

				    identity REMOTE_BASE;
				  }
				`},
			{
				name: "remote2.yang",
				content: `
				  module remote2 {
				    namespace "urn:remote2";
				    prefix "remote";

				    identity REMOTE_BASE2;
				  }
				`},
		},
		identities: []identityOut{
			{
				module: "remote",
				name:   "REMOTE_BASE",
				values: []string{"LOCAL_REMOTE_BASE"},
			},
			{
				module: "remote2",
				name:   "REMOTE_BASE2",
				values: []string{"LOCAL_REMOTE_BASE2"},
			},
			{
				module:    "base",
				name:      "LOCAL_REMOTE_BASE",
				baseNames: []string{"r:REMOTE_BASE"},
			},
			{
				module:    "base",
				name:      "LOCAL_REMOTE_BASE2",
				baseNames: []string{"r2:REMOTE_BASE2"},
			},
		},
	},
	{
		name: "tree-test-case-2: Multi-level inheritance validation.",
		in: []inputModule{
			{
				name: "base.yang",
				content: `
				  module base {
				    namespace "urn:base";
				    prefix "base";

				    identity GREATGRANDFATHER;
						identity GRANDFATHER {
							base "GREATGRANDFATHER";
						}
						identity FATHER {
							base "GRANDFATHER";
						}
						identity SON {
							base "FATHER";
						}
						identity UNCLE {
							base "GRANDFATHER";
						}
						identity BROTHER {
							base "FATHER";
						}
						identity GREATUNCLE {
							base "GREATGRANDFATHER";
						}
				  }
				`},
		},
		identities: []identityOut{
			{
				module: "base",
				name:   "GREATGRANDFATHER",
				values: []string{
					"BROTHER", // Order is alphabetical
					"FATHER",
					"GRANDFATHER",
					"GREATUNCLE",
					"SON",
					"UNCLE",
				},
			},
			{
				module:    "base",
				name:      "GRANDFATHER",
				baseNames: []string{"GREATGRANDFATHER"},
				values:    []string{"BROTHER", "FATHER", "SON", "UNCLE"},
			},
			{
				module:    "base",
				name:      "GREATUNCLE",
				baseNames: []string{"GREATGRANDFATHER"},
			},
			{
				module:    "base",
				name:      "FATHER",
				baseNames: []string{"GRANDFATHER"},
				values:    []string{"BROTHER", "SON"},
			},
			{
				module:    "base",
				name:      "UNCLE",
				baseNames: []string{"GRANDFATHER"},
			},
			{
				module:    "base",
				name:      "BROTHER",
				baseNames: []string{"FATHER"},
			},
		},
	},
	{
		name: "tree-test-case-3",
		in: []inputModule{
			{
				name: "base.yang",
				content: `
				  module base {
				    namespace "urn:base";
				    prefix "base";

						identity BASE;
						identity NOTBASE {
							base BASE;
						}

						leaf idref {
							type identityref {
								base "BASE";
							}
						}
				  }
				`},
		},
		identities: []identityOut{
			{
				module: "base",
				name:   "BASE",
				values: []string{"NOTBASE"},
			},
			{
				module:    "base",
				name:      "NOTBASE",
				baseNames: []string{"BASE"},
			},
		},
		idrefs: []idrefOut{
			{
				module: "base",
				name:   "idref",
				values: []string{"NOTBASE"},
			},
		},
	},
	{
		name: "tree-test-case-4",
		in: []inputModule{
			{
				name: "base.yang",
				content: `
				  module base4 {
				    namespace "urn:base";
				    prefix "base4";

						identity BASE4;
						identity CHILD4 {
							base BASE4;
						}

						typedef t {
							type identityref {
								base BASE4;
							}
						}

						leaf tref {
							type t;
						}
				  }
				`},
		},
		identities: []identityOut{
			{
				module: "base4",
				name:   "BASE4",
				values: []string{"CHILD4"},
			},
			{
				module:    "base4",
				name:      "CHILD4",
				baseNames: []string{"BASE4"},
			},
		},
		idrefs: []idrefOut{
			{
				module: "base4",
				name:   "tref",
				values: []string{"CHILD4"},
			},
		},
	},
	{
		name: "tree-test-case-5",
		in: []inputModule{
			{
				name: "base.yang",
				content: `
					module base5 {
						namespace "urn:base";
						prefix "base5";

						identity BASE5A;
						identity BASE5B;

						identity FIVE_ONE {
							base BASE5A;
						}

						identity FIVE_TWO {
							base BASE5B;
						}

						leaf union {
							type union {
								type identityref {
									base BASE5A;
								}
								type identityref {
									base BASE5B;
								}
							}
						}
					}`},
		},
		identities: []identityOut{
			{
				module: "base5",
				name:   "BASE5A",
				values: []string{"FIVE_ONE"},
			},
			{
				module: "base5",
				name:   "BASE5B",
				values: []string{"FIVE_TWO"},
			},
		},
		idrefs: []idrefOut{
			{
				module: "base5",
				name:   "union",
				values: []string{"FIVE_ONE", "FIVE_TWO"},
			},
		},
	},
	{
		name: "identity's base can't be found",
		in: []inputModule{
			{
				name: "idtest",
				content: `
					module idtest{
					  namespace "urn:idtwo";
					  prefix "idone";

					  identity TEST_ID_TWO;
					  identity TEST_CHILD {
					    base TEST_ID;
					  }
					}
				`},
		},
		identities: []identityOut{
			{module: "idtest", name: "TEST_ID2"},
		},
		wantErrSubstr: "can't resolve the local base",
	},
	{
		name: "identity's base can't be found in remote",
		in: []inputModule{
			{
				name: "remote.yang",
				content: `
				  module remote {
				    namespace "urn:remote";
				    prefix "remote";

				    identity REMOTE_BASE_ESCAPE;
				  }
				`},
			{
				name: "base.yang",
				content: `
				  module base {
				    namespace "urn:base";
				    prefix "base";

				    import remote { prefix "r"; }

				    identity LOCAL_REMOTE_BASE {
				      base r:REMOTE_BASE;
				    }
				  }
				`},
		},
		identities: []identityOut{
			{module: "base", name: "LOCAL_REMOTE_BASE"},
		},
		wantErrSubstr: "can't resolve remote base",
	},
	{
		name: "identity's base's module can't be found",
		in: []inputModule{
			{
				name: "remote.yang",
				content: `
				  module remote {
				    namespace "urn:remote";
				    prefix "remote";

				    identity REMOTE_BASE;
				  }
				`},
			{
				name: "base.yang",
				content: `
				  module base {
				    namespace "urn:base";
				    prefix "base";

				    import remote { prefix "r"; }

				    identity LOCAL_REMOTE_BASE {
				      base roe:REMOTE_BASE;
				    }
				  }
				`},
		},
		identities: []identityOut{
			{module: "base", name: "LOCAL_REMOTE_BASE"},
		},
		wantErrSubstr: "can't find external module",
	},
}

// TestIdentityTree - check inheritance of identities from local and remote
// sources. The Values of an Identity correspond to the values that are
// referenced by that identity, which need to be inherited.
func TestIdentityTree(t *testing.T) {
	for _, tt := range treeTestCases {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewModules()

			for _, mod := range tt.in {
				_ = ms.Parse(mod.content, mod.name)
			}

			errs := ms.Process()

			var err error
			switch len(errs) {
			case 1:
				err = errs[0]
				if diff := errdiff.Substring(err, tt.wantErrSubstr); diff != "" {
					t.Fatalf("%s", diff)
				}
				return
			case 0:
				if diff := errdiff.Substring(err, tt.wantErrSubstr); diff != "" {
					t.Fatalf("%s", diff)
				}
			default:
				t.Fatalf("got multiple errors: %v", errs)
			}

			// Walk through the identities that are defined in the test case output
			// and validate that they exist, and their base and values are as expected.
			for _, chkID := range tt.identities {
				m, errs := ms.GetModule(chkID.module)
				if errs != nil {
					t.Errorf("Couldn't find expected module: %v", errs)
					continue
				}

				var foundID *Identity
				for _, i := range m.Identities {
					if i.Name == chkID.name {
						foundID = i
						break
					}
				}

				if foundID == nil {
					t.Errorf("Couldn't find identity %s in module %s", chkID.name,
						chkID.module)
					continue
				}

				if len(chkID.baseNames) > 0 {
					actualBaseNames := getBaseNamesFrom(foundID)
					if diff := cmp.Diff(actualBaseNames, chkID.baseNames); diff != "" {
						t.Errorf("(-got, +want):\n%s", diff)
					}
				}

				valueMap := make(map[string]bool)

				for i, val := range chkID.values {
					valueMap[val] = false
					// Check that IsDefined returns the right result
					if !foundID.IsDefined(val) {
						t.Errorf("Couldn't find defined value %s  for %s", val, chkID.name)
					}

					// Check that the values are sorted in a consistent order
					if foundID.Values[i].Name != val {
						t.Errorf("Invalid order for value #%d. Expecting %s Got %s", i, foundID.Values[i].Name, val)
					}
					// Check that GetValue returns the right Identity
					idval := foundID.GetValue(val)
					if idval == nil {
						t.Errorf("Couldn't GetValue(%s) for %s", val, chkID.name)
					}
				}

				// Ensure that IsDefined does not return false positives
				if foundID.IsDefined("DoesNotExist") {
					t.Errorf("Non-existent value IsDefined for %s", foundID.Name)
				}

				if foundID.GetValue("DoesNotExist") != nil {
					t.Errorf("Non-existent value GetValue not nil for %s", foundID.Name)
				}

				for _, chkv := range foundID.Values {
					_, ok := valueMap[chkv.Name]
					if !ok {
						t.Errorf("Found unexpected value %s for %s", chkv.Name, chkID.name)
						continue
					}
					valueMap[chkv.Name] = true
				}

				for k, v := range valueMap {
					if v == false {
						t.Errorf("Could not find identity %s for %s", k, chkID.name)
					}
				}
			}

			for _, idr := range tt.idrefs {
				m, errs := ms.GetModule(idr.module)
				if errs != nil {
					t.Errorf("Couldn't find expected module %s: %v", idr.module, errs)
					continue
				}

				if _, ok := m.Dir[idr.name]; !ok {
					t.Errorf("Could not find expected identity, got: nil, want: %v", idr.name)
					continue
				}

				identity := m.Dir[idr.name]
				var vals []*Identity
				switch len(identity.Type.Type) {
				case 0:
					vals = identity.Type.IdentityBase.Values
				default:
					for _, b := range identity.Type.Type {
						if b.IdentityBase != nil {
							vals = append(vals, b.IdentityBase.Values...)
						}
					}
				}

				var valNames []string
				for _, v := range vals {
					valNames = append(valNames, v.Name)
				}

				if diff := cmp.Diff(idr.values, valNames); diff != "" {
					t.Errorf("Identity %s did not have expected values, (-got, +want):\n%s", idr.name, diff)
				}
			}
		})
	}
}
