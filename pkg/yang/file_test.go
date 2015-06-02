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
	"reflect"
	"testing"
)

func TestFindFile(t *testing.T) {
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
			check: []string{"four.yang", "dir1/four.yang", "dir2/four.yang"},
		},
	} {
		var checked []string
		Path = tt.path
		readFile = func(path string) ([]byte, error) {
			checked = append(checked, path)
			return nil, errors.New("no such file")
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
