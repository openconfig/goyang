// Copyright 2017 Google Inc.
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

// Options defines the options that should be used when parsing YANG modules,
// including specific overrides for potentially problematic YANG constructs.
type Options struct {
	// IgnoreSubmoduleCircularDependencies specifies whether circular dependencies
	// between submodules. Setting this value to true will ensure that this
	// package will explicitly ignore the case where a submodule will include
	// itself through a circular reference.
	IgnoreSubmoduleCircularDependencies bool
	// StoreUses controls whether the Uses field of each YANG entry should be
	// populated. Setting this value to true will cause each Entry which is
	// generated within the schema to store the logical grouping from which it
	// is derived.
	StoreUses bool
}
