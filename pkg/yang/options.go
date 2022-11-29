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
	// DeviateOptions contains options for how deviations are handled.
	DeviateOptions DeviateOptions
}

// DeviateOptions contains options for how deviations are handled.
type DeviateOptions struct {
	// IgnoreDeviateNotSupported indicates to the parser to retain nodes
	// that are marked with "deviate not-supported". An example use case is
	// where the user wants to interact with different targets that have
	// different support for a leaf without having to use a second instance
	// of an AST.
	IgnoreDeviateNotSupported bool
}

// IsDeviateOpt ensures that DeviateOptions satisfies the DeviateOpt interface.
func (DeviateOptions) IsDeviateOpt() {}

// DeviateOpt is an interface that can be used in function arguments.
type DeviateOpt interface {
	IsDeviateOpt()
}

func hasIgnoreDeviateNotSupported(opts []DeviateOpt) bool {
	for _, o := range opts {
		if opt, ok := o.(DeviateOptions); ok {
			return opt.IgnoreDeviateNotSupported
		}
	}
	return false
}
