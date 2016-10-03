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

type modOut struct {
	name       string
	identities []*idOut
}

type idOut struct {
	idName   string
	baseName string
}

type testCase struct {
	name string
	in   []*modIn
	out  *modOut
	err  string
}

// A set of test modules to parse as part of the test case.
var testCases = []*testCase{
	&testCase{
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
		out: &modOut{
			name: "idtest-one",
			identities: []*idOut{
				&idOut{
					idName: "TEST_ID",
				},
			},
		},
		err: "tc1: could not resolve identities",
	},
	&testCase{
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
		out: &modOut{
			name: "idtest-two",
			identities: []*idOut{
				&idOut{
					idName: "TEST_ID",
				},
				&idOut{
					idName: "TEST_ID_TWO",
				},
				&idOut{
					idName:   "TEST_CHILD",
					baseName: "TEST_ID",
				},
			},
		},
		err: "could not resolve identities",
	},
}

func TestIdentityExtract(t *testing.T) {
	for _, testCase := range testCases {
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
