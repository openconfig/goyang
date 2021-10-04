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
	"strings"
	"testing"

	"github.com/openconfig/gnmi/errdiff"
)

var testdataFindModulesText = map[string]string{
	"foo":         `module foo { prefix "foo"; namespace "urn:foo"; }`,
	"bar":         `module bar { prefix "bar"; namespace "urn:bar"; }`,
	"baz":         `module baz { prefix "baz"; namespace "urn:baz"; }`,
	"dup-pre-one": `module dup-pre-one { prefix duplicate; namespace urn:duplicate:one; }`,
	"dup-pre-two": `module dup-pre-two { prefix duplicate; namespace urn:duplicate:two; }`,
	"dup-ns-one":  `module dup-ns-one { prefix ns-one; namespace urn:duplicate; }`,
	"dup-ns-two":  `module dup-ns-two { prefix ns-two; namespace urn:duplicate; }`,
}

func TestDupModule(t *testing.T) {
	tests := []struct {
		desc      string
		inModules map[string]string
		wantErr   bool
	}{{
		desc: "two modules with the same name",
		inModules: map[string]string{
			"foo": `module foo { prefix "foo"; namespace "urn:foo"; }`,
			"bar": `module foo { prefix "foo"; namespace "urn:foo"; }`,
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()
			var err error
			for name, modtext := range tt.inModules {
				if err = ms.Parse(modtext, name+".yang"); err != nil {
					break
				}
			}
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("wantErr: %v, got error: %v", tt.wantErr, err)
			}
		})
	}
}

func testModulesForTestdataModulesText(t *testing.T) *Modules {
	ms := NewModules()
	for name, modtext := range testdataFindModulesText {
		if err := ms.Parse(modtext, name+".yang"); err != nil {
			t.Fatalf("error importing testdataFindModulesText[%q]: %v", name, err)
		}
	}
	if errs := ms.Process(); errs != nil {
		for _, err := range errs {
			t.Errorf("error: %v", err)
		}
		t.Fatalf("fatal error(s) calling Process()")
	}
	return ms
}

func testModulesFindByCommonHandler(t *testing.T, i int, got, want *Module, wantError string, err error) {
	if err != nil {
		if wantError != "" {
			if !strings.Contains(err.Error(), wantError) {
				t.Errorf("[%d] want error containing %q, got %q",
					i, wantError, err.Error())
			}
		} else {
			t.Errorf("[%d] unexpected error: %v", i, err)
		}
	} else if wantError != "" {
		t.Errorf("[%d] want error containing %q, got nil", i, wantError)
	} else if want != got {
		t.Errorf("[%d] want module %#v, got %#v", i, want, got)
	}
}

func TestModulesFindByNamespace(t *testing.T) {
	ms := testModulesForTestdataModulesText(t)

	for i, tc := range []struct {
		namespace string
		want      *Module
		wantError string
	}{
		{
			namespace: "does-not-exist",
			wantError: `"does-not-exist": no such namespace`,
		},
		{
			namespace: "urn:foo",
			want:      ms.Modules["foo"],
		},
		{
			namespace: "urn:bar",
			want:      ms.Modules["bar"],
		},
		{
			namespace: "urn:baz",
			want:      ms.Modules["baz"],
		},
		{
			namespace: "urn:duplicate",
			wantError: "namespace urn:duplicate matches two or more modules (dup-ns-",
		},
	} {
		got, err := ms.FindModuleByNamespace(tc.namespace)
		testModulesFindByCommonHandler(t, i, got, tc.want, tc.wantError, err)
	}
}

func TestModuleLinkage(t *testing.T) {
	tests := []struct {
		desc          string
		inMods        map[string]string
		wantErrSubstr string
	}{{
		desc: "invalid import",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					import sys { prefix sys; }

					revision 01-01-01 { description "the start of time"; }

					deviation /sys:sys/sys:hostname {
						deviate not-supported;
					}
				}`,
		},
		wantErrSubstr: "no such module",
	}, {
		desc: "valid include",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
		},
	}, {
		desc: "invalid include",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sysdb": `
				submodule sysdb {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
		},
		wantErrSubstr: "no such submodule",
	}, {
		desc: "valid include in submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}
					include sysdb;

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
			"sysdb": `
				submodule sysdb {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					container sysdb { leaf hostname { type string; } }
				}`,
		},
	}, {
		desc: "invalid include in submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}
					include sysdb;

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
			"syyysdb": `
				submodule syyysdb {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					container sysdb { leaf hostname { type string; } }
				}`,
		},
		wantErrSubstr: "no such submodule",
	}, {
		desc: "valid import in submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}
					import sysdb {
						prefix "sd";
					}

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
			"sysdb": `
				module sysdb {
					prefix sd;
					namespace "urn:sd";

					revision 01-01-01 { description "the start of time"; }

					container sysdb { leaf hostname { type string; } }
				}`,
		},
	}, {
		desc: "invalid import in submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}
					import sysdb {
						prefix "sd";
					}

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
			"syyysdb": `
				module syyysdb {
					prefix sd;
					namespace "urn:sd";

					revision 01-01-01 { description "the start of time"; }

					container sysdb { leaf hostname { type string; } }
				}`,
		},
		wantErrSubstr: "no such module",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()

			for n, m := range tt.inMods {
				if err := ms.Parse(m, n); err != nil {
					t.Fatalf("cannot parse module %s, err: %v", n, err)
				}
			}

			errs := ms.Process()
			var err error
			switch len(errs) {
			case 1:
				err = errs[0]
				fallthrough
			case 0:
				if diff := errdiff.Substring(err, tt.wantErrSubstr); diff != "" {
					t.Fatalf("%s", diff)
				}
			default:
				t.Fatalf("got multiple errors: %v", errs)
			}
		})
	}
}

func TestModulesTotalProcess(t *testing.T) {
	tests := []struct {
		desc    string
		inMods  map[string]string
		wantErr bool
	}{{
		desc: "import with deviation",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					import sys { prefix sys; }

					revision 01-01-01 { description "the start of time"; }

					deviation /sys:sys/sys:hostname {
						deviate not-supported;
					}
				}`,
			"sys": `
				module sys {
					prefix s;
					namespace "urn:s";

					revision 01-01-01 { description "the start of time"; }

					container sys { leaf hostname { type string; } }
				}`,
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()

			for n, m := range tt.inMods {
				if err := ms.Parse(m, n); err != nil {
					t.Fatalf("cannot parse module %s, err: %v", n, err)
				}
			}

			errs := ms.Process()
			switch {
			case len(errs) == 0 && tt.wantErr:
				t.Fatalf("did not get expected errors, got: %v, wantErr: %v", errs, tt.wantErr)
			case len(errs) != 0 && !tt.wantErr:
				t.Fatalf("got unexpected errors, got: %v, wantErr: %v", errs, tt.wantErr)
			}
		})
	}
}
