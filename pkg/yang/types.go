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

// This file implements the functions relating to types and typedefs.

import (
	"errors"
	"fmt"
	"regexp/syntax"
	"sync"
)

// A typeDictionary is a dictionary of all Typedefs defined in all Typedefers.
// A map of Nodes is used rather than a map of Typedefers to simplify usage
// when traversing up a Node tree.
type typeDictionary struct {
	mu   sync.Mutex
	dict map[Node]map[string]*Typedef
}

// typeDict is a protected global dictionary of all typedefs.
// TODO(borman): should this be made as part of some other structure, rather
// than a singleton.  That can be done later when we replumb everything to more
// or less pass around an extra pointer.  That is not needed until such time
// that we plan for a single application to process completely independent YANG
// modules where there may be conflicts between the modules or we plan to
// process them completely independently of eachother.
var typeDict = typeDictionary{dict: map[Node]map[string]*Typedef{}}

// add adds an entry to the typeDictionary d.
func (d *typeDictionary) add(n Node, name string, td *Typedef) {
	defer d.mu.Unlock()
	d.mu.Lock()
	if d.dict[n] == nil {
		d.dict[n] = map[string]*Typedef{}
	}
	d.dict[n][name] = td
}

// find returns the Typedef name define in node n, or nil.
func (d *typeDictionary) find(n Node, name string) *Typedef {
	defer d.mu.Unlock()
	d.mu.Lock()
	if d.dict[n] == nil {
		return nil
	}
	return d.dict[n][name]
}

// findExternal finds the externally defined typedef name in the module imported
// by n's root with the specified prefix.
func (d *typeDictionary) findExternal(n Node, prefix, name string) (*Typedef, error) {
	root := FindModuleByPrefix(n, prefix)
	if root == nil {
		return nil, fmt.Errorf("%s: unknown prefix: %s for type %s", Source(n), prefix, name)
	}
	if td := d.find(root, name); td != nil {
		return td, nil
	}
	if prefix != "" {
		name = prefix + ":" + name
	}
	return nil, fmt.Errorf("%s: unknown type %s", Source(n), name)
}

// typedefs returns a slice of all typedefs in d.
func (d *typeDictionary) typedefs() []*Typedef {
	var tds []*Typedef
	defer d.mu.Unlock()
	d.mu.Lock()
	for _, dict := range d.dict {
		for _, td := range dict {
			tds = append(tds, td)
		}
	}
	return tds
}

// addTypedefs is called from BuildAST after each Typedefer is defined.  There
// are no error conditions in this process as it is simply used to build up the
// typedef dictionary.
func (d *typeDictionary) addTypedefs(t Typedefer) {
	for _, td := range t.Typedefs() {
		d.add(t, td.Name, td)
	}
}

// resolveTypedefs is called after all of modules and submodules have been read,
// as well as their imports and includes.  It resolves all typedefs found in all
// modules and submodules read in.
func (d *typeDictionary) resolveTypedefs() []error {
	var errs []error

	// When resolve typedefs, we may need to look up other typedefs.
	// We gather all typedefs into a slice so we don't deadlock on
	// typeDict.
	for _, td := range d.typedefs() {
		errs = append(errs, td.resolve(d)...)
	}
	return errs
}

// resolve creates a YangType for t, if not already done.  Resolving t
// requires resolving the Type that t is based on.
func (t *Typedef) resolve(d *typeDictionary) []error {
	// If we have no parent we are a base type and
	// are already resolved.
	if t.Parent == nil || t.YangType != nil {
		return nil
	}

	if errs := t.Type.resolve(d); len(errs) != 0 {
		return errs
	}

	// Make a copy of the YangType we are based on and then
	// update it with local information.
	y := *t.Type.YangType
	y.Name = t.Name
	y.Base = t.Type

	if t.Units != nil {
		y.Units = t.Units.Name
	}
	if t.Default != nil {
		y.Default = t.Default.Name
	}

	if t.Type.IdentityBase != nil {
		// We need to copy over the IdentityBase statement if the type has one
		if idBase, err := RootNode(t).findIdentityBase(t.Type.IdentityBase.Name); err == nil {
			y.IdentityBase = idBase.Identity
		} else {
			return []error{fmt.Errorf("could not resolve identity base for typedef: %s", t.Type.IdentityBase.Name)}
		}
	}

	// If we changed something, we are the new root.
	if y.Root == t.Type.YangType || !y.Equal(y.Root) {
		y.Root = &y
	}
	t.YangType = &y
	return nil
}

// resolve resolves Type t, as well as the underlying typedef for t.  If t
// cannot be resolved then one or more errors are returned.
func (t *Type) resolve(d *typeDictionary) (errs []error) {
	if t.YangType != nil {
		return nil
	}

	// If t.Name is a base type then td will not be nil, otherwise
	// td will be nil and of type *Typedef.
	td := BaseTypedefs[t.Name]

	prefix, name := getPrefix(t.Name)
	root := RootNode(t)
	rootPrefix := root.GetPrefix()

	source := "unknown"
check:
	switch {
	case td != nil:
		source = "builtin"
		// This was a base type
	case prefix == "" || rootPrefix == prefix:
		source = "local"
		// If we have no prefix, or the prefix is what we call our own
		// root, then we look in our ancestors for a typedef of name.
		for n := Node(t); n != nil; n = n.ParentNode() {
			if td = d.find(n, name); td != nil {
				break check
			}
		}
		// We need to check our sub-modules as well
		for _, in := range root.Include {
			if td = d.find(in.Module, name); td != nil {
				break check
			}
		}
		var pname string
		switch {
		case prefix == "", prefix == root.Prefix.Name:
			pname = root.Prefix.Name + ":" + t.Name
		default:
			pname = fmt.Sprintf("%s[%s]:%s", prefix, root.Prefix.Name, t.Name)
		}

		return []error{fmt.Errorf("%s: unknown type: %s", Source(t), pname)}

	default:
		source = "imported"
		// prefix is not local to our module, so we have to go find
		// what module it is part of and if it is defined at the top
		// level of that module.
		var err error
		td, err = d.findExternal(t, prefix, name)
		if err != nil {
			return []error{err}
		}
	}
	if errs := td.resolve(d); len(errs) > 0 {
		return errs
	}

	// Make a copy of the typedef we are based on so we can
	// augment it.
	if td.YangType == nil {
		return []error{fmt.Errorf("%s: no YangType defined for %s %s", Source(td), source, td.Name)}
	}
	y := *td.YangType

	y.Base = td.Type
	t.YangType = &y

	if v := t.RequireInstance; v != nil {
		b, err := v.asBool()
		if err != nil {
			errs = append(errs, err)
		}
		y.OptionalInstance = !b
	}
	if v := t.Path; v != nil {
		y.Path = v.asString()
	}
	isDecimal64 := y.Kind == Ydecimal64 && (t.Name == "decimal64" || y.FractionDigits != 0)
	switch {
	case isDecimal64 && y.FractionDigits != 0:
		if t.FractionDigits != nil {
			return append(errs, fmt.Errorf("%s: overriding of fraction-digits not allowed", Source(t)))
		}
		// FractionDigits already set via type inheritance.
	case isDecimal64:
		// If we are directly of type decimal64 then we must specify
		// fraction-digits in the range from 1-18.
		i, err := t.FractionDigits.asRangeInt(1, 18)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", Source(t), err))
		}
		y.FractionDigits = int(i)
	case t.FractionDigits != nil:
		errs = append(errs, fmt.Errorf("%s: fraction-digits only allowed for decimal64 values", Source(t)))
	case y.Kind == Yidentityref:
		if source != "builtin" {
			// This is a typedef that refers to an identityref, so we want to simply
			// maintain the base that the typedef resolution provided
			break
		}

		if t.IdentityBase == nil {
			errs = append(errs, fmt.Errorf("%s: an identityref must specify a base", Source(t)))
			break
		}

		root := RootNode(t.Parent)
		resolvedBase, baseErr := root.findIdentityBase(t.IdentityBase.Name)
		if baseErr != nil {
			errs = append(errs, baseErr...)
			break
		}

		if resolvedBase.Identity == nil {
			errs = append(errs, fmt.Errorf("%s: identity has a null base", t.IdentityBase.Name))
			break
		}
		y.IdentityBase = resolvedBase.Identity
	}

	if t.Range != nil {
		yr, err := parseRanges(t.Range.Name, isDecimal64, uint8(y.FractionDigits))
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: bad range: %v", Source(t.Range), err))
		case !y.Range.Contains(yr):
			errs = append(errs, fmt.Errorf("%s: bad range: %v not within %v", Source(t.Range), yr, y.Range))
		case yr.Equal(y.Range):
		default:
			y.Range = yr
		}
	}

	if t.Length != nil {
		yr, err := ParseRangesInt(t.Length.Name)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: bad length: %v", Source(t.Length), err))
		case !y.Length.Contains(yr):
			errs = append(errs, fmt.Errorf("%s: bad length: %v not within %v", Source(t.Length), yr, y.Length))
		case yr.Equal(y.Length):
		default:
			for _, r := range yr {
				if r.Min.Kind == Negative {
					errs = append(errs, fmt.Errorf("%s: negative length: %v", Source(t.Length), yr))
					break
				}
			}
			y.Length = yr
		}
	}

	set := func(e *EnumType, name string, value *Value) error {
		if value == nil {
			return e.SetNext(name)
		}
		n, err := ParseInt(value.Name)
		if err != nil {
			return err
		}
		i, err := n.Int()
		if err != nil {
			return err
		}
		return e.Set(name, i)
	}

	if len(t.Enum) > 0 {
		enum := NewEnumType()
		for _, e := range t.Enum {
			if err := set(enum, e.Name, e.Value); err != nil {
				errs = append(errs, fmt.Errorf("%s: %v", Source(e), err))
			}
		}
		y.Enum = enum
	}

	if len(t.Bit) > 0 {
		bit := NewBitfield()
		for _, e := range t.Bit {
			if err := set(bit, e.Name, e.Position); err != nil {
				errs = append(errs, fmt.Errorf("%s: %v", Source(e), err))
			}
		}
		y.Bit = bit
	}

	// Append any newly found patterns to the end of the list of patterns.
	// Patterns are ANDed according to section 9.4.6.  If all the patterns
	// declared by t were also declared by the type t is based on, then
	// no patterns are added.
	seenPatterns := map[string]bool{}
	for _, p := range y.Pattern {
		seenPatterns[p] = true
	}
	seenPOSIXPatterns := map[string]bool{}
	for _, p := range y.POSIXPattern {
		seenPOSIXPatterns[p] = true
	}

	// First parse out the pattern statements.
	// These patterns are not checked because there is no support for W3C regexes by Go.
	for _, pv := range t.Pattern {
		if !seenPatterns[pv.Name] {
			seenPatterns[pv.Name] = true
			y.Pattern = append(y.Pattern, pv.Name)
		}
	}

	// Then, parse out the posix-pattern statements, if they exist.
	// A YANG module could make use of either or both, so we deal with each separately.
	posixPatterns, err := MatchingExtensions(t, "openconfig-extensions", "posix-pattern")
	if err != nil {
		return []error{err}
	}

	checkPattern := func(n Node, p string, flags syntax.Flags) {
		if _, err := syntax.Parse(p, flags); err != nil {
			if re, ok := err.(*syntax.Error); ok {
				// Error adds "error parsing regexp" to
				// the error, re.Code is the real error.
				err = errors.New(re.Code.String())
			}
			errs = append(errs, fmt.Errorf("%s: bad pattern: %v: %s", Source(n), err, p))
		}
	}
	for _, ext := range posixPatterns {
		checkPattern(ext, ext.Argument, syntax.POSIX)
		if !seenPOSIXPatterns[ext.Argument] {
			seenPOSIXPatterns[ext.Argument] = true
			y.POSIXPattern = append(y.POSIXPattern, ext.Argument)
		}
	}

	// I don't know of an easy way to use a type as a key to a map,
	// so we have to check equality the hard way.
looking:
	for _, ut := range t.Type {
		errs = append(errs, ut.resolve(d)...)
		if ut.YangType != nil {
			for _, yt := range y.Type {
				if ut.YangType.Equal(yt) {
					continue looking
				}
			}
			y.Type = append(y.Type, ut.YangType)
		}
	}

	// If we changed something, we are the new root.
	if !y.Equal(y.Root) {
		y.Root = &y
	}

	return errs
}
