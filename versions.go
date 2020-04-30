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

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
	"github.com/pborman/getopt"
)

func init() {
	flags := getopt.New()
	register(&formatter{
		name:              "oc-versions",
		f:                 doOcVersions,
		includeSubmodules: true,
		help:              "output files that describe a non-null schema",
		flags:             flags,
	})
}

func doOcVersions(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
		m, ok := e.Node.(*yang.Module)
		if !ok {
			fmt.Fprintf(os.Stderr, "error: cannot convert entry %q to *yang.Module", e.Name)
			continue
		}

		for _, e := range m.Extensions {
			keywordParts := strings.Split(e.Keyword, ":")
			if len(keywordParts) != 2 {
				// Unrecognized extension declaration
				continue
			}
			pfx, ext := strings.TrimSpace(keywordParts[0]), strings.TrimSpace(keywordParts[1])
			if ext == "openconfig-version" {
				extMod := yang.FindModuleByPrefix(m, pfx)
				if extMod == nil {
					fmt.Fprintf(os.Stderr, "unable to find module using prefix %q from referencing module %q\n", pfx, m.Name)
				} else if extMod.Name == "openconfig-extensions" {
					fmt.Fprintf(w, "%s.yang: openconfig-version:%q\n", m.Name, e.Argument)
				}
			}
		}

	}
}
