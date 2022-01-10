// Copyright 2016 Google Inc.
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
	"fmt"
	"sync"
)

// This file implements data structures and functions that relate to the
// identity type.

// identityDictionary stores a set of identities across all parsed Modules that
// have been resolved to be identified by their module and name.
type identityDictionary struct {
	mu sync.Mutex
	// dict is a global cache of identities keyed by
	// modulename:identityname, where modulename is the full name of the
	// module to which the identity belongs. If the identity were defined
	// in a submodule, then the parent module name is used instead.
	dict map[string]resolvedIdentity
}

// resolvedIdentity is an Identity that has been disambiguated.
type resolvedIdentity struct {
	Module   *Module
	Identity *Identity
}

// isEmpty determines whether the resolvedIdentity struct value is populated.
func (r resolvedIdentity) isEmpty() bool {
	return r.Module == nil && r.Identity == nil
}

// newResolvedIdentity creates a resolved identity from an identity and its
// associated module, and returns the prefixed name (Prefix:IdentityName)
// along with the resolved identity.
func newResolvedIdentity(m *Module, i *Identity) (string, *resolvedIdentity) {
	r := &resolvedIdentity{
		Module:   m,
		Identity: i,
	}
	return i.modulePrefixedName(), r
}

func appendIfNotIn(ids []*Identity, chk *Identity) []*Identity {
	for _, id := range ids {
		if id == chk {
			return ids
		}
	}
	return append(ids, chk)
}

// addChildren adds identity r and all of its children to ids
// deterministically.
func addChildren(r *Identity, ids []*Identity) []*Identity {
	ids = appendIfNotIn(ids, r)

	// Iterate through the values of r.
	for _, ch := range r.Values {
		ids = addChildren(ch, ids)
	}
	return ids
}

// findIdentityBase returns the resolved identity that is corresponds to the
// baseStr string in the context of the module/submodule mod.
func (mod *Module) findIdentityBase(baseStr string) (*resolvedIdentity, []error) {
	var base resolvedIdentity
	var ok bool
	var errs []error

	basePrefix, baseName := getPrefix(baseStr)
	rootPrefix := mod.GetPrefix()
	source := Source(mod)
	typeDict := mod.Modules.typeDict

	switch basePrefix {
	case "", rootPrefix:
		// This is a local identity which is defined within the current
		// module
		keyName := fmt.Sprintf("%s:%s", module(mod).Name, baseName)
		base, ok = typeDict.identities.dict[keyName]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: can't resolve the local base %s as %s", source, baseStr, keyName))
		}
	default:
		// This is an identity which is defined within another module
		extmod := FindModuleByPrefix(mod, basePrefix)
		if extmod == nil {
			errs = append(errs,
				fmt.Errorf("%s: can't find external module with prefix %s", source, basePrefix))
			break
		}
		// The identity we are looking for is modulename:basename.
		if id, ok := typeDict.identities.dict[fmt.Sprintf("%s:%s", module(extmod).Name, baseName)]; ok {
			base = id
			break
		}

		// Error if we did not find the identity that had the name specified in
		// the module it was expected to be in.
		if base.isEmpty() {
			errs = append(errs, fmt.Errorf("%s: can't resolve remote base %s", source, baseStr))
		}
	}
	return &base, errs
}

func (ms *Modules) resolveIdentities() []error {
	defer ms.typeDict.identities.mu.Unlock()
	ms.typeDict.identities.mu.Lock()

	var errs []error

	// Across all modules, read the identity values that have been extracted
	// from them, and compile them into a "fully resolved" map that means that
	// we can look them up based on the 'real' prefix of the module and the
	// name of the identity.
	for _, mod := range ms.Modules {
		for _, i := range mod.Identities() {
			keyName, r := newResolvedIdentity(mod, i)
			ms.typeDict.identities.dict[keyName] = *r
		}

		// Hoist up all identities in our included submodules.
		// We could just do a range on ms.SubModules, but that
		// might process a submodule that no module included.
		for _, in := range mod.Include {
			if in.Module == nil {
				continue
			}
			for _, i := range in.Module.Identities() {
				keyName, r := newResolvedIdentity(in.Module, i)
				ms.typeDict.identities.dict[keyName] = *r
			}
		}
	}

	// Determine which identities have a base statement, and link this to a
	// fully resolved identity statement. The intention here is to make sure
	// that the Children slice is fully populated with pointers to all identities
	// that have a base, so that we can do inheritance of these later.
	for _, i := range ms.typeDict.identities.dict {
		if i.Identity.Base != nil {
			// This identity inherits from one or more other identities.

			root := RootNode(i.Identity)
			for _, b := range i.Identity.Base {
				base, baseErr := root.findIdentityBase(b.asString())

				if baseErr != nil {
					errs = append(errs, baseErr...)
					continue
				}

				// Build up a list of direct children of this identity.
				base.Identity.Values = append(base.Identity.Values, i.Identity)
			}
		}
	}

	// Do a final sweep through the identities to build up their children.
	for _, i := range ms.typeDict.identities.dict {
		newValues := []*Identity{}
		for _, j := range i.Identity.Values {
			newValues = addChildren(j, newValues)
		}
		i.Identity.Values = newValues
	}

	return errs
}
