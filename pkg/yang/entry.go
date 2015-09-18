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

// The file contains the code to convert an AST (Node) tree into an Entry tree
// via the ToEntry function.  The entry tree, once fully resolved, is the
// product of this package.  The tree should have all types and references
// resolved.
//
// TODO(borman): handle types, leafrefs, and extensions

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/indent"
)

// A TriState may be true, false, or unset
type TriState int

// The possible states of a TriState.
const (
	TSUnset = TriState(iota)
	TSTrue
	TSFalse
)

// Value returns the value of t as a boolean.  Unset is returned as false.
func (t TriState) Value() bool {
	return t == TSTrue
}

// String displays t as a string.
func (t TriState) String() string {
	switch t {
	case TSUnset:
		return "unset"
	case TSTrue:
		return "true"
	case TSFalse:
		return "false"
	default:
		return fmt.Sprintf("ts-%d", t)
	}
}

type ListAttr struct {
	MinElements *Value // leaf-list or list MUST have at least min-elements
	MaxElements *Value // leaf-list or list has at most max-elements
	OrderedBy   *Value // order of entries determined by "system" or "user"
}

// An Entry represents a single node (directory or leaf) created from the
// AST.  Directory entries have a non-nil Dir entry.  Leaf nodes have a nil
// Dir entry.  If Errors is not nil then the only other valid field is Node.
type Entry struct {
	Parent      *Entry
	Node        Node      // the base node this Entry was derived from.
	Name        string    // our name, same as the key in our parent Dirs
	Description string    // description from node, if any
	Errors      []error   // list of errors encounterd on this node
	Kind        EntryKind // kind of Entry
	IsList      bool      // if true, this node is actually a list
	Config      TriState  // config state of this entry, if known
	Prefix      string    // prefix to use from this point down

	// Fields associated with directory nodes
	Dir map[string]*Entry
	Key string // Optional key name for lists (i.e., maps)

	// Fields associated with leaf nodes
	Type *YangType
	Exts []*Entry // extensions found

	// Fields associted with choice statements
	Choice *Entry // The choice statement the entry is part of
	Case   *Entry // The case statement, if any, the entry is in

	// Fields associated with list nodes (both lists and leaf-lists)
	ListAttr *ListAttr
}

// Print prints e to w in human readable form.
func (e *Entry) Print(w io.Writer) {
	if e.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}
	if e.ReadOnly() {
		fmt.Fprintf(w, "RO: ")
	} else {
		fmt.Fprintf(w, "rw: ")
	}
	if e.Type != nil {
		fmt.Fprintf(w, "%s ", e.Type.Name)
	}
	switch {
	case e.Dir == nil && e.IsList:
		fmt.Fprintf(w, "[]%s\n", e.Name)
		return
	case e.Dir == nil:
		fmt.Fprintf(w, "%s\n", e.Name)
		return
	case e.IsList:
		fmt.Fprintf(w, "[%s]%s {\n", e.Key, e.Name) //}
	default:
		fmt.Fprintf(w, "%s {\n", e.Name) //}
	}
	var names []string
	for k := range e.Dir {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		e.Dir[k].Print(indent.NewWriter(w, "  "))
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}

// An EntryKind is the kind of node an Entry is.  All leaf nodes are of kind
// LeafEntry.  A LeafList is also considered a leaf node.  All other kinds are
// directory nodes.
type EntryKind int

// Enumeration of the types of entries.
const (
	LeafEntry = EntryKind(iota)
	DirectoryEntry
	AnyXMLEntry
	CaseEntry
	ChoiceEntry
	InputEntry
	NotificationEntry
	OutputEntry
)

var EntryKindToName = map[EntryKind]string{
	LeafEntry:         "Leaf",
	DirectoryEntry:    "Directory",
	AnyXMLEntry:       "AnyXML",
	CaseEntry:         "Case",
	ChoiceEntry:       "Choice",
	InputEntry:        "Input",
	NotificationEntry: "Notification",
	OutputEntry:       "Output",
}

func (k EntryKind) String() string {
	if s := EntryKindToName[k]; s != "" {
		return s
	}
	return fmt.Sprintf("unknown-entry-%d", k)
}

// newDirectory returns an empty directory Entry.
func newDirectory(n Node) *Entry {
	return &Entry{
		Kind: DirectoryEntry,
		Dir:  make(map[string]*Entry),
		Node: n,
		Name: n.NName(),
	}
}

// newLeaf returns an empty leaf Entry.
func newLeaf(n Node) *Entry {
	return &Entry{
		Kind: LeafEntry,
		Node: n,
		Name: n.NName(),
	}
}

// newError returns an error node using format and v to create the error
// contained in the node.  The location of the error is prepended.
func newError(n Node, format string, v ...interface{}) *Entry {
	e := &Entry{Node: n}
	e.errorf("%s: "+format, append([]interface{}{Source(n)}, v...)...)
	return e
}

// errorf appends the entry constructed from string and v to the list of errors
// on e.
func (e *Entry) errorf(format string, v ...interface{}) {
	e.Errors = append(e.Errors, fmt.Errorf(format, v...))
}

// addError appends err to the list of errors on e if err is not nil.
func (e *Entry) addError(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// importErrors imports all the errors from c and its children into e.
func (e *Entry) importErrors(c *Entry) {
	if c == nil {
		return
	}
	for _, err := range c.Errors {
		e.addError(err)
	}
	for _, ce := range e.Exts {
		e.importErrors(ce)
	}
	for _, ce := range c.Dir {
		e.importErrors(ce)
	}
	e.importErrors(e.Choice)
	e.importErrors(e.Case)
}

// checkErrors calls f on every error found in the tree e and its children.
func (e *Entry) checkErrors(f func(error)) {
	if e == nil {
		return
	}
	for _, e := range e.Dir {
		e.checkErrors(f)
	}
	for _, err := range e.Errors {
		f(err)
	}
	for _, e := range e.Exts {
		e.checkErrors(f)
	}
	e.Choice.checkErrors(f)
	e.Case.checkErrors(f)
}

// GetErrors returns a sorted list of errors found in e.
func (e *Entry) GetErrors() []error {
	// the seen map is used to eliminate dupicate errors.
	// Some entries will be processed more than once
	// (groupings in particular) and as such may cause
	// duplication of errors.
	seen := map[error]bool{}
	var errs []error
	e.checkErrors(func(err error) {
		if !seen[err] {
			errs = append(errs, err)
			seen[err] = true
		}
	})
	errorSort(errs)
	return errs
}

// asList annotates e as a list and returns e.
func (e *Entry) asList() *Entry {
	e.IsList = true
	return e
}

// asKind sets the kind of e to k and returns e.
func (e *Entry) asKind(k EntryKind) *Entry {
	e.Kind = k
	return e
}

// add adds the directory entry key assigned to the provided value.
func (e *Entry) add(key string, value *Entry) *Entry {
	value.Parent = e
	if e.Dir[key] != nil {
		e.errorf("%s: duplicate key from %s: %s", Source(e.Node), Source(value.Node), key)
		return e
	}
	e.Dir[key] = value
	return e
}

// entryCache is used to prevent unnecessary recursion into previously
// converted nodes.
var entryCache = map[Node]*Entry{}

var depth = 0

// ToEntry expands node n into a directory Entry.  Expansion is based on the
// YANG tags in the structure behind n.  ToEntry must only be used
// with nodes that are directories, such as top level modules and sub-modules.
// ToEntry never returns nil.  Any errors encountered are found in the Errors
// fields of the returned Entry and its children.  Use GetErrors to determine
// if there were any errors.
func ToEntry(n Node) (e *Entry) {
	if n == nil {
		err := errors.New("ToEntry called with nil")
		return &Entry{
			Node:   &ErrorNode{Error: err},
			Errors: []error{err},
		}
	}
	if e := entryCache[n]; e != nil {
		return e
	}
	defer func() {
		entryCache[n] = e
	}()

	// TODO(borman): should have a defer to process extensions at this point

	// configValue return TSTrue if i contains the value of true, TSFalse
	// if it contains the value of false, and TSUnset if i does not have
	// a set value (for instance, i is nil).  An error is returned if i
	// contains a value other than true or false.
	configValue := func(i interface{}) (TriState, error) {
		if v, ok := i.(*Value); ok && v != nil {
			switch v.Name {
			case "true":
				return TSTrue, nil
			case "false":
				return TSFalse, nil
			default:
				return TSUnset, fmt.Errorf("%s: invalid config value: %s", Source(n), v.Name)
			}
		}
		return TSUnset, nil
	}

	var err error

	// Handle non-directory nodes (leaf, leafref, and oddly enough, uses).
	switch s := n.(type) {
	case *Leaf:
		e := newLeaf(n)
		if errs := s.Type.resolve(); errs != nil {
			e.Errors = errs
		}
		if s.Description != nil {
			e.Description = s.Description.Name
		}
		e.Type = s.Type.YangType
		entryCache[n] = e
		e.Config, err = configValue(s.Config)
		e.addError(err)
		return e
	case *LeafList:
		// Create the equivalent leaf element that we are a list of.
		// We can then just annotate it as a list rather than a leaf.
		leaf := &Leaf{
			Name:        s.Name,
			Source:      s.Source,
			Parent:      s.Parent,
			Extensions:  s.Extensions,
			Config:      s.Config,
			Description: s.Description,
			IfFeature:   s.IfFeature,
			Must:        s.Must,
			Reference:   s.Reference,
			Status:      s.Status,
			Type:        s.Type,
			Units:       s.Units,
			When:        s.When,
		}

		e := ToEntry(leaf).asList()
		e.ListAttr = &ListAttr{
			MinElements: s.MinElements,
			MaxElements: s.MaxElements,
			OrderedBy:   s.OrderedBy,
		}
		return e
	case *Uses:
		g := FindGrouping(s, s.Name)
		if g == nil {
			return newError(n, "unknown group: %s", s.Name)
		}
		// We need to return a duplicate so we resolve properly
		// when the group is used in multiple locations and the
		// grouping has a leafref that references outside the group.
		return ToEntry(g).dup()
	}

	e = newDirectory(n)

	// Special handling of lists.  The only difference between a List
	// and any other node is that a List has the IsList bit set.  Other
	// than that it can be processed just like any other Node.
	switch s := n.(type) {
	case *List:
		e.asList()
		e.ListAttr = &ListAttr{
			MinElements: s.MinElements,
			MaxElements: s.MaxElements,
			OrderedBy:   s.OrderedBy,
		}
	}

	// Use Elem to get the Value of structure that n is pointing to, not
	// the Value of the pointer.
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	found := false

	// Collect all the augment entries and apply them after the tree
	// is fully read in.
	var augments []*Entry

	for i := t.NumField() - 1; i > 0; i-- {
		f := t.Field(i)
		yang := f.Tag.Get("yang")
		if yang == "" {
			continue
		}
		fv := v.Field(i)
		name := strings.Split(yang, ",")[0]
		switch name {
		default:
			continue
		case "config":
			e.Config, err = configValue(fv.Interface())
			e.addError(err)
		case "description":
			if v := fv.Interface().(*Value); v != nil {
				e.Description = v.Name
			}
		case "prefix":
			if v := fv.Interface().(*Value); v != nil {
				e.Prefix = v.Name
			}
		case "augment":
			for _, a := range fv.Interface().([]*Augment) {
				ne := ToEntry(a)
				ne.Parent = e
				augments = append(augments, ne)
			}
		case "anyxml":
			for _, a := range fv.Interface().([]*AnyXML) {
				e.add(a.Name, ToEntry(a))
			}
		case "case":
			for _, a := range fv.Interface().([]*Case) {
				e.add(a.Name, ToEntry(a))
			}
		case "choice":
			for _, a := range fv.Interface().([]*Choice) {
				e.add(a.Name, ToEntry(a))
			}
		case "container":
			for _, a := range fv.Interface().([]*Container) {
				e.add(a.Name, ToEntry(a))
			}
		case "grouping":
			for _, a := range fv.Interface().([]*Grouping) {
				// We just want to parse the grouping to
				// collect errors.
				e.importErrors(ToEntry(a))
			}
		case "import":
			// Apparently import only makes types and such
			// available.  There is nothing else for us to do.
		case "include":
			for _, a := range fv.Interface().([]*Include) {
				e.merge(a.Module.Prefix, ToEntry(a.Module))
			}
		case "leaf":
			for _, a := range fv.Interface().([]*Leaf) {
				e.add(a.Name, ToEntry(a))
			}
		case "leaf-list":
			for _, a := range fv.Interface().([]*LeafList) {
				e.add(a.Name, ToEntry(a))
			}
		case "list":
			for _, a := range fv.Interface().([]*List) {
				e.add(a.Name, ToEntry(a))
			}
		case "key":
			if v := fv.Interface().(*Value); v != nil {
				e.Key = v.Name
			}
		case "notification":
			for _, a := range fv.Interface().([]*Notification) {
				e.add(a.Name, ToEntry(a))
			}
		case "rpc":
			// TODO(borman): what do we do with these?
			// seems fine to ignore them for now, we are
			// just interested in the tree structure.
		case "uses":
			for _, a := range fv.Interface().([]*Uses) {
				e.merge(nil, ToEntry(a))
			}
		case "type":
			// We don't expect this to happen, so throw an error.
			// BUG(borman): I think a deviate statement might trigger this.
			e.addError(fmt.Errorf("%s: unexpected type in %s:%s", Source(n), n.Kind(), n.NName()))
		}
		// We found at least one field.
		found = true
	}

	// Now process the augments we found
	// NOTE(borman): is it possible this will fail if the augment refers
	// to some removed sibling that has not been processed?  Perhaps this
	// should be done after the entire tree is built.  Is it correct to
	// assume augment paths are data tree paths and not schema tree paths?
	for _, a := range augments {
		ae := a.Find(a.Name)
		if ae == nil {
			e.errorf("%s: augment %s not found", Source(a.Node), a.Name)
			continue
		}
		// Augments do not have a prefix we merge in, just a node.
		ae.merge(nil, a)
	}
	if !found {
		return newError(n, "%T: cannot be converted to a *Entry", n)
	}
	return e
}

// ReadOnly returns true if e is a read-only variable (config == false).
// If Config is unset in e, then false is returned if e has no parent,
// otherwise the value parent's ReadOnly is returned.
func (e *Entry) ReadOnly() bool {
	if e == nil {
		// We made it all the way to the root of the tree
		return false
	}
	if e.Config == TSUnset {
		return e.Parent.ReadOnly()
	}
	return !e.Config.Value()
}

// Find finds the Entry named by name relative to e.
// TODO(borman): support relative paths and prefixes.
func (e *Entry) Find(name string) *Entry {
	if e == nil || name == "" || name[0] != '/' {
		return nil
	}
	for e.Parent != nil {
		e = e.Parent
	}
	parts := strings.Split(name[1:], "/")
	for _, part := range parts {
		_, part = getPrefix(part)
		ne := e.Dir[part]
		if ne == nil {
			return nil
		}
		e = ne
	}
	return e
}

// Path returns the path to e. A nil Entry returns "".
func (e *Entry) Path() string {
	if e == nil {
		return ""
	}
	return e.Parent.Path() + "/" + e.Name
}

// dup makes a deep duplicate of e.
func (e *Entry) dup() *Entry {
	// Warning: if we add any elements to Entry that should not be
	// copied we will have to explicitly uncopy them.
	// It is possible we may want to do a deep copy on some other fields,
	// such as Exts, Choice and Case, but it is not clear that we need
	// to do that.
	ne := *e

	// Now recurse down to all of our children, fixing up Parent
	// pointers as we go.
	if e.Dir != nil {
		ne.Dir = make(map[string]*Entry, len(e.Dir))
		for k, v := range e.Dir {
			de := v.dup()
			de.Parent = &ne
			ne.Dir[k] = de
		}
	}
	return &ne
}

// merge merges a duplicate of oe.Dir into e.Dir, setting the prefix of each
// element to prefix, if not nil.  It is an error if e and oe contain common
// elements.
func (e *Entry) merge(prefix *Value, oe *Entry) {
	e.importErrors(oe)
	for k, v := range oe.Dir {
		v := v.dup()
		if prefix != nil {
			v.Prefix = prefix.Name
		}
		if se := e.Dir[k]; se != nil {
			er := newError(oe.Node, `Duplicate node %q in %q from:
   %s: %s
   %s: %s`, k, e.Name, Source(v.Node), v.Name, Source(se.Node), se.Name)
			e.addError(er.Errors[0])
		} else {
			e.Dir[k] = v
		}
	}
}

// nless returns -1 if a is less than b, 0 if a == b, and 1 if a > b.
// If a and b are both numeric, then nless compares them as numbers,
// otherwise they are compared lexicographically.
func nless(a, b string) int {
	an, ae := strconv.Atoi(a)
	bn, be := strconv.Atoi(b)
	switch {
	case ae == nil && be == nil:
		switch {
		case an < bn:
			return -1
		case an > bn:
			return 1
		default:
			return 0
		}
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

type sError struct {
	s   string
	err error
}

type sortedErrors []sError

func (s sortedErrors) Len() int      { return len(s) }
func (s sortedErrors) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortedErrors) Less(i, j int) bool {
	fi := strings.SplitN(s[i].s, ":", 4)
	fj := strings.SplitN(s[j].s, ":", 4)
	if fi[0] < fj[0] {
		return true
	}
	if fi[0] > fj[0] {
		return false
	}

	// compare compares field x to see which is less.
	// numbers are compared as numbers.
	compare := func(x int) int {
		switch {
		case len(fi) == x && len(fj) > x:
			return -1
		case len(fj) == x && len(fi) > x:
			return 1
		case len(fj) < x && len(fi) < x:
			return 0
		}
		return nless(fi[x], fj[x])
	}
	for x := 1; x < 4; x++ {
		switch compare(1) {
		case -1:
			return true
		case 1:
			return false
		}
	}
	return false
}

// errorSort sorts the strings in the errors slice assuming each line starts
// with file:line:col.  Line and column number are sorted numerically.
func errorSort(errors []error) {
	if len(errors) < 2 {
		return
	}
	elist := make(sortedErrors, len(errors))
	for x, err := range errors {
		elist[x] = sError{err.Error(), err}
	}
	sort.Sort(elist)
	for x, err := range elist {
		errors[x] = err.err
	}
}
