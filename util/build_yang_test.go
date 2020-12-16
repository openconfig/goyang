package util

import "testing"

// TestProcessModules tests the ProcessModules function - which takes an input
// set of modules and processes them using the goyang compiler into a set of
// yang.Entry pointers.
func TestProcessModules(t *testing.T) {
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
	}}

	for _, tt := range tests {
		entries, errs := ProcessModules(tt.inFiles, tt.inPath)
		if len(errs) != 0 && !tt.wantErr {
			t.Errorf("%s: unexpected error processing modules: %v", tt.name, errs)
			continue
		}

		for _, m := range tt.wantMods {
			if _, ok := entries[m]; !ok {
				t.Errorf("%s: could not find module %s", tt.name, m)
			}
		}
	}
}
