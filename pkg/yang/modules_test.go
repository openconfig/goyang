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

package yang_test

import (
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
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

func testModulesForTestdataModulesText(t *testing.T) *yang.Modules {
	ms := yang.NewModules()
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

func testModulesFindByCommonHandler(t *testing.T, i int, got, want *yang.Module, wantError string, err error) {
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

func TestModulesFindByPrefix(t *testing.T) {
	ms := testModulesForTestdataModulesText(t)

	for i, tc := range []struct {
		prefix    string
		want      *yang.Module
		wantError string
	}{
		{
			prefix:    "does-not-exist",
			wantError: "does-not-exist: no such prefix",
		},
		{
			prefix: "foo",
			want:   ms.Modules["foo"],
		},
		{
			prefix: "bar",
			want:   ms.Modules["bar"],
		},
		{
			prefix: "baz",
			want:   ms.Modules["baz"],
		},
		{
			prefix:    "duplicate",
			wantError: "prefix duplicate matches two or more modules (dup-pre-",
		},
	} {
		got, err := ms.FindModuleByPrefix(tc.prefix)
		testModulesFindByCommonHandler(t, i, got, tc.want, tc.wantError, err)
	}
}

func TestModulesFindByNamespace(t *testing.T) {
	ms := testModulesForTestdataModulesText(t)

	for i, tc := range []struct {
		namespace string
		want      *yang.Module
		wantError string
	}{
		{
			namespace: "does-not-exist",
			wantError: "does-not-exist: no such namespace",
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
