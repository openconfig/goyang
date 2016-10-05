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

type identityDictionary struct {
	mu   sync.Mutex
	dict map[string]*resolvedIdentity
}

// Global dictionary of resolved identities

var identityDict = identityDictionary{dict: map[string]*resolvedIdentity{}}

type resolvedIdentity struct {
	Module   *Module
	Identity *Identity
	//Children []*resolvedIdentity
}

func makeResolvedIdentity(m *Module, i *Identity) (string, *resolvedIdentity) {
	r := &resolvedIdentity{
		Module:   m,
		Identity: i,
	}
	prefix := RootNode(i).GetPrefix()
	keyName := fmt.Sprintf("%s:%s", prefix, i.Name)
	return keyName, r
}

func appendIfNotIn(ids []*Identity, chk *Identity) []*Identity {
	for _, id := range ids {
		if id == chk {
			return ids
		}
	}
	return append(ids, chk)
}

func learnChildren(r *Identity, ids []*Identity) []*Identity {
	// Learn this child
	ids = appendIfNotIn(ids, r)
	// But if it doesn't have any children, then return
	if r.Values == nil {
		return ids
	}
	// Else iterate through them
	for _, ch := range r.Values {
		ids = learnChildren(ch, ids)
	}
	return ids
}

func findIdentityBase(baseStr string, mod *Module) (*resolvedIdentity, []error) {
	var base *resolvedIdentity
	var ok bool
	var errs []error

	basePrefix, baseName := getPrefix(baseStr)
	rootPrefix := mod.GetPrefix()

	switch basePrefix {
	case "", rootPrefix:
		// This is a local identity which is defined within the current
		// module
		keyName := fmt.Sprintf("%s:%s", rootPrefix, baseName)
		base, ok = identityDict.dict[keyName]
		if !ok {
			errs = append(errs, fmt.Errorf("can't resolve the local base %s as %s", baseStr, keyName))
		}
	default:
		// This is an identity which is defined within another module
		externalModule := FindModuleByPrefix(mod, basePrefix)
		if externalModule == nil {
			errs = append(errs, fmt.Errorf("can't find external module with prefix %s", basePrefix))
		}

		// Run through the identities within the remote module and find the
		// one that matches the base that we have been specified.
		for _, rid := range externalModule.Identities() {
			if rid.Name == baseName {
				key, _ := makeResolvedIdentity(externalModule, rid)
				if id, ok := identityDict.dict[key]; ok {
					base = id
				} else {
					errs = append(errs, fmt.Errorf("can't find base %s", baseStr))
				}
				break
			}
		}
		// Error if we did not find the identity that had the name specified in
		// the module it was expected to be in.
		if base == nil {
			errs = append(errs, fmt.Errorf("can't resolve remote base %s", baseStr))
		}
	}
	return base, errs
}

func (ms *Modules) resolveIdentities() []error {

	// We are inplace modifying this global, and hence we have a lock on it.
	defer identityDict.mu.Unlock()
	identityDict.mu.Lock()

	var errs []error

	// Across all modules, read the identity values that have been extracted
	// from them, and compile them into a "fully resolved" map that mean that
	// we can look them up based on the 'real' prefix of the module and the
	// name of the identity.
	for _, mod := range ms.Modules {
		for _, i := range mod.Identities() {
			keyName, r := makeResolvedIdentity(mod, i)
			identityDict.dict[keyName] = r
		}
	}

	// Determine which identities have a base statement, and link this to a
	// fully resolved identity statement. The intention here is to make sure
	// that the Children slice is fully populated with pointers to all identities
	// that have a base, so that we can do inheritance of these later.
	for _, i := range identityDict.dict {
		if i.Identity.Base != nil {
			// This identity inherits from another identity.

			root := RootNode(i.Identity)
			base, baseErr := findIdentityBase(i.Identity.Base.asString(), root)

			if baseErr != nil {
				errs = append(errs, errs...)
				continue
			}

			// Append this value to the children of the base identity.
			base.Identity.Values = append(base.Identity.Values, i.Identity)
		}
	}

	// Do a final sweep through the identities to build up their children.
	for _, i := range identityDict.dict {
		newValues := []*Identity{}
		for _, j := range i.Identity.Values {
			newValues = learnChildren(j, newValues)
		}
		i.Identity.Values = newValues
	}

	return errs
}
