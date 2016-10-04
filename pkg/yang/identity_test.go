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

import "testing"

type modIn struct {
	name    string
	content string
}

type basicIdentityModOut struct {
	name       string
	identities []*identityOut
	modules    []string
	idrefs     []*idrefOut
}

type idrefOut struct {
	module string
	name   string
	values []string
}

type identityOut struct {
	idName   string
	baseName string
	values   []string
}

type basicIdentityTestCase struct {
	name string
	in   []*modIn
	out  *basicIdentityModOut
	err  string
}

// A set of test modules to parse as part of the test case.
var basicTestCases = []*basicIdentityTestCase{
	&basicIdentityTestCase{
		in: []*modIn{
			&modIn{
				name: "idtest-one",
				content: `
module idtest-one {
  namespace "urn:idone";
  prefix "idone";

  identity TEST_ID;
}
    `},
		},
		out: &basicIdentityModOut{
			name: "idtest-one",
			identities: []*identityOut{
				&identityOut{
					idName: "TEST_ID",
				},
			},
		},
		err: "tc1: could not resolve identities",
	},
	&basicIdentityTestCase{
		in: []*modIn{
			&modIn{
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
		out: &basicIdentityModOut{
			name: "idtest-two",
			identities: []*identityOut{
				&identityOut{
					idName: "TEST_ID",
				},
				&identityOut{
					idName: "TEST_ID_TWO",
				},
				&identityOut{
					idName:   "TEST_CHILD",
					baseName: "TEST_ID",
				},
			},
		},
		err: "could not resolve identities",
	},
}

func TestIdentityExtract(t *testing.T) {
	for _, testCase := range basicTestCases {
		ms := NewModules()
		for _, mod := range testCase.in {
			_ = ms.Parse(mod.content, mod.name)
		}
		parsedMod, err := ms.GetModule(testCase.out.name)

		if err != nil {
			t.Errorf("could not parse module: %s", testCase.out.name)
			continue
		}

		identityNames := make(map[string]*Identity)
		for _, id := range parsedMod.Identities {
			identityNames[id.Name] = id
		}

		for _, expID := range testCase.out.identities {
			identity, ok := identityNames[expID.idName]

			if !ok {
				t.Errorf("%s: couldn't find %s", testCase.err, expID.idName)
				continue
			}

			if expID.baseName != "" {
				if expID.baseName != identity.Base.Name {
					t.Errorf("%s: couldn't resolve expected base %s", testCase.err,
						expID.baseName)
				}
			} else {
				if identity.Base != nil {
					t.Errorf("%s: identity had an unexpected base %s: %v", testCase.err,
						identity.Name, identity.Base)
				}
			}
		}
	}
}

var treeTestCases = []*basicIdentityTestCase{
	&basicIdentityTestCase{
		in: []*modIn{
			&modIn{
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
			&modIn{
				name: "remote.yang",
				content: `
  module remote {
    namespace "urn:remote";
    prefix "remote";

    identity REMOTE_BASE;
  }
      `},
		},
		out: &basicIdentityModOut{
			modules: []string{"remote", "base"},
			identities: []*identityOut{
				&identityOut{
					idName: "REMOTE_BASE",
					values: []string{"LOCAL_REMOTE_BASE"},
				},
				&identityOut{
					idName:   "LOCAL_REMOTE_BASE",
					baseName: "r:REMOTE_BASE",
				},
			},
		},
	},
	&basicIdentityTestCase{
		in: []*modIn{
			&modIn{
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
			base "GRANDFATHER";
		}
  }
        `},
		},
		out: &basicIdentityModOut{
			modules: []string{"base"},
			identities: []*identityOut{
				&identityOut{
					idName: "GREATGRANDFATHER",
					values: []string{"GRANDFATHER", "FATHER", "UNCLE", "SON", "BROTHER"},
				},
				&identityOut{
					idName:   "GRANDFATHER",
					baseName: "GREATGRANDFATHER",
					values:   []string{"FATHER", "UNCLE", "SON", "BROTHER"},
				},
				&identityOut{
					idName:   "FATHER",
					baseName: "GRANDFATHER",
					values:   []string{"SON", "BROTHER"},
				},
				&identityOut{
					idName:   "UNCLE",
					baseName: "GRANDFATHER",
				},
				&identityOut{
					idName:   "SON",
					baseName: "FATHER",
				},
				&identityOut{
					idName:   "BROTHER",
					baseName: "FATHER",
				},
			},
		},
	},
	&basicIdentityTestCase{
		in: []*modIn{
			&modIn{
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
		out: &basicIdentityModOut{
			modules: []string{"base"},
			identities: []*identityOut{
				&identityOut{
					idName: "BASE",
				},
				&identityOut{
					idName:   "NOTBASE",
					baseName: "BASE",
				},
			},
			idrefs: []*idrefOut{
				&idrefOut{
					module: "base",
					name:   "idref",
					values: []string{"NOTBASE"},
				},
			},
		},
	},
}

// TestIdentityTree - check inheritance of identities from local and remote
// sources. The Values of an Identity correspond to the values that are
// referenced by that identity, which need to be inherited.
func TestIdentityTree(t *testing.T) {
	for _, testCase := range treeTestCases {
		ms := NewModules()

		for _, mod := range testCase.in {
			_ = ms.Parse(mod.content, mod.name)
		}

		if errs := ms.Process(); len(errs) != 0 {
			t.Errorf("couldn't process modules: %v", errs)
			continue
		}

		identityValues := make(map[string][]string)
		var foundIdentities []*Identity

		if testCase.out == nil {
			continue
		}

		// Go through and find all the identities and values for later comparison
		for _, mn := range testCase.out.modules {
			m, errs := ms.GetModule(mn)
			if errs != nil {
				t.Errorf("couldn't find expected module: %v", errs)
			}
			for _, i := range m.Identities {
				foundIdentities = append(foundIdentities, i)
				identityValues[i.Name] = make([]string, 0)
				for _, j := range i.Values {
					identityValues[i.Name] = append(identityValues[i.Name], j.Name)
				}
			}
		}

		// For each of the test cases, go through and compare whether we can find
		// the identity, and the relevant base if one exists.
		for _, tc := range testCase.out.identities {
			nMatch := false
			//vMatch := false
			bMatch := 0
			if tc.baseName != "" {
				bMatch = -1
			}

			for _, fid := range foundIdentities {
				if fid.Name == tc.idName {
					nMatch = true
					if bMatch < 0 && fid.Base != nil {
						if fid.Base.Name == tc.baseName {
							bMatch = 1
						}
					}
					if tc.values != nil {
						// Need to check values - create a map keyed by value with a bool
						// as the value, we set this to true when we find the value, and
						// then check for any values that are false.
						rem := make(map[string]bool)
						for _, v := range tc.values {
							rem[v] = false
						}

						for _, v := range fid.Values {
							if _, ok := rem[v.Name]; ok {
								rem[v.Name] = true
							}
						}

						for k, v := range rem {
							if v == false {
								t.Errorf("unable to find value %s for %s", k, tc.idName)
							}
						}
					}
				}
			}

			if nMatch == false {
				t.Errorf("couldn't find identity %s", tc.idName)
			}
			if bMatch < 0 {
				t.Errorf("couldn't find identity %s base %s", tc.idName, tc.baseName)
			}
		}

		// Check the identityrefs that we have been asked to check and test
		// whether the identity pointer is set up correctly.
		if testCase.out.idrefs != nil {
			for _, tidr := range testCase.out.idrefs {
				mod, errs := ms.GetModule(tidr.module)
				if errs != nil {
					t.Errorf("can't find module %s for idref %s", tidr.module, tidr.name)
					continue
				}
				if leaf, ok := mod.Dir[tidr.name]; ok {
					tMap := make(map[string]bool)
					for _, v := range tidr.values {
						tMap[v] = false
					}

					if leaf.Type == nil || leaf.Type.IdentityBase == nil ||
						leaf.Type.IdentityBase.Values == nil {
						t.Errorf("identityref leaf %s was not properly formed", tidr.name)
					}

					// Check whether the identityref leaf had an Identity within the
					// Values that corresponds to the one that we were asked to retrieve
					// within the test data.
					for _, v := range leaf.Type.IdentityBase.Values {
						if _, ok := tMap[v.Name]; ok {
							tMap[v.Name] = true
						} else {
							t.Errorf("couldn't find identity value %s in base identity", v)
						}
					}

					// Check whether GetValue returns the defined value
					for _, k := range tidr.values {
						if v := leaf.Type.IdentityBase.GetValue(k); v == nil {
							t.Errorf("couldn't retrieve identity value %s with GetValue from %s",
								k, leaf.Type.IdentityBase.Name)
						}
					}

					// Check whether IsDefined returns the right result.
					for _, k := range tidr.values {
						if c := leaf.Type.IdentityBase.IsDefined(k); !c {
							t.Errorf("couldn't retrieve identity value %s with IsDefiend from %s",
								k, leaf.Type.IdentityBase.Name)
						}
					}

					// If any entries are false in the tMap, this means that it did not
					// match when we walked through the values that are defined.
					for k, v := range tMap {
						if v == false {
							t.Errorf("couldn't find identity value %s from base identity of %s",
								k, tidr.name)
						}
					}

				} else {
					t.Errorf("couldn't find identityref leaf %s in module %s", tidr.module,
						tidr.name)
				}
			}
		}
	}
}
