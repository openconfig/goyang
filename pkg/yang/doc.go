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

// Package yang is used to parse .yang files (see RFC 6020).
//
// A generic yang statments takes one of the forms:
//
//    keyword [argument] ;
//    keyword [argument] { [statement [...]] }
//
// At the lowest level, package yang returns a simple tree of statements via the
// Parse function.  The Parse function makes no attempt to determine the
// validity of the source, other than checking for generic syntax errors.
//
// At it's simplest, the GetModule function is used.  The GetModule function
// searches the current directory, and any directory added to the Path variable,
// for a matching .yang source file by appending .yang to the name of the
// module:
//
//	// Get the tree for the module module-name by looking for the source
//	// file named module-name.yang.
//	e, errs := yang.GetModule("module-name" [, optional sources...])
//	if len(errs) > 0 {
//		for _, err := range errs {
//			fmt.Fprintln(os.Stderr, err)
//		}
//		os.Exit(1)
//	}
//
//	// e is the Entry tree for "module-name"
//
//
// More complicated uses cases should use NewModules and then some combination
// of Modules.GetModule, Modules.Read, Modules.Parse, and Modules.GetErrors.
//
// The GetErrors method is mandatory, however, both yang.GetModule and
// Modules.GetModule automatically call Modules.GetErrors.
package yang
