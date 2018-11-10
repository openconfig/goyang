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

import "fmt"

// This file contains the definitions for all nodes of the yang AST.
// The actual building of the AST is in ast.go

// Some field names have specific meanings:
//
//  Grouping - This field must always be of type []*Grouping
//  Typedef - This field must always be of type []*Typedef

// A Value is just a string that can have extensions.
type Value struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge" json:",omitempty"`
	Parent     Node         `yang:"Parent,nomerge" json:"-"`
	Extensions []*Statement `yang:"Ext" json:",omitempty"`

	Description *Value `yang:"description" json:",omitempty"`
}

func (Value) Kind() string             { return "string" }
func (s *Value) ParentNode() Node      { return s.Parent }
func (s *Value) NName() string         { return s.Name }
func (s *Value) Statement() *Statement { return s.Source }
func (s *Value) Exts() []*Statement    { return s.Extensions }

// asRangeInt returns the value v as an int64 if it is between the values of
// min and max inclusive.  An error is returned if v is out of range or does
// not parse into a number.  If v is nil then an error is returned.
func (s *Value) asRangeInt(min, max int64) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("value is required in the range of [%d..%d]", min, max)
	}
	n, err := ParseNumber(s.Name)
	if err != nil {
		return 0, err
	}
	switch n.Kind {
	case MinNumber:
		return min, nil
	case MaxNumber:
		return max, nil
	}
	i, err := n.Int()
	if err != nil {
		return 0, err
	}
	if i < min || i > max {
		return 0, fmt.Errorf("value %s out of range [%d..%d]", s.Name, min, max)
	}
	return i, nil
}

// asBool returns v as a boolean (true or flase) or returns an error if v
// is neither true nor false.  If v is nil then false is returned.
func (s *Value) asBool() (bool, error) {
	// A missing value is considered false
	if s == nil {
		return false, nil
	}
	switch s.Name {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s", s.Name)
	}
}

// asString simply returns the string value of v.  If v is nil then an empty
// string is returned.
func (s *Value) asString() string {
	if s == nil {
		return ""
	}
	return s.Name
}

// See http://tools.ietf.org/html/rfc6020#section-7 for a description of the
// following structures.  The structures are derived from that document.

// A Module is defined in: http://tools.ietf.org/html/rfc6020#section-7.1
//
// A SubModule is defined in: http://tools.ietf.org/html/rfc6020#section-7.2
type Module struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata      []*AnyData      `yang:"anydata"`
	Anyxml       []*AnyXML       `yang:"anyxml"`
	Augment      []*Augment      `yang:"augment"`
	BelongsTo    *BelongsTo      `yang:"belongs-to,required=submodule,nomerge"`
	Choice       []*Choice       `yang:"choice"`
	Contact      *Value          `yang:"contact,nomerge"`
	Container    []*Container    `yang:"container"`
	Description  *Value          `yang:"description,nomerge"`
	Deviation    []*Deviation    `yang:"deviation"`
	Extension    []*Extension    `yang:"extension"`
	Feature      []*Feature      `yang:"feature"`
	Grouping     []*Grouping     `yang:"grouping"`
	Identity     []*Identity     `yang:"identity"`
	Import       []*Import       `yang:"import"`
	Include      []*Include      `yang:"include"`
	Leaf         []*Leaf         `yang:"leaf"`
	LeafList     []*LeafList     `yang:"leaf-list"`
	List         []*List         `yang:"list"`
	Namespace    *Value          `yang:"namespace,required=module,nomerge"`
	Notification []*Notification `yang:"notification"`
	Organization *Value          `yang:"organization,nomerge"`
	Prefix       *Value          `yang:"prefix,required=module,nomerge"`
	Reference    *Value          `yang:"reference,nomerge"`
	Revision     []*Revision     `yang:"revision,nomerge"`
	RPC          []*RPC          `yang:"rpc"`
	Typedef      []*Typedef      `yang:"typedef"`
	Uses         []*Uses         `yang:"uses"`
	YangVersion  *Value          `yang:"yang-version,nomerge"`

	// modules is used to get back to the Modules structure
	// when searching for a rooted element in the schema tree
	// as the schema tree has multiple root elements.
	// typedefs is a list of all top level typedefs in this
	// module.
	modules *Modules

	typedefs map[string]*Typedef
}

func (s *Module) Kind() string {
	if s.BelongsTo != nil {
		return "submodule"
	}
	return "module"
}
func (s *Module) ParentNode() Node        { return s.Parent }
func (s *Module) NName() string           { return s.Name }
func (s *Module) Statement() *Statement   { return s.Source }
func (s *Module) Exts() []*Statement      { return s.Extensions }
func (s *Module) Groupings() []*Grouping  { return s.Grouping }
func (s *Module) Typedefs() []*Typedef    { return s.Typedef }
func (s *Module) Identities() []*Identity { return s.Identity }

// Current returns the most recent revision of this module, or "" if the module
// has no revisions.
func (s *Module) Current() string {
	var rev string
	for _, r := range s.Revision {
		if r.Name > rev {
			rev = r.Name
		}
	}
	return rev
}

// FullName returns the full name of the module including the most recent
// revision, if any.
func (s *Module) FullName() string {
	if rev := s.Current(); rev != "" {
		return s.Name + "@" + rev
	}
	return s.Name
}

// GetPrefix returns the proper prefix of m.  Useful when looking up types
// in modules found by FindModuleByPrefix.
func (s *Module) GetPrefix() string {
	pfx := s.getPrefix()
	if pfx == nil {
		// This case can be true during testing.
		return ""
	}
	return pfx.Name
}

func (s *Module) getPrefix() *Value {
	switch {
	case s == nil:
		return nil
	case s.Prefix != nil:
		return s.Prefix
	case s.BelongsTo != nil:
		return s.BelongsTo.Prefix
	default:
		return nil
	}
}

// An Import is defined in: http://tools.ietf.org/html/rfc6020#section-7.1.5
type Import struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Prefix       *Value `yang:"prefix,required"`
	RevisionDate *Value `yang:"revision-date"`

	// Module is the imported module.  The types and groupings are
	// available to the importer with the defined prefix.
	Module *Module
}

func (Import) Kind() string             { return "import" }
func (s *Import) ParentNode() Node      { return s.Parent }
func (s *Import) NName() string         { return s.Name }
func (s *Import) Statement() *Statement { return s.Source }
func (s *Import) Exts() []*Statement    { return s.Extensions }

// An Include is defined in: http://tools.ietf.org/html/rfc6020#section-7.1.6
type Include struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	RevisionDate *Value `yang:"revision-date"`

	// Module is the included module.  The types and groupings are
	// available to the importer with the defined prefix.
	Module *Module
}

func (Include) Kind() string             { return "include" }
func (s *Include) ParentNode() Node      { return s.Parent }
func (s *Include) NName() string         { return s.Name }
func (s *Include) Statement() *Statement { return s.Source }
func (s *Include) Exts() []*Statement    { return s.Extensions }

// A Revision is defined in: http://tools.ietf.org/html/rfc6020#section-7.1.9
type Revision struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value `yang:"description"`
	Reference   *Value `yang:"reference"`
}

func (Revision) Kind() string             { return "revision" }
func (s *Revision) ParentNode() Node      { return s.Parent }
func (s *Revision) NName() string         { return s.Name }
func (s *Revision) Statement() *Statement { return s.Source }
func (s *Revision) Exts() []*Statement    { return s.Extensions }

// A BelongsTo is defined in: http://tools.ietf.org/html/rfc6020#section-7.2.2
type BelongsTo struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Prefix *Value `yang:"prefix,required"`
}

func (BelongsTo) Kind() string             { return "belongs-to" }
func (s *BelongsTo) ParentNode() Node      { return s.Parent }
func (s *BelongsTo) NName() string         { return s.Name }
func (s *BelongsTo) Statement() *Statement { return s.Source }
func (s *BelongsTo) Exts() []*Statement    { return s.Extensions }

// A Typedef is defined in: http://tools.ietf.org/html/rfc6020#section-7.3
type Typedef struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Default     *Value `yang:"default"`
	Description *Value `yang:"description"`
	Reference   *Value `yang:"reference"`
	Status      *Value `yang:"status"`
	Type        *Type  `yang:"type,required"`
	Units       *Value `yang:"units"`

	YangType *YangType `json:"-"`
}

func (Typedef) Kind() string             { return "typedef" }
func (s *Typedef) ParentNode() Node      { return s.Parent }
func (s *Typedef) NName() string         { return s.Name }
func (s *Typedef) Statement() *Statement { return s.Source }
func (s *Typedef) Exts() []*Statement    { return s.Extensions }

// A Type is defined in: http://tools.ietf.org/html/rfc6020#section-7.4
// Note that Name is the name of the type we want, it is what must
// be looked up and resolved.
type Type struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	IdentityBase    *Value     `yang:"base"` // Name == identityref
	Bit             []*Bit     `yang:"bit"`
	Enum            []*Enum    `yang:"enum"`
	FractionDigits  *Value     `yang:"fraction-digits"` // Name == decimal64
	Length          *Length    `yang:"length"`
	Path            *Value     `yang:"path"`
	Pattern         []*Pattern `yang:"pattern"`
	Range           *Range     `yang:"range"`
	RequireInstance *Value     `yang:"require-instance"`
	Type            []*Type    `yang:"type"` // len > 1 only when Name is "union"

	YangType *YangType
}

func (Type) Kind() string             { return "type" }
func (s *Type) ParentNode() Node      { return s.Parent }
func (s *Type) NName() string         { return s.Name }
func (s *Type) Statement() *Statement { return s.Source }
func (s *Type) Exts() []*Statement    { return s.Extensions }

// A Container is defined in: http://tools.ietf.org/html/rfc6020#section-7.5
// and http://tools.ietf.org/html/rfc7950#section-7.5 ("action" sub-statement)
type Container struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Action      []*Action    `yang:"action"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Choice      []*Choice    `yang:"choice"`
	Config      *Value       `yang:"config"`
	Container   []*Container `yang:"container"`
	Description *Value       `yang:"description"`
	Grouping    []*Grouping  `yang:"grouping"`
	IfFeature   []*Value     `yang:"if-feature"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	Must        []*Must      `yang:"must"`
	Presence    *Value       `yang:"presence"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	Typedef     []*Typedef   `yang:"typedef"`
	Uses        []*Uses      `yang:"uses"`
	When        *Value       `yang:"when"`
}

func (Container) Kind() string              { return "container" }
func (s *Container) ParentNode() Node       { return s.Parent }
func (s *Container) NName() string          { return s.Name }
func (s *Container) Statement() *Statement  { return s.Source }
func (s *Container) Exts() []*Statement     { return s.Extensions }
func (s *Container) Groupings() []*Grouping { return s.Grouping }
func (s *Container) Typedefs() []*Typedef   { return s.Typedef }

// A Must is defined in: http://tools.ietf.org/html/rfc6020#section-7.5.3
type Must struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description  *Value `yang:"description"`
	ErrorAppTag  *Value `yang:"error-app-tag"`
	ErrorMessage *Value `yang:"error-message"`
	Reference    *Value `yang:"reference"`
}

func (Must) Kind() string             { return "must" }
func (s *Must) ParentNode() Node      { return s.Parent }
func (s *Must) NName() string         { return s.Name }
func (s *Must) Statement() *Statement { return s.Source }
func (s *Must) Exts() []*Statement    { return s.Extensions }

// A Leaf is defined in: http://tools.ietf.org/html/rfc6020#section-7.6
type Leaf struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Config      *Value   `yang:"config"`
	Default     *Value   `yang:"default"`
	Description *Value   `yang:"description"`
	IfFeature   []*Value `yang:"if-feature"`
	Mandatory   *Value   `yang:"mandatory"`
	Must        []*Must  `yang:"must"`
	Reference   *Value   `yang:"reference"`
	Status      *Value   `yang:"status"`
	Type        *Type    `yang:"type,required"`
	Units       *Value   `yang:"units"`
	When        *Value   `yang:"when"`
}

func (Leaf) Kind() string             { return "leaf" }
func (s *Leaf) ParentNode() Node      { return s.Parent }
func (s *Leaf) NName() string         { return s.Name }
func (s *Leaf) Statement() *Statement { return s.Source }
func (s *Leaf) Exts() []*Statement    { return s.Extensions }

// A LeafList is defined in: http://tools.ietf.org/html/rfc6020#section-7.7
// It this is supposed to be an array of nodes..
type LeafList struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Config      *Value   `yang:"config"`
	Description *Value   `yang:"description"`
	IfFeature   []*Value `yang:"if-feature"`
	MaxElements *Value   `yang:"max-elements"`
	MinElements *Value   `yang:"min-elements"`
	Must        []*Must  `yang:"must"`
	OrderedBy   *Value   `yang:"ordered-by"`
	Reference   *Value   `yang:"reference"`
	Status      *Value   `yang:"status"`
	Type        *Type    `yang:"type,required"`
	Units       *Value   `yang:"units"`
	When        *Value   `yang:"when"`
}

func (LeafList) Kind() string             { return "leaf-list" }
func (s *LeafList) ParentNode() Node      { return s.Parent }
func (s *LeafList) NName() string         { return s.Name }
func (s *LeafList) Statement() *Statement { return s.Source }
func (s *LeafList) Exts() []*Statement    { return s.Extensions }

// A List is defined in: http://tools.ietf.org/html/rfc6020#section-7.8
// and http://tools.ietf.org/html/rfc7950#section-7.8 ("action" sub-statement)
type List struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Action      []*Action    `yang:"action"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Choice      []*Choice    `yang:"choice"`
	Config      *Value       `yang:"config"`
	Container   []*Container `yang:"container"`
	Description *Value       `yang:"description"`
	Grouping    []*Grouping  `yang:"grouping"`
	IfFeature   []*Value     `yang:"if-feature"`
	Key         *Value       `yang:"key"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	MaxElements *Value       `yang:"max-elements"`
	MinElements *Value       `yang:"min-elements"`
	Must        []*Must      `yang:"must"`
	OrderedBy   *Value       `yang:"ordered-by"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	Typedef     []*Typedef   `yang:"typedef"`
	Unique      []*Value     `yang:"unique"`
	Uses        []*Uses      `yang:"uses"`
	When        *Value       `yang:"when"`
}

func (List) Kind() string              { return "list" }
func (s *List) ParentNode() Node       { return s.Parent }
func (s *List) NName() string          { return s.Name }
func (s *List) Statement() *Statement  { return s.Source }
func (s *List) Exts() []*Statement     { return s.Extensions }
func (s *List) Groupings() []*Grouping { return s.Grouping }
func (s *List) Typedefs() []*Typedef   { return s.Typedef }

// A Choice is defined in: http://tools.ietf.org/html/rfc6020#section-7.9
type Choice struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Case        []*Case      `yang:"case"`
	Config      *Value       `yang:"config"`
	Container   []*Container `yang:"container"`
	Default     *Value       `yang:"default"`
	Description *Value       `yang:"description"`
	IfFeature   []*Value     `yang:"if-feature"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	Mandatory   *Value       `yang:"mandatory"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	When        *Value       `yang:"when"`
}

func (Choice) Kind() string             { return "choice" }
func (s *Choice) ParentNode() Node      { return s.Parent }
func (s *Choice) NName() string         { return s.Name }
func (s *Choice) Statement() *Statement { return s.Source }
func (s *Choice) Exts() []*Statement    { return s.Extensions }

// A Case is defined in: http://tools.ietf.org/html/rfc6020#section-7.9.2
type Case struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Choice      []*Choice    `yang:"choice"`
	Container   []*Container `yang:"container"`
	Description *Value       `yang:"description"`
	IfFeature   []*Value     `yang:"if-feature"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	Uses        []*Uses      `yang:"uses"`
	When        *Value       `yang:"when"`
}

func (Case) Kind() string             { return "case" }
func (s *Case) ParentNode() Node      { return s.Parent }
func (s *Case) NName() string         { return s.Name }
func (s *Case) Statement() *Statement { return s.Source }
func (s *Case) Exts() []*Statement    { return s.Extensions }

// An AnyXML is defined in: http://tools.ietf.org/html/rfc6020#section-7.10
type AnyXML struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Config      *Value   `yang:"config"`
	Description *Value   `yang:"description"`
	IfFeature   []*Value `yang:"if-feature"`
	Mandatory   *Value   `yang:"mandatory"`
	Must        []*Must  `yang:"must"`
	Reference   *Value   `yang:"reference"`
	Status      *Value   `yang:"status"`
	When        *Value   `yang:"when"`
}

func (AnyXML) Kind() string             { return "anyxml" }
func (s *AnyXML) ParentNode() Node      { return s.Parent }
func (s *AnyXML) NName() string         { return s.Name }
func (s *AnyXML) Statement() *Statement { return s.Source }
func (s *AnyXML) Exts() []*Statement    { return s.Extensions }

// An AnyData is defined in: http://tools.ietf.org/html/rfc7950#section-7.10
//
// AnyData are only expected in YANG 1.1 modules (those with a
// "yang-version 1.1;" statement in the module).
type AnyData struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Config      *Value   `yang:"config"`
	Description *Value   `yang:"description"`
	IfFeature   []*Value `yang:"if-feature"`
	Mandatory   *Value   `yang:"mandatory"`
	Must        []*Must  `yang:"must"`
	Reference   *Value   `yang:"reference"`
	Status      *Value   `yang:"status"`
	When        *Value   `yang:"when"`
}

func (AnyData) Kind() string             { return "anydata" }
func (s *AnyData) ParentNode() Node      { return s.Parent }
func (s *AnyData) NName() string         { return s.Name }
func (s *AnyData) Statement() *Statement { return s.Source }
func (s *AnyData) Exts() []*Statement    { return s.Extensions }

// A Grouping is defined in: http://tools.ietf.org/html/rfc6020#section-7.11
// and http://tools.ietf.org/html/rfc7950#section-7.12 ("action" sub-statement)
type Grouping struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Action      []*Action    `yang:"action"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Choice      []*Choice    `yang:"choice"`
	Container   []*Container `yang:"container"`
	Description *Value       `yang:"description"`
	Grouping    []*Grouping  `yang:"grouping"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	Typedef     []*Typedef   `yang:"typedef"`
	Uses        []*Uses      `yang:"uses"`
}

func (Grouping) Kind() string              { return "grouping" }
func (s *Grouping) ParentNode() Node       { return s.Parent }
func (s *Grouping) NName() string          { return s.Name }
func (s *Grouping) Statement() *Statement  { return s.Source }
func (s *Grouping) Exts() []*Statement     { return s.Extensions }
func (s *Grouping) Groupings() []*Grouping { return s.Grouping }
func (s *Grouping) Typedefs() []*Typedef   { return s.Typedef }

// A Uses is defined in: http://tools.ietf.org/html/rfc6020#section-7.12
type Uses struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Augment     *Augment  `yang:"augment"`
	Description *Value    `yang:"description"`
	IfFeature   []*Value  `yang:"if-feature"`
	Refine      []*Refine `yang:"refine"`
	Reference   *Value    `yang:"reference"`
	Status      *Value    `yang:"status"`
	When        *Value    `yang:"when"`
}

func (Uses) Kind() string             { return "uses" }
func (s *Uses) ParentNode() Node      { return s.Parent }
func (s *Uses) NName() string         { return s.Name }
func (s *Uses) Statement() *Statement { return s.Source }
func (s *Uses) Exts() []*Statement    { return s.Extensions }

// A Refine is defined in: http://tools.ietf.org/html/rfc6020#section-7.12.2
type Refine struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Default     *Value  `yang:"default"`
	Description *Value  `yang:"description"`
	Reference   *Value  `yang:"reference"`
	Config      *Value  `yang:"config"`
	Mandatory   *Value  `yang:"mandatory"`
	Presence    *Value  `yang:"presence"`
	Must        []*Must `yang:"must"`
	MaxElements *Value  `yang:"max-elements"`
	MinElements *Value  `yang:"min-elements"`
}

func (Refine) Kind() string             { return "refine" }
func (s *Refine) ParentNode() Node      { return s.Parent }
func (s *Refine) NName() string         { return s.Name }
func (s *Refine) Statement() *Statement { return s.Source }
func (s *Refine) Exts() []*Statement    { return s.Extensions }

// An RPC is defined in: http://tools.ietf.org/html/rfc6020#section-7.13
type RPC struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value      `yang:"description"`
	Grouping    []*Grouping `yang:"grouping"`
	IfFeature   []*Value    `yang:"if-feature"`
	Input       *Input      `yang:"input"`
	Output      *Output     `yang:"output"`
	Reference   *Value      `yang:"reference"`
	Status      *Value      `yang:"status"`
	Typedef     []*Typedef  `yang:"typedef"`
}

func (RPC) Kind() string              { return "rpc" }
func (s *RPC) ParentNode() Node       { return s.Parent }
func (s *RPC) NName() string          { return s.Name }
func (s *RPC) Statement() *Statement  { return s.Source }
func (s *RPC) Exts() []*Statement     { return s.Extensions }
func (s *RPC) Groupings() []*Grouping { return s.Grouping }
func (s *RPC) Typedefs() []*Typedef   { return s.Typedef }

// An Input is defined in: http://tools.ietf.org/html/rfc6020#section-7.13.2
type Input struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata   []*AnyData   `yang:"anydata"`
	Anyxml    []*AnyXML    `yang:"anyxml"`
	Choice    []*Choice    `yang:"choice"`
	Container []*Container `yang:"container"`
	Grouping  []*Grouping  `yang:"grouping"`
	Leaf      []*Leaf      `yang:"leaf"`
	LeafList  []*LeafList  `yang:"leaf-list"`
	List      []*List      `yang:"list"`
	Typedef   []*Typedef   `yang:"typedef"`
	Uses      []*Uses      `yang:"uses"`
}

func (Input) Kind() string              { return "input" }
func (s *Input) ParentNode() Node       { return s.Parent }
func (s *Input) NName() string          { return s.Name }
func (s *Input) Statement() *Statement  { return s.Source }
func (s *Input) Exts() []*Statement     { return s.Extensions }
func (s *Input) Groupings() []*Grouping { return s.Grouping }
func (s *Input) Typedefs() []*Typedef   { return s.Typedef }

// An Output is defined in: http://tools.ietf.org/html/rfc6020#section-7.13.3
type Output struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata   []*AnyData   `yang:"anydata"`
	Anyxml    []*AnyXML    `yang:"anyxml"`
	Choice    []*Choice    `yang:"choice"`
	Container []*Container `yang:"container"`
	Grouping  []*Grouping  `yang:"grouping"`
	Leaf      []*Leaf      `yang:"leaf"`
	LeafList  []*LeafList  `yang:"leaf-list"`
	List      []*List      `yang:"list"`
	Typedef   []*Typedef   `yang:"typedef"`
	Uses      []*Uses      `yang:"uses"`
}

func (Output) Kind() string              { return "output" }
func (s *Output) ParentNode() Node       { return s.Parent }
func (s *Output) NName() string          { return s.Name }
func (s *Output) Statement() *Statement  { return s.Source }
func (s *Output) Exts() []*Statement     { return s.Extensions }
func (s *Output) Groupings() []*Grouping { return s.Grouping }
func (s *Output) Typedefs() []*Typedef   { return s.Typedef }

// A Notification is defined in: http://tools.ietf.org/html/rfc6020#section-7.14
type Notification struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Choice      []*Choice    `yang:"choice"`
	Container   []*Container `yang:"container"`
	Description *Value       `yang:"description"`
	Grouping    []*Grouping  `yang:"grouping"`
	IfFeature   []*Value     `yang:"if-feature"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	Typedef     []*Typedef   `yang:"typedef"`
	Uses        []*Uses      `yang:"uses"`
}

func (Notification) Kind() string              { return "notification" }
func (s *Notification) ParentNode() Node       { return s.Parent }
func (s *Notification) NName() string          { return s.Name }
func (s *Notification) Statement() *Statement  { return s.Source }
func (s *Notification) Exts() []*Statement     { return s.Extensions }
func (s *Notification) Groupings() []*Grouping { return s.Grouping }
func (s *Notification) Typedefs() []*Typedef   { return s.Typedef }

// An Augment is defined in: http://tools.ietf.org/html/rfc6020#section-7.15
// and http://tools.ietf.org/html/rfc7950#section-7.17 ("action" sub-statement)
type Augment struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Anydata     []*AnyData   `yang:"anydata"`
	Action      []*Action    `yang:"action"`
	Anyxml      []*AnyXML    `yang:"anyxml"`
	Case        []*Case      `yang:"case"`
	Choice      []*Choice    `yang:"choice"`
	Container   []*Container `yang:"container"`
	Description *Value       `yang:"description"`
	IfFeature   []*Value     `yang:"if-feature"`
	Leaf        []*Leaf      `yang:"leaf"`
	LeafList    []*LeafList  `yang:"leaf-list"`
	List        []*List      `yang:"list"`
	Reference   *Value       `yang:"reference"`
	Status      *Value       `yang:"status"`
	Uses        []*Uses      `yang:"uses"`
	When        *Value       `yang:"when"`
}

func (Augment) Kind() string             { return "augment" }
func (s *Augment) ParentNode() Node      { return s.Parent }
func (s *Augment) NName() string         { return s.Name }
func (s *Augment) Statement() *Statement { return s.Source }
func (s *Augment) Exts() []*Statement    { return s.Extensions }

// An Identity is defined in: http://tools.ietf.org/html/rfc6020#section-7.16
type Identity struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge" json:"-"`
	Parent     Node         `yang:"Parent,nomerge" json:"-"`
	Extensions []*Statement `yang:"Ext" json:"-"`

	Base        *Value      `yang:"base" json:"-"`
	Description *Value      `yang:"description" json:"-"`
	Reference   *Value      `yang:"reference" json:"-"`
	Status      *Value      `yang:"status" json:"-"`
	Values      []*Identity `json:",omitempty"`
}

func (Identity) Kind() string             { return "identity" }
func (s *Identity) ParentNode() Node      { return s.Parent }
func (s *Identity) NName() string         { return s.Name }
func (s *Identity) Statement() *Statement { return s.Source }
func (s *Identity) Exts() []*Statement    { return s.Extensions }

// PrefixedName returns the prefix-qualified name for the identity
func (s *Identity) PrefixedName() string {
	return fmt.Sprintf("%s:%s", RootNode(s).GetPrefix(), s.Name)
}

// IsDefined behaves the same as the implementation for Enum - it returns
// true if an identity with the name is defined within the Values of the
// identity
func (s *Identity) IsDefined(name string) bool {
	if s.GetValue(name) != nil {
		return true
	}
	return false
}

// GetValue returns a pointer to the identity with name "name" that is within
// the values of the identity
func (s *Identity) GetValue(name string) *Identity {
	for _, v := range s.Values {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// An Extension is defined in: http://tools.ietf.org/html/rfc6020#section-7.17
type Extension struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Argument    *Argument `yang:"argument"`
	Description *Value    `yang:"description"`
	Reference   *Value    `yang:"reference"`
	Status      *Value    `yang:"status"`
}

func (Extension) Kind() string             { return "extension" }
func (s *Extension) ParentNode() Node      { return s.Parent }
func (s *Extension) NName() string         { return s.Name }
func (s *Extension) Statement() *Statement { return s.Source }
func (s *Extension) Exts() []*Statement    { return s.Extensions }

// An Argument is defined in: http://tools.ietf.org/html/rfc6020#section-7.17.2
type Argument struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	YinElement *Value `yang:"yin-element"`
}

func (Argument) Kind() string             { return "argument" }
func (s *Argument) ParentNode() Node      { return s.Parent }
func (s *Argument) NName() string         { return s.Name }
func (s *Argument) Statement() *Statement { return s.Source }
func (s *Argument) Exts() []*Statement    { return s.Extensions }

// An Element is defined in: http://tools.ietf.org/html/rfc6020#section-7.17.2.2
type Element struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	YinElement *Value `yang:"yin-element"`
}

func (Element) Kind() string             { return "element" }
func (s *Element) ParentNode() Node      { return s.Parent }
func (s *Element) NName() string         { return s.Name }
func (s *Element) Statement() *Statement { return s.Source }
func (s *Element) Exts() []*Statement    { return s.Extensions }

// A Feature is defined in: http://tools.ietf.org/html/rfc6020#section-7.18.1
type Feature struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value   `yang:"description"`
	IfFeature   []*Value `yang:"if-feature"`
	Status      *Value   `yang:"status"`
	Reference   *Value   `yang:"reference"`
}

func (Feature) Kind() string             { return "feature" }
func (s *Feature) ParentNode() Node      { return s.Parent }
func (s *Feature) NName() string         { return s.Name }
func (s *Feature) Statement() *Statement { return s.Source }
func (s *Feature) Exts() []*Statement    { return s.Extensions }

// A Deviation is defined in: http://tools.ietf.org/html/rfc6020#section-7.18.3
type Deviation struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value     `yang:"description"`
	Deviate     []*Deviate `yang:"deviate,required"`
	Reference   *Value     `yang:"reference"`
}

func (Deviation) Kind() string             { return "deviation" }
func (s *Deviation) ParentNode() Node      { return s.Parent }
func (s *Deviation) NName() string         { return s.Name }
func (s *Deviation) Statement() *Statement { return s.Source }
func (s *Deviation) Exts() []*Statement    { return s.Extensions }

// A Deviate is defined in: http://tools.ietf.org/html/rfc6020#section-7.18.3.2
type Deviate struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Config      *Value   `yang:"config"`
	Default     *Value   `yang:"default"`
	Mandatory   *Value   `yang:"mandatory"`
	MaxElements *Value   `yang:"max-elements"`
	MinElements *Value   `yang:"min-elements"`
	Must        []*Must  `yang:"must"`
	Type        *Type    `yang:"type"`
	Unique      []*Value `yang:"unique"`
	Units       *Value   `yang:"units"`
}

func (Deviate) Kind() string             { return "deviate" }
func (s *Deviate) ParentNode() Node      { return s.Parent }
func (s *Deviate) NName() string         { return s.Name }
func (s *Deviate) Statement() *Statement { return s.Source }
func (s *Deviate) Exts() []*Statement    { return s.Extensions }

// An Enum is defined in: http://tools.ietf.org/html/rfc6020#section-9.6.4
type Enum struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value `yang:"description"`
	Reference   *Value `yang:"reference"`
	Status      *Value `yang:"status"`
	Value       *Value `yang:"value"`
}

func (Enum) Kind() string             { return "enum" }
func (s *Enum) ParentNode() Node      { return s.Parent }
func (s *Enum) NName() string         { return s.Name }
func (s *Enum) Statement() *Statement { return s.Source }
func (s *Enum) Exts() []*Statement    { return s.Extensions }

// A Bit is defined in: http://tools.ietf.org/html/rfc6020#section-9.7.4
type Bit struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value `yang:"description"`
	Reference   *Value `yang:"reference"`
	Status      *Value `yang:"status"`
	Position    *Value `yang:"position"`
}

func (Bit) Kind() string             { return "bit" }
func (s *Bit) ParentNode() Node      { return s.Parent }
func (s *Bit) NName() string         { return s.Name }
func (s *Bit) Statement() *Statement { return s.Source }
func (s *Bit) Exts() []*Statement    { return s.Extensions }

// A Range is defined in: http://tools.ietf.org/html/rfc6020#section-9.2.4
type Range struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description  *Value `yang:"description"`
	ErrorAppTag  *Value `yang:"error-app-tag"`
	ErrorMessage *Value `yang:"error-message"`
	Reference    *Value `yang:"reference"`
}

func (Range) Kind() string             { return "range" }
func (s *Range) ParentNode() Node      { return s.Parent }
func (s *Range) NName() string         { return s.Name }
func (s *Range) Statement() *Statement { return s.Source }
func (s *Range) Exts() []*Statement    { return s.Extensions }

// A Length is defined in: http://tools.ietf.org/html/rfc6020#section-9.4.4
type Length struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description  *Value `yang:"description"`
	ErrorAppTag  *Value `yang:"error-app-tag"`
	ErrorMessage *Value `yang:"error-message"`
	Reference    *Value `yang:"reference"`
}

func (Length) Kind() string             { return "length" }
func (s *Length) ParentNode() Node      { return s.Parent }
func (s *Length) NName() string         { return s.Name }
func (s *Length) Statement() *Statement { return s.Source }
func (s *Length) Exts() []*Statement    { return s.Extensions }

// A Pattern is defined in: http://tools.ietf.org/html/rfc6020#section-9.4.6
type Pattern struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description  *Value `yang:"description"`
	ErrorAppTag  *Value `yang:"error-app-tag"`
	ErrorMessage *Value `yang:"error-message"`
	Reference    *Value `yang:"reference"`
}

func (Pattern) Kind() string             { return "pattern" }
func (s *Pattern) ParentNode() Node      { return s.Parent }
func (s *Pattern) NName() string         { return s.Name }
func (s *Pattern) Statement() *Statement { return s.Source }
func (s *Pattern) Exts() []*Statement    { return s.Extensions }

// An Action is defined in http://tools.ietf.org/html/rfc7950#section-7.15
//
// Action define an RPC operation connected to a specific container or list data
// node in the schema. In the schema tree, Action differ from RPC only in where
// in the tree they are found. RPC nodes are only found as sub-statements of a
// Module, while Action are found only as sub-statements of Container, List,
// Grouping and Augment nodes.
type Action struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Description *Value      `yang:"description"`
	Grouping    []*Grouping `yang:"grouping"`
	IfFeature   []*Value    `yang:"if-feature"`
	Input       *Input      `yang:"input"`
	Output      *Output     `yang:"output"`
	Reference   *Value      `yang:"reference"`
	Status      *Value      `yang:"status"`
	Typedef     []*Typedef  `yang:"typedef"`
}

func (Action) Kind() string              { return "action" }
func (s *Action) ParentNode() Node       { return s.Parent }
func (s *Action) NName() string          { return s.Name }
func (s *Action) Statement() *Statement  { return s.Source }
func (s *Action) Exts() []*Statement     { return s.Extensions }
func (s *Action) Groupings() []*Grouping { return s.Grouping }
func (s *Action) Typedefs() []*Typedef   { return s.Typedef }
