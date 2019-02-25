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

// This file implements the Modules type.  This includes the processing of
// include and import statements, which must be done prior to turning the
// module into an Entry tree.

import "fmt"

// Modules contains information about all the top level modules and
// submodules that are read into it via its Read method.
type Modules struct {
	Modules    map[string]*Module // All "module" nodes
	SubModules map[string]*Module // All "submodule" nodes
	includes   map[*Module]bool   // Modules we have already done include on
	byPrefix   map[string]*Module // Cache of prefix lookup
	byNS       map[string]*Module // Cache of namespace lookup
}

// NewModules returns a newly created and initialized Modules.
func NewModules() *Modules {
	return &Modules{
		Modules:    map[string]*Module{},
		SubModules: map[string]*Module{},
		includes:   map[*Module]bool{},
		byPrefix:   map[string]*Module{},
		byNS:       map[string]*Module{},
	}
}

// Read reads the named yang module into ms.  The name can be the name of an
// actual .yang file or a module/submodule name (the base name of a .yang file,
// e.g., foo.yang is named foo).  An error is returned if the file is not
// found or there was an error parsing the file.
func (ms *Modules) Read(name string) error {
	name, data, err := findFile(name)
	if err != nil {
		return err
	}
	return ms.Parse(string(data), name)
}

// Parse parses data as YANG source and adds it to ms.  The name should reflect
// the source of data.
func (ms *Modules) Parse(data, name string) error {
	ss, err := Parse(data, name)
	if err != nil {
		return err
	}
	for _, s := range ss {
		n, err := BuildAST(s)
		if err != nil {
			return err
		}
		ms.add(n)
	}
	return nil
}

// GetModule returns the Entry of the module named by name.  GetModule will
// search for and read the file named name + ".yang" if it cannot satisfy the
// request from what it has currntly read.
//
// GetModule is a convience function for calling Read and Process, and
// then looking up the module name.  It is safe to call Read and Process prior
// to calling GetModule.
func (ms *Modules) GetModule(name string) (*Entry, []error) {
	if ms.Modules[name] == nil {
		if err := ms.Read(name); err != nil {
			return nil, []error{err}
		}
		if ms.Modules[name] == nil {
			return nil, []error{fmt.Errorf("module not found: %s", name)}
		}
	}
	// Make sure that the modules have all been processed and have no
	// errors.
	if errs := ms.Process(); len(errs) != 0 {
		return nil, errs
	}
	return ToEntry(ms.Modules[name]), nil
}

// GetModule optionally reads in a set of YANG source files, named by sources,
// and then returns the Entry for the module named module.  If sources is
// missing, or the named module is not yet known, GetModule searches for name
// with the suffix ".yang".  GetModule either returns an Entry or returns
// one or more errors.
//
// GetModule is a convience function for calling NewModules, Read, and Process,
// and then looking up the module name.
func GetModule(name string, sources ...string) (*Entry, []error) {
	var errs []error
	ms := NewModules()
	for _, source := range sources {
		if err := ms.Read(source); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return ms.GetModule(name)
}

// add adds Node n to ms.  n must be assignable to *Module (i.e., it is a
// "module" or "submodule").  An error is returned if n is a duplicate of
// a name already added, or n is not assignable to *Module.
func (ms *Modules) add(n Node) error {
	var m map[string]*Module

	name := n.NName()
	kind := n.Kind()
	switch kind {
	case "module":
		m = ms.Modules
	case "submodule":
		m = ms.SubModules
	default:
		return fmt.Errorf("not a module or submodule: %s is of type %s", name, kind)
	}

	mod := n.(*Module)
	fullName := mod.FullName()
	mod.modules = ms

	if o := m[fullName]; o != nil {
		return fmt.Errorf("duplicate %s %s at %s and %s", kind, fullName, Source(o), Source(n))
	}
	m[fullName] = mod
	if fullName == name {
		return nil
	}

	// Add us to the map if:
	// name has not been added before
	// fullname is a more recent version of the entry.
	if o := m[name]; o == nil || o.FullName() < fullName {
		m[name] = mod
	}
	return nil
}

// FindModule returns the Module/Submodule specified by n, which must be a
// *Include or *Import.  If n is a *Include then a submodule is returned.  If n
// is a *Import then a module is returned.
func (ms *Modules) FindModule(n Node) *Module {
	name := n.NName()
	rev := name
	var m map[string]*Module

	switch i := n.(type) {
	case *Include:
		m = ms.SubModules
		if i.RevisionDate != nil {
			rev = name + "@" + i.RevisionDate.Name
		}
		// TODO(borman): we should check the BelongsTo field below?
	case *Import:
		m = ms.Modules
		if i.RevisionDate != nil {
			rev = name + "@" + i.RevisionDate.Name
		}
	default:
		return nil
	}
	if n := m[rev]; n != nil {
		return n
	}
	if n := m[name]; n != nil {
		return n
	}

	// Try to read it in.
	if err := ms.Read(name); err != nil {
		return nil
	}
	if n := m[rev]; n != nil {
		return n
	}
	return m[name]
}

// FindModuleByNamespace either returns the Module specified by the namespace
// or returns an error.
func (ms *Modules) FindModuleByNamespace(ns string) (*Module, error) {
	if m, ok := ms.byNS[ns]; ok {
		if m == nil {
			return nil, fmt.Errorf("%s: no such namespace", ns)
		}
		return m, nil
	}
	var found *Module
	for _, m := range ms.Modules {
		if m.Namespace.Name == ns {
			switch {
			case m == found:
			case found != nil:
				return nil, fmt.Errorf("namespace %s matches two or more modules (%s, %s)",
					ns, found.Name, m.Name)
			default:
				found = m
			}
		}
	}
	ms.byNS[ns] = found
	if found == nil {
		return nil, fmt.Errorf("%s: no such namespace", ns)
	}
	return found, nil
}

// FindModuleByPrefix either returns the Module specified by prefix or returns
// an error.
func (ms *Modules) FindModuleByPrefix(prefix string) (*Module, error) {
	if m, ok := ms.byPrefix[prefix]; ok {
		if m == nil {
			return nil, fmt.Errorf("%s: no such prefix", prefix)
		}
		return m, nil
	}
	var found *Module
	for _, m := range ms.Modules {
		if m.Prefix.Name == prefix {
			switch {
			case m == found:
			case found != nil:
				return nil, fmt.Errorf("prefix %s matches two or more modules (%s, %s)", prefix, found.Name, m.Name)
			default:
				found = m
			}
		}
	}
	ms.byPrefix[prefix] = found
	if found == nil {
		return nil, fmt.Errorf("%s: no such prefix", prefix)
	}
	return found, nil
}

// process satisfies all include and import statements and verifies that all
// link ref paths reference a known node.  If an import or include references
// a [sub]module that is not already known, Process will search for a .yang
// file that contains it, returning an error if not found.  An error is also
// returned if there is an unknown link ref path or other parsing errors.
//
// Process must be called once all the source modules have been read in and
// prior to converting Node tree into an Entry tree.
func (ms *Modules) process() []error {
	var mods []*Module
	var errs []error

	// Collect the list of modules we know about now so when we range
	// below we don't pick up new modules.  We assume the user tells
	// us explicitly which modules they are interested in.
	for _, m := range ms.Modules {
		mods = append(mods, m)
	}
	for _, m := range mods {
		if err := ms.include(m); err != nil {
			errs = append(errs, err)
		}
	}

	// Resolve identities before resolving typedefs, otherwise when we resolve a
	// typedef that has an identityref within it, then the identity dictionary
	// has not yet been built.
	errs = append(errs, ms.resolveIdentities()...)
	// Append any errors found trying to resolve typedefs
	errs = append(errs, resolveTypedefs()...)

	return errs
}

// Process processes all the modules and submodules that have been read into
// ms.  While processing, if an include or import is found for which there
// is no matching module, Process attempts to locate the source file (using
// Path) and automatically load them.  If a file cannot be found then an
// error is returned.  When looking for a source file, Process searches for a
// file using the module's or submodule's name with ".yang" appended.  After
// searching the current directory, the directories in Path are searched.
//
// Process builds Entry trees for each modules and submodules in ms.  These
// trees are accessed using the ToEntry function.  Process does augmentation
// on Entry trees once all the modules and submodules in ms have been built.
// Following augmentation, Process inserts implied case statements.  I.e.,
//
//   choice interface-type {
//       container ethernet { ... }
//   }
//
// has a case statement inserted to become:
//
//   choice interface-type {
//       case ethernet {
//           container ethernet { ... }
//       }
//   }
//
// Process may return multiple errors if multiple errors were encountered
// while processing.  Even though multiple errors may be returned, this does
// not mean these are all the errors.  Process will terminate processing early
// based on the type and location of the error.
func (ms *Modules) Process() []error {
	// Reset globals that may remain stale if multiple Process() calls are
	// made by the same caller.
	mergedSubmodule = map[string]bool{}
	entryCache = map[Node]*Entry{}

	errs := ms.process()
	if len(errs) > 0 {
		return errorSort(errs)
	}

	for _, m := range ms.Modules {
		errs = append(errs, ToEntry(m).GetErrors()...)
	}
	for _, m := range ms.SubModules {
		errs = append(errs, ToEntry(m).GetErrors()...)
	}

	if len(errs) > 0 {
		return errorSort(errs)
	}

	// Now handle all the augments.  We don't have a good way to know
	// what order to process them in, so repeat until no progress is made

	mods := make([]*Module, 0, len(ms.Modules)+len(ms.SubModules))
	for _, m := range ms.Modules {
		mods = append(mods, m)
	}
	for _, m := range ms.SubModules {
		mods = append(mods, m)
	}
	for len(mods) > 0 {
		var processed int
		for i := 0; i < len(mods); {
			m := mods[i]
			p, s := ToEntry(m).Augment(false)
			processed += p
			if s == 0 {
				mods[i] = mods[len(mods)-1]
				mods = mods[:len(mods)-1]
				continue
			}
			i++
		}
		if processed == 0 {
			break
		}
	}

	// Now fix up all the choice statements to add in the missing case
	// statements.
	for _, m := range ms.Modules {
		ToEntry(m).FixChoice()
	}
	for _, m := range ms.SubModules {
		ToEntry(m).FixChoice()
	}

	// Go through any modules that have remaining augments and collect
	// the errors.
	for _, m := range mods {
		ToEntry(m).Augment(true)
		errs = append(errs, ToEntry(m).GetErrors()...)
	}

	// The deviation statement is only valid under a module or submodule,
	// which allows us to avoid having to process it within ToEntry, and
	// rather we can just walk all modules and submodules *after* entries
	// are resolved. This means we do not need to concern ourselves that
	// an entry does not exist.
	dvP := map[string]bool{} // cache the modules we've handled since we have both modname and modname@revision-date
	for _, devmods := range []map[string]*Module{ms.Modules, ms.SubModules} {
		for _, m := range devmods {
			e := ToEntry(m)
			if !dvP[e.Name] {
				errs = append(errs, e.ApplyDeviate()...)
				dvP[e.Name] = true
			}
		}
	}

	return errorSort(errs)
}

// include resolves all the include and import statements for m.  It returns
// an error if m, or recursively, any of the modules it includes or imports,
// reference a module that cannot be found.
func (ms *Modules) include(m *Module) error {
	if ms.includes[m] {
		return nil
	}
	ms.includes[m] = true

	// First process any includes in this module.
	for _, i := range m.Include {
		im := ms.FindModule(i)
		if im == nil {
			return fmt.Errorf("no such submodule: %s", i.Name)
		}
		// Process the include statements in our included module.
		if err := ms.include(im); err != nil {
			return err
		}
		i.Module = im
	}

	// Next process any imports in this module.  Imports are used
	// when searching.
	for _, i := range m.Import {
		im := ms.FindModule(i)
		if im == nil {
			return fmt.Errorf("no such module: %s", i.Name)
		}
		// Process the include statements in our included module.
		if err := ms.include(im); err != nil {
			return err
		}

		i.Module = im
	}
	return nil
}
