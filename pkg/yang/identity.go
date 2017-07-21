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

// identityDictionary stores a global set of identities that have been resolved
// to be identified by their module and name.
type identityDictionary struct {
	mu   sync.Mutex
	dict map[string]resolvedIdentity
}

// Global dictionary of resolved identities
var identities = identityDictionary{dict: map[string]resolvedIdentity{}}

// resolvedIdentity is an Identity that has been disambiguated.
type resolvedIdentity struct {
	Module   *Module
	Identity *Identity
}

// isEmpty determines whether the resolvedIdentity struct value was defined.
func (r resolvedIdentity) isEmpty() bool {
	return r.Module == nil && r.Identity == nil
}

// newResolvedIdentity creates a resolved identity from an identity and its
// associated value, and returns the prefixed name (Prefix:IdentityName)
// along with the resolved identity.
func newResolvedIdentity(m *Module, i *Identity) (string, *resolvedIdentity) {
	r := &resolvedIdentity{
		Module:   m,
		Identity: i,
	}
	return i.PrefixedName(), r
}

func appendIfNotIn(ids []*Identity, chk *Identity) []*Identity {
	for _, id := range ids {
		if id == chk {
			return ids
		}
	}
	return append(ids, chk)
}

// addChildren recursively adds the identity r to ids.
func addChildren(r *Identity, ids []*Identity) []*Identity {
	ids = appendIfNotIn(ids, r)

	// Iterate through the values of r.
	for _, ch := range r.Values {
		ids = addChildren(ch, ids)
	}
	return ids
}

// findIdentityBase returns the resolved identity that is corresponds to the
// baseStr string in the context of the Module mod.
func (mod *Module) findIdentityBase(baseStr string) (*resolvedIdentity, []error) {
	var base resolvedIdentity
	var ok bool
	var errs []error

	basePrefix, baseName := getPrefix(baseStr)
	rootPrefix := mod.GetPrefix()
	source := Source(mod)

	switch basePrefix {
	case "", rootPrefix:
		// This is a local identity which is defined within the current
		// module
		keyName := fmt.Sprintf("%s:%s", rootPrefix, baseName)
		base, ok = identities.dict[keyName]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: can't resolve the local base %s as %s", source, baseStr, keyName))
		}
		break
	default:
		// The identity we are looking for is prefix:basename.  If
		// we already know prefix:basename then just use it.  If not,
		// try again within the module identified by prefix.
		if id, ok := identities.dict[baseStr]; ok {
			base = id
			break
		}
		// This is an identity which is defined within another module
		extmod := FindModuleByPrefix(mod, basePrefix)
		if extmod == nil {
			errs = append(errs,
				fmt.Errorf("%s: can't find external module with prefix %s", source, basePrefix))
			break
		}

		// Run through the identities within the remote module and find the
		// one that matches the base that we have been specified.
		for _, rid := range extmod.Identities() {
			if rid.Name == baseName {
				key := rid.PrefixedName()
				if id, ok := identities.dict[key]; ok {
					base = id
				} else {
					errs = append(errs, fmt.Errorf("%s: can't find base %s", source, baseStr))
				}
				break
			}
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
	defer identities.mu.Unlock()
	identities.mu.Lock()

	var errs []error

	// Across all modules, read the identity values that have been extracted
	// from them, and compile them into a "fully resolved" map that means that
	// we can look them up based on the 'real' prefix of the module and the
	// name of the identity.
	for _, mod := range ms.Modules {
		for _, i := range mod.Identities() {
			keyName, r := newResolvedIdentity(mod, i)
			identities.dict[keyName] = *r
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
				identities.dict[keyName] = *r
			}
		}
	}

	// Determine which identities have a base statement, and link this to a
	// fully resolved identity statement. The intention here is to make sure
	// that the Children slice is fully populated with pointers to all identities
	// that have a base, so that we can do inheritance of these later.
	for _, i := range identities.dict {
		if i.Identity.Base != nil {
			// This identity inherits from another identity.

			root := RootNode(i.Identity)
			base, baseErr := root.findIdentityBase(i.Identity.Base.asString())

			if baseErr != nil {
				errs = append(errs, baseErr...)
				continue
			}

			// Append this value to the children of the base identity.
			base.Identity.Values = append(base.Identity.Values, i.Identity)
		}
	}

	// Do a final sweep through the identities to build up their children.
	for _, i := range identities.dict {
		newValues := []*Identity{}
		for _, j := range i.Identity.Values {
			newValues = addChildren(j, newValues)
		}
		i.Identity.Values = newValues
	}

	return errs
}
