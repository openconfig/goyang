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

// Package util contains goyang utility functions that could be useful for
// external users.
package util

import (
	"fmt"

	"github.com/openconfig/goyang/pkg/yang"
)

// ProcessModules takes a list of either .yang file or module/submodule names
// and a list of include paths, and runs the yang parser against them,
// returning a slice of yang.Entry pointers which represent the parsed top
// level modules.
func ProcessModules(yangfiles, path []string) (map[string]*yang.Entry, []error) {
	for _, p := range path {
		yang.AddPath(fmt.Sprintf("%s/...", p))
	}

	ms := yang.NewModules()

	var processErr []error
	for _, name := range yangfiles {
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
