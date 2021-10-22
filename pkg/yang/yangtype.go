// Copyright 2021 Google Inc.
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

	"github.com/google/go-cmp/cmp"
)

var (
	// TypeKindFromName maps the string name used in a YANG file to the enumerated
	// TypeKind used in this library.
	TypeKindFromName = map[string]TypeKind{
		"none":                Ynone,
		"int8":                Yint8,
		"int16":               Yint16,
		"int32":               Yint32,
		"int64":               Yint64,
		"uint8":               Yuint8,
		"uint16":              Yuint16,
		"uint32":              Yuint32,
		"uint64":              Yuint64,
		"binary":              Ybinary,
		"bits":                Ybits,
		"boolean":             Ybool,
		"decimal64":           Ydecimal64,
		"empty":               Yempty,
		"enumeration":         Yenum,
		"identityref":         Yidentityref,
		"instance-identifier": YinstanceIdentifier,
		"leafref":             Yleafref,
		"string":              Ystring,
		"union":               Yunion,
	}

	// TypeKindToName maps the enumerated type used in this library to the string
	// used in a YANG file.
	TypeKindToName = map[TypeKind]string{
		Ynone:               "none",
		Yint8:               "int8",
		Yint16:              "int16",
		Yint32:              "int32",
		Yint64:              "int64",
		Yuint8:              "uint8",
		Yuint16:             "uint16",
		Yuint32:             "uint32",
		Yuint64:             "uint64",
		Ybinary:             "binary",
		Ybits:               "bits",
		Ybool:               "boolean",
		Ydecimal64:          "decimal64",
		Yempty:              "empty",
		Yenum:               "enumeration",
		Yidentityref:        "identityref",
		YinstanceIdentifier: "instance-identifier",
		Yleafref:            "leafref",
		Ystring:             "string",
		Yunion:              "union",
	}

	// BaseTypedefs is a map of all base types to the Typedef structure manufactured
	// for the type.
	BaseTypedefs = map[string]*Typedef{}

	baseTypes = map[string]*YangType{
		"int8": {
			Name:  "int8",
			Kind:  Yint8,
			Range: Int8Range,
		},
		"int16": {
			Name:  "int16",
			Kind:  Yint16,
			Range: Int16Range,
		},
		"int32": {
			Name:  "int32",
			Kind:  Yint32,
			Range: Int32Range,
		},
		"int64": {
			Name:  "int64",
			Kind:  Yint64,
			Range: Int64Range,
		},
		"uint8": {
			Name:  "uint8",
			Kind:  Yuint8,
			Range: Uint8Range,
		},
		"uint16": {
			Name:  "uint16",
			Kind:  Yuint16,
			Range: Uint16Range,
		},
		"uint32": {
			Name:  "uint32",
			Kind:  Yuint32,
			Range: Uint32Range,
		},
		"uint64": {
			Name:  "uint64",
			Kind:  Yuint64,
			Range: Uint64Range,
		},

		"decimal64": {
			Name:  "decimal64",
			Kind:  Ydecimal64,
			Range: Decimal64Range,
		},
		"string": {
			Name: "string",
			Kind: Ystring,
		},
		"boolean": {
			Name: "boolean",
			Kind: Ybool,
		},
		"enumeration": {
			Name: "enumeration",
			Kind: Yenum,
		},
		"bits": {
			Name: "bits",
			Kind: Ybits,
		},
		"binary": {
			Name: "binary",
			Kind: Ybinary,
		},
		"leafref": {
			Name: "leafref",
			Kind: Yleafref,
		},
		"identityref": {
			Name: "identityref",
			Kind: Yidentityref,
		},
		"empty": {
			Name: "empty",
			Kind: Yempty,
		},
		"union": {
			Name: "union",
			Kind: Yunion,
		},
		"instance-identifier": {
			Name: "instance-identifier",
			Kind: YinstanceIdentifier,
		},
	}
)

// Install builtin types as know types
func init() {
	for k, v := range baseTypes {
		// Base types are always their own root
		v.Root = v
		BaseTypedefs[k] = v.typedef()
	}
}

// TypeKind is the enumeration of the base types available in YANG.  It
// is analogous to reflect.Kind.
type TypeKind uint

func (k TypeKind) String() string {
	if s := TypeKindToName[k]; s != "" {
		return s
	}
	return fmt.Sprintf("unknown-type-%d", k)
}

const (
	// Ynone represents the invalid (unset) type.
	Ynone = TypeKind(iota)
	// Yint8 is an int in the range [-128, 127].
	Yint8
	// Yint16 is an int in the range [-32768, 32767].
	Yint16
	// Yint32 is an int in the range [-2147483648, 2147483647].
	Yint32
	// Yint64 is an int in the range [-9223372036854775808, 9223372036854775807]
	Yint64
	// Yuint8 is an int in the range [0, 255]
	Yuint8
	// Yuint16 is an int in the range [0, 65535]
	Yuint16
	// Yuint32 is an int in the range [0, 4294967295]
	Yuint32
	// Yuint64 is an int in the range [0, 18446744073709551615]
	Yuint64

	// Ybinary stores arbitrary data.
	Ybinary
	// Ybits is a named set of bits or flags.
	Ybits
	// Ybool is true or false.
	Ybool
	// Ydecimal64 is a signed decimal number.
	Ydecimal64
	// Yempty has no associated value.
	Yempty
	// Yenum stores enumerated strings.
	Yenum
	// Yidentityref stores an extensible enumeration.
	Yidentityref
	// YinstanceIdentifier stores a reference to a data tree node.
	YinstanceIdentifier
	// Yleafref stores a reference to a leaf instance.
	Yleafref
	// Ystring is a human readable string.
	Ystring
	// Yunion is a choice of types.
	Yunion
)

// A YangType is the internal representation of a type in YANG.  It may
// refer to either a builtin type or type specified with typedef.  Not
// all fields in YangType are used for all types.
type YangType struct {
	Name             string
	Kind             TypeKind    // Ynone if not a base type
	Base             *Type       `json:"-"`          // Base type for non-builtin types
	IdentityBase     *Identity   `json:",omitempty"` // Base statement for a type using identityref
	Root             *YangType   `json:"-"`          // root of this type that is the same
	Bit              *EnumType   `json:",omitempty"` // bit position, "status" is lost
	Enum             *EnumType   `json:",omitempty"` // enum name to value, "status" is lost
	Units            string      `json:",omitempty"` // units to be used for this type
	Default          string      `json:",omitempty"` // default value, if any
	FractionDigits   int         `json:",omitempty"` // decimal64 fixed point precision
	Length           YangRange   `json:",omitempty"` // this should be processed by section 12
	OptionalInstance bool        `json:",omitempty"` // !require-instances which defaults to true
	Path             string      `json:",omitempty"` // the path in a leafref
	Pattern          []string    `json:",omitempty"` // limiting XSD-TYPES expressions on strings
	POSIXPattern     []string    `json:",omitempty"` // limiting POSIX ERE on strings (specified by openconfig-extensions:posix-pattern)
	Range            YangRange   `json:",omitempty"` // range for integers
	Type             []*YangType `json:",omitempty"` // for unions
}

// Equal returns true if y and t describe the same type.
func (y *YangType) Equal(t *YangType) bool {
	switch {
	case
		// Don't check the Name, it contains no information
		y.Kind != t.Kind,
		y.Units != t.Units,
		y.Default != t.Default,
		y.FractionDigits != t.FractionDigits,
		y.IdentityBase != t.IdentityBase,
		len(y.Length) != len(t.Length),
		!y.Length.Equal(t.Length),
		y.OptionalInstance != t.OptionalInstance,
		y.Path != t.Path,
		!ssEqual(y.Pattern, t.Pattern),
		!ssEqual(y.POSIXPattern, t.POSIXPattern),
		len(y.Range) != len(t.Range),
		!y.Range.Equal(t.Range),
		!tsEqual(y.Type, t.Type),
		!cmp.Equal(y.Enum, t.Enum, cmp.Comparer(func(t, u EnumType) bool {
			return cmp.Equal(t.unique, u.unique) && cmp.Equal(t.toInt, u.toInt) && cmp.Equal(t.toString, u.toString)
		})):

		return false
	}
	// TODO(borman): Base, Bit
	return true
}

// typedef returns a Typedef created from y for insertion into the BaseTypedefs
// map.
func (y *YangType) typedef() *Typedef {
	return &Typedef{
		Name:   y.Name,
		Source: &Statement{},
		Type: &Type{
			Name:     y.Name,
			Source:   &Statement{},
			YangType: y,
		},
		YangType: y,
	}
}

// ssEqual returns true if the two slices are equivalent.
func ssEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for x, s := range s1 {
		if s != s2[x] {
			return false
		}
	}
	return true
}

// tsEqual returns true if the two Type slices are identical.
func tsEqual(t1, t2 []*YangType) bool {
	if len(t1) != len(t2) {
		return false
	}
	// For now we compare absolute pointers.
	// This may be wrong.
	for x, t := range t1 {
		if !t.Equal(t2[x]) {
			return false
		}
	}
	return true
}
