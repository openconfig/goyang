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
	"reflect"
	"testing"
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
	module   string   // The module that the identity is within.
	name     string   // The name of the identity.
	baseName string   // The base of the identity as a string.
	values   []string // The string names of derived identities.
}

// identityTestCase is a test case for a module which contains identities.
type identityTestCase struct {
	name       string
	in         []inputModule // The set of input modules for the test
	identities []identityOut // Slice of the identity values expected
	idrefs     []idrefOut    // Slice of identityref results expected
	err        string        // Test case error string
}

// Test cases for basic identity extraction.
var basicTestCases = []identityTestCase{
	identityTestCase{
		name: "basic-test-case-1: Check identity is found in module.",
		in: []inputModule{
			inputModule{
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
			identityOut{module: "idtest-one", name: "TEST_ID"},
		},
		err: "basic-test-case-1: could not resolve identities",
	},
	identityTestCase{
		name: "basic-test-case-2: Check identity with base is found in module.",
		in: []inputModule{
			inputModule{
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
			identityOut{module: "idtest-two", name: "TEST_ID"},
			identityOut{module: "idtest-two", name: "TEST_ID_TWO"},
			identityOut{module: "idtest-two", name: "TEST_CHILD", baseName: "TEST_ID"},
		},
		err: "basic-test-case-2: could not resolve identities",
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
			parsedMod, err := ms.GetModule(ti.module)

			if err != nil {
				t.Errorf("Could not parse module : %s", ti.module)
				continue
			}

			foundIdentity := false
			var thisID *Identity
			for _, identity := range parsedMod.Identities {
				if identity.Name == ti.name {
					foundIdentity = true
					thisID = identity
					break
				}
			}

			if foundIdentity == false {
				t.Errorf("Could not found identity %s in %s", ti.name, ti.module)
			}

			if ti.baseName != "" {
				if ti.baseName != thisID.Base.Name {
					t.Errorf("Identity %s did not have expected base %s, had %s", ti.name,
						ti.baseName, thisID.Base.Name)
				}
			} else {
				if thisID.Base != nil {
					t.Errorf("Identity %s had an unexpected base %s", thisID.Name,
						thisID.Base.Name)
				}
			}
		}
	}
}

// Test cases for validating that identities can be resolved correctly.
var treeTestCases = []identityTestCase{
	identityTestCase{
		name: "tree-test-case-1: Validate identity resolution across modules",
		in: []inputModule{
			inputModule{
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
			inputModule{
				name: "remote.yang",
				content: `
				  module remote {
				    namespace "urn:remote";
				    prefix "remote";

				    identity REMOTE_BASE;
				  }
				`},
		},
		identities: []identityOut{
			identityOut{
				module: "remote",
				name:   "REMOTE_BASE",
				values: []string{"LOCAL_REMOTE_BASE"},
			},
			identityOut{
				module:   "base",
				name:     "LOCAL_REMOTE_BASE",
				baseName: "r:REMOTE_BASE",
			},
		},
	},
	identityTestCase{
		name: "tree-test-case-2: Multi-level inheritance validation.",
		in: []inputModule{
			inputModule{
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
			identityOut{
				module: "base",
				name:   "GREATGRANDFATHER",
				values: []string{
					"GRANDFATHER",
					"GREATUNCLE",
					"FATHER",
					"UNCLE",
					"SON",
					"BROTHER",
				},
			},
			identityOut{
				module:   "base",
				name:     "GRANDFATHER",
				baseName: "GREATGRANDFATHER",
				values:   []string{"FATHER", "UNCLE", "SON", "BROTHER"},
			},
			identityOut{
				module:   "base",
				name:     "GREATUNCLE",
				baseName: "GREATGRANDFATHER",
			},
			identityOut{
				module:   "base",
				name:     "FATHER",
				baseName: "GRANDFATHER",
				values:   []string{"SON", "BROTHER"},
			},
			identityOut{
				module:   "base",
				name:     "UNCLE",
				baseName: "GRANDFATHER",
			},
			identityOut{
				module:   "base",
				name:     "BROTHER",
				baseName: "FATHER",
			},
		},
	},
	identityTestCase{
		in: []inputModule{
			inputModule{
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
			identityOut{
				module: "base",
				name:   "BASE",
				values: []string{"NOTBASE"},
			},
			identityOut{
				module:   "base",
				name:     "NOTBASE",
				baseName: "BASE",
			},
		},
		idrefs: []idrefOut{
			idrefOut{
				module: "base",
				name:   "idref",
				values: []string{"NOTBASE"},
			},
		},
	},
	identityTestCase{
		in: []inputModule{
			inputModule{
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
			identityOut{
				module: "base4",
				name:   "BASE4",
				values: []string{"CHILD4"},
			},
			identityOut{
				module:   "base4",
				name:     "CHILD4",
				baseName: "BASE4",
			},
		},
		idrefs: []idrefOut{
			idrefOut{
				module: "base4",
				name:   "tref",
				values: []string{"CHILD4"},
			},
		},
	},
	identityTestCase{
		in: []inputModule{
			inputModule{
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
			identityOut{
				module: "base5",
				name:   "BASE5A",
				values: []string{"FIVE_ONE"},
			},
			identityOut{
				module: "base5",
				name:   "BASE5B",
				values: []string{"FIVE_TWO"},
			},
		},
		idrefs: []idrefOut{
			idrefOut{
				module: "base5",
				name:   "union",
				values: []string{"FIVE_ONE", "FIVE_TWO"},
			},
		},
	},
}

// TestIdentityTree - check inheritance of identities from local and remote
// sources. The Values of an Identity correspond to the values that are
// referenced by that identity, which need to be inherited.
func TestIdentityTree(t *testing.T) {
	for _, tt := range treeTestCases {
		ms := NewModules()

		for _, mod := range tt.in {
			_ = ms.Parse(mod.content, mod.name)
		}

		if errs := ms.Process(); len(errs) != 0 {
			t.Errorf("Couldn't process modules: %v", errs)
			continue
		}

		// Walk through the identities that are defined in the test case output
		// and validate that they exist, and their base and values are as expected.
		for _, chkID := range tt.identities {
			m, errs := ms.GetModule(chkID.module)
			if errs != nil {
				t.Errorf("Couldn't find expected module: %v", errs)
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
			}

			if chkID.baseName != "" {
				if chkID.baseName != foundID.Base.Name {
					t.Errorf("Couldn't find base %s for ID %s", chkID.baseName,
						foundID.Base.Name)
				}
			}

			valueMap := make(map[string]bool)

			for _, val := range chkID.values {
				valueMap[val] = false
				// Check that IsDefined returns the right result
				if !foundID.IsDefined(val) {
					t.Errorf("Couldn't find defined value %s  for %s", val, chkID.name)
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

			if !reflect.DeepEqual(idr.values, valNames) {
				t.Errorf("Identity %s did not have expected values, got: %v, want: %v", idr.name, valNames, idr.values)
			}
		}
	}
}
