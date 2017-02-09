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
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testPathReset() {
	Path = []string{}
	pathMap = map[string]bool{}
}

func TestFindFile(t *testing.T) {
	// clean up global state
	defer testPathReset()

	sep := string(os.PathSeparator)

	for _, tt := range []struct {
		name  string
		path  []string
		check []string
	}{
		{
			name:  "one",
			check: []string{"one.yang"},
		},
		{
			name:  "./two",
			check: []string{"./two"},
		},
		{
			name:  "three.yang",
			check: []string{"three.yang"},
		},
		{
			name:  "four",
			path:  []string{"dir1", "dir2"},
			check: []string{"four.yang", "dir1" + sep + "four.yang", "dir2" + sep + "four.yang"},
		},
	} {
		var checked []string
		Path = tt.path
		readFile = func(path string) ([]byte, error) {
			checked = append(checked, path)
			return nil, errors.New("no such file")
		}
		scanDir = func(dir, name string, recurse bool) string {
			return filepath.Join(dir, name)
		}
		if _, _, err := findFile(tt.name); err == nil {
			t.Errorf("%s unexpectedly succeeded", tt.name)
			continue
		}
		if !reflect.DeepEqual(tt.check, checked) {
			t.Errorf("%s: got %v, want %v", tt.name, checked, tt.check)
		}
	}
}

func TestScanForPathsAndAddModules(t *testing.T) {
	// clean up global state
	defer testPathReset()

	// disable any readFile mock setup by other tests
	readFile = ioutil.ReadFile

	// Scan the directory tree for YANG modules
	paths, err := PathsWithModules("../../testdata")
	if err != nil {
		t.Fatal(err)
	}
	// we should have seen two directories being testdata and
	// testdata/subdir.
	if len(paths) != 2 {
		t.Errorf("got %d paths imported, want 2", len(paths))
	}
	// add the paths found in the scan to the module path
	AddPath(paths...)

	// confirm we can load the four modules that exist in
	// the two paths we scanned.
	modules := []string{"aug", "base", "other", "subdir1"}
	ms := NewModules()
	for _, name := range modules {
		if _, err := ms.GetModule(name); err != nil {
			t.Errorf("getting %s: %v", name, err)
		}
	}

	// however, a sub module is not a valid argument to GetModule.
	if _, err := ms.GetModule("sub"); err == nil {
		t.Error("want an error when loading 'sub', got nil")
	}

}
