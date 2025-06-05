// Copyright 2020 Google Inc.
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

package yangentry

import (
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// TestParse tests the Parse function - which takes an input
// set of modules and processes them using the goyang compiler into a set of
// yang.Entry pointers.
func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		inFiles  []string
		inPath   []string
		wantErr  bool
		wantMods []string
	}{{
		name:     "simple valid module",
		inFiles:  []string{"testdata/00-valid-module.yang"},
		inPath:   []string{"testdata"},
		wantMods: []string{"test-module"},
	}, {
		name:     "simple valid module without .yang extension",
		inFiles:  []string{"00-valid-module"},
		inPath:   []string{"testdata"},
		wantMods: []string{"test-module"},
	}, {
		name:    "simple invalid module",
		inFiles: []string{"testdata/01-invalid-module.yang"},
		inPath:  []string{"testdata"},
		wantErr: true,
	}, {
		name:     "valid import",
		inFiles:  []string{"testdata/02-valid-import.yang"},
		inPath:   []string{"testdata/subdir"},
		wantMods: []string{"test-module"},
	}, {
		name:    "invalid import",
		inFiles: []string{"testdata/03-invalid-import.yang"},
		inPath:  []string{},
		wantErr: true,
	}, {
		name:     "two modules",
		inFiles:  []string{"testdata/04-valid-module-one.yang", "testdata/04-valid-module-two.yang"},
		inPath:   []string{},
		wantMods: []string{"module-one", "module-two"},
	}, {
		name:    "circular submodule dependency",
		inFiles: []string{"testdata/05-circular-main.yang"},
		inPath:  []string{"testdata/subdir"},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, errs := Parse(tt.inFiles, tt.inPath)
			if len(errs) != 0 && !tt.wantErr {
				t.Fatalf("%s: unexpected error processing modules: %v", tt.name, errs)
			}

			for _, m := range tt.wantMods {
				if _, ok := entries[m]; !ok {
					t.Fatalf("%s: could not find module %s", tt.name, m)
				}
			}
		})
	}
}

// TestParseWithOptions tests the ParseWithOptions function - which takes an input
// set of modules along with a set of parse options, and processes them using the goyang
// compiler into a set of yang.Entry pointers.
func TestParseWithOptions(t *testing.T) {
	tests := []struct {
		name         string
		inFiles      []string
		inPath       []string
		parseOptions yang.Options
		wantErr      bool
		wantMods     []string
	}{
		{
			name:         "circular submodule dependency with default options",
			inFiles:      []string{"testdata/05-circular-main.yang"},
			inPath:       []string{"testdata/subdir"},
			parseOptions: yang.Options{},
			wantErr:      true,
		},
		{
			name:         "circular submodule dependency with IgnoreSubmoduleCircularDependencies",
			inFiles:      []string{"testdata/05-circular-main.yang"},
			inPath:       []string{"testdata/subdir"},
			parseOptions: yang.Options{IgnoreSubmoduleCircularDependencies: true},
			wantMods:     []string{"circular-main"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, errs := ParseWithOptions(tt.inFiles, tt.inPath, tt.parseOptions)
			if len(errs) != 0 && !tt.wantErr {
				t.Fatalf("%s: unexpected error processing modules: %v", tt.name, errs)
			}

			for _, m := range tt.wantMods {
				if _, ok := entries[m]; !ok {
					t.Fatalf("%s: could not find module %s", tt.name, m)
				}
			}
		})
	}
}
