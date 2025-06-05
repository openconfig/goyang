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

// Package yangentry contains high-level helpers for using yang.Entry objects.
package yangentry

import (
	"fmt"

	"github.com/openconfig/goyang/pkg/yang"
)

// Parse takes a list of either module/submodule names or .yang file
// paths, and a list of include paths. It runs the yang parser on the YANG
// files by searching for them in the include paths or in the current
// directory, returning a slice of yang.Entry pointers which represent the
// parsed top level modules. It also returns a list of errors encountered while
// parsing, if any.
func Parse(yangfiles, path []string) (map[string]*yang.Entry, []error) {
	return parse(yangfiles, path, yang.NewModules())
}

// ParseWithOptions takes a list of either module/submodule names or .yang file
// paths, a list of include paths, and a set of parse options. It configures the
// yang parser with the specified parse options and runs it on the YANG
// files by searching for them in the include paths or in the current
// directory, returning a slice of yang.Entry pointers which represent the
// parsed top level modules. It also returns a list of errors encountered while
// parsing, if any.
func ParseWithOptions(yangfiles, path []string, parseOptions yang.Options) (map[string]*yang.Entry, []error) {
	ms := yang.NewModules()
	ms.ParseOptions = parseOptions

	return parse(yangfiles, path, ms)
}

func parse(yangfiles, path []string, ms *yang.Modules) (map[string]*yang.Entry, []error) {
	for _, p := range path {
		ms.AddPath(fmt.Sprintf("%s/...", p))
	}

	var processErr []error
	for _, name := range yangfiles {
		if name == "" {
			continue
		}
		if err := ms.Read(name); err != nil {
			processErr = append(processErr, err)
		}
	}

	if len(processErr) > 0 {
		return nil, processErr
	}

	if errs := ms.Process(); len(errs) != 0 {
		return nil, errs
	}

	entries := make(map[string]*yang.Entry)
	for _, m := range ms.Modules {
		e := yang.ToEntry(m)
		entries[e.Name] = e
	}

	return entries, nil
}
