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

// This module contains all the builtin types as well as types related
// to types (such as ranges, enums, etc).

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// This file handles interpretation of types

// TypeKind is the enumeration of the base types available in YANG.  It
// is analogous to reflect.Kind.
type TypeKind uint

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

// TypeKindFromName maps the string name used in a YANG file to the enumerated
// TypeKind used in this library.
var TypeKindFromName = map[string]TypeKind{
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
var TypeKindToName = map[TypeKind]string{
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

func (k TypeKind) String() string {
	if s := TypeKindToName[k]; s != "" {
		return s
	}
	return fmt.Sprintf("unknown-type-%d", k)
}

// A EnumType represents a mapping of strings to integers.  It is used both
// for enumerations as well as bitfields.
type EnumType struct {
	last     int64 // maximum value assigned thus far
	min      int64 // minimum value allowed
	max      int64 // maximum value allowed
	unique   bool  // numeric values must be unique (enums)
	toString map[int64]string
	toInt    map[string]int64
}

// NewEnumType returns an initialized EnumType.
func NewEnumType() *EnumType {
	return &EnumType{
		last:     -1, // +1 will start at 0
		min:      MinEnum,
		max:      MaxEnum,
		unique:   true,
		toString: map[int64]string{},
		toInt:    map[string]int64{},
	}
}

// NewBitfield returns an EnumType initialized as a bitfield.  Multiple string
// values may map to the same numeric values.  Numeric values must be small
// non-negative integers.
func NewBitfield() *EnumType {
	return &EnumType{
		last:     -1, // +1 will start at 0
		min:      0,
		max:      MaxBitfieldSize - 1,
		toString: map[int64]string{},
		toInt:    map[string]int64{},
	}
}

// Set sets name in e to the provided value.  Set returns an error if the value
// is invalid, name is already signed, or when used as an enum rather than a
// bitfield, the value has previousl been used.  When two different names are
// assigned to the same value, the conversion from value to name will result in
// the most recently assigned name.
func (e *EnumType) Set(name string, value int64) error {
	if _, ok := e.toInt[name]; ok {
		return fmt.Errorf("field %s already assigned", name)
	}
	if oname, ok := e.toString[value]; e.unique && ok {
		return fmt.Errorf("fields %s and %s conflict on value %d", name, oname, value)
	}
	if value < e.min {
		return fmt.Errorf("value %d for %s too small (minimum is %d)", value, name, e.min)
	}
	if value > e.max {
		return fmt.Errorf("value %d for %s too large (maximum is %d)", value, name, e.max)
	}
	e.toString[value] = name
	e.toInt[name] = value
	if value >= e.last {
		e.last = value
	}
	return nil
}

// SetNext sets the name in e using the next possible value that is greater than
// all previous values.
func (e *EnumType) SetNext(name string) error {
	if e.last == MaxEnum {
		return fmt.Errorf("enum must specify value")
	}
	return e.Set(name, e.last+1)
}

// Name returns the name in e associated with value.  The empty string is
// returned if no name has been assigned to value.
func (e *EnumType) Name(value int64) string { return e.toString[value] }

// Value returns the value associated with name in e associated.  0 is returned
// if name is not in e, or if it is the first value in an unnumbered enum. Use
// IsDefined to definitively confirm name is in e.
func (e *EnumType) Value(name string) int64 { return e.toInt[name] }

// IsDefined returns true if name is defined in e, else false.
func (e *EnumType) IsDefined(name string) bool {
	_, defined := e.toInt[name]
	return defined
}

// Names returns the sorted list of enum string names.
func (e *EnumType) Names() []string {
	names := make([]string, len(e.toInt))
	i := 0
	for name := range e.toInt {
		names[i] = name
		i++
	}
	sort.Strings(names)
	return names
}

type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Values returns the sorted list of enum values.
func (e *EnumType) Values() []int64 {
	values := make([]int64, len(e.toInt))
	i := 0
	for _, value := range e.toInt {
		values[i] = value
		i++
	}
	sort.Sort(int64Slice(values))
	return values
}

// NameMap returns a map of names to values.
func (e *EnumType) NameMap() map[string]int64 {
	m := make(map[string]int64, len(e.toInt))
	for name, value := range e.toInt {
		m[name] = value
	}
	return m
}

// ValueMap returns a map of values to names.
func (e *EnumType) ValueMap() map[int64]string {
	m := make(map[int64]string, len(e.toString))
	for name, value := range e.toString {
		m[name] = value
	}
	return m
}

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
	Range            YangRange   `json:",omitempty"` // range for integers
	Type             []*YangType `json:",omitempty"` // for unions
}

// BaseTypedefs is a map of all base types to the Typedef structure manufactured
// for the type.
var BaseTypedefs = map[string]*Typedef{}

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
		len(y.Range) != len(t.Range),
		!y.Range.Equal(t.Range),
		!tsEqual(y.Type, t.Type):

		return false
	}
	// TODO(borman): Base, Bit, Enum
	return true
}

// Install builtin types as know types
func init() {
	for k, v := range baseTypes {
		// Base types are always their own root
		v.Root = v
		BaseTypedefs[k] = v.typedef()
	}
}

var baseTypes = map[string]*YangType{
	"int8": &YangType{
		Name:  "int8",
		Kind:  Yint8,
		Range: Int8Range,
	},
	"int16": &YangType{
		Name:  "int16",
		Kind:  Yint16,
		Range: Int16Range,
	},
	"int32": &YangType{
		Name:  "int32",
		Kind:  Yint32,
		Range: Int32Range,
	},
	"int64": &YangType{
		Name:  "int64",
		Kind:  Yint64,
		Range: Int64Range,
	},
	"uint8": &YangType{
		Name:  "uint8",
		Kind:  Yuint8,
		Range: Uint8Range,
	},
	"uint16": &YangType{
		Name:  "uint16",
		Kind:  Yuint16,
		Range: Uint16Range,
	},
	"uint32": &YangType{
		Name:  "uint32",
		Kind:  Yuint32,
		Range: Uint32Range,
	},
	"uint64": &YangType{
		Name:  "uint64",
		Kind:  Yuint64,
		Range: Uint64Range,
	},

	"decimal64": &YangType{
		Name:  "decimal64",
		Kind:  Ydecimal64,
		Range: Decimal64Range,
	},
	"string": &YangType{
		Name: "string",
		Kind: Ystring,
	},
	"boolean": &YangType{
		Name: "boolean",
		Kind: Ybool,
	},
	"enumeration": &YangType{
		Name: "enumeration",
		Kind: Yenum,
	},
	"bits": &YangType{
		Name: "bits",
		Kind: Ybits,
	},
	"binary": &YangType{
		Name: "binary",
		Kind: Ybinary,
	},
	"leafref": &YangType{
		Name: "leafref",
		Kind: Yleafref,
	},
	"identityref": &YangType{
		Name: "identityref",
		Kind: Yidentityref,
	},
	"empty": &YangType{
		Name: "empty",
		Kind: Yempty,
	},
	"union": &YangType{
		Name: "union",
		Kind: Yunion,
	},
	"instance-identifier": &YangType{
		Name: "instance-identifier",
		Kind: YinstanceIdentifier,
	},
}

// These are the default ranges defined by the YANG standard.
var (
	Int8Range  = mustParseRanges("-128..127")
	Int16Range = mustParseRanges("-32768..32767")
	Int32Range = mustParseRanges("-2147483648..2147483647")
	Int64Range = mustParseRanges("-9223372036854775808..9223372036854775807")

	Uint8Range  = mustParseRanges("0..255")
	Uint16Range = mustParseRanges("0..65535")
	Uint32Range = mustParseRanges("0..4294967295")
	Uint64Range = mustParseRanges("0..18446744073709551615")

	Decimal64Range = mustParseRanges("min..max")
)

const (
	// MaxInt64 corresponds to the maximum value of a signed int64.
	MaxInt64 = 1<<63 - 1
	// MinInt64 corresponds to the maximum value of a signed int64.
	MinInt64 = -1 << 63
	// AbsMinInt64 is the absolute value of MinInt64.
	AbsMinInt64 = 1 << 63
	// MaxEnum is the maximum value of an enumeration.
	MaxEnum = 1<<31 - 1
	// MinEnum is the minimum value of an enumeration.
	MinEnum = -1 << 31
	// MaxBitfieldSize is the maximum number of bits in a bitfield.
	MaxBitfieldSize = 1 << 32
	// MaxFractionDigits is the maximum number of fractional digits as per RFC6020 Section 9.3.4.
	MaxFractionDigits uint8 = 18
)

type NumberKind int

const (
	// Positive indicates that a Number is non-negative.
	Positive = NumberKind(iota)
	// Negative indicates that a Number is negative.
	Negative
	// MinNumber indicates that the Number is the minimum value allowed for the range.
	MinNumber
	// MaxNumber indicates that the Number is the maximum value allowed for the range.
	MaxNumber
)

const space18 = "000000000000000000" // used for prepending 0's

// A Number is either an integer the range of [-(1<<64) - 1, (1<<64)-1], or a
// YANG decimal conforming to https://tools.ietf.org/html/rfc6020#section-9.3.4.
type Number struct {
	// Kind is the kind of number (+/-ve, min/max).
	Kind NumberKind
	// Absolute value of the number.
	Value uint64
	// Number of fractional digits.
	FractionDigits uint8
}

var maxNumber = Number{Kind: MaxNumber}
var minNumber = Number{Kind: MinNumber}

// IsDecimal reports whether n is a decimal number.
func (n Number) IsDecimal() bool {
	return n.FractionDigits != 0
}

// FromInt creates a Number from an int64.
func FromInt(i int64) Number {
	if i < 0 {
		return Number{Kind: Negative, Value: uint64(-i)}
	}
	return Number{Kind: Positive, Value: uint64(i)}
}

// FromUint creates a Number from a uint64.
func FromUint(i uint64) Number {
	return Number{Kind: Positive, Value: i}
}

// FromFloat creates a Number from a float64. Input values with absolute value
// larger than MaxInt64/MinInt64 are converted into maxNumber/minNumber.
func FromFloat(f float64) Number {
	if f > float64(MaxInt64) {
		return maxNumber
	}
	if f < float64(MinInt64) {
		return minNumber
	}
	var fracDig uint8
	for ; Frac(f) != 0.0 && fracDig <= MaxFractionDigits; fracDig++ {
		f *= 10.0
	}
	v := uint64(f)
	kind := Positive
	if f < 0 {
		kind = Negative
		v = -v
	}

	n := Number{Kind: kind, Value: v, FractionDigits: fracDig}

	return n
}

// ParseNumber returns s as a Number.  Numbers may be represented in decimal,
// octal, or hexadecimal using the standard prefix notations (e.g., 0 and 0x)
func ParseNumber(s string) (n Number, err error) {
	s = strings.TrimSpace(s)
	switch s {
	case "max":
		return maxNumber, nil
	case "min":
		return minNumber, nil
	case "":
		return n, errors.New("converting empty string to number")
	case "+", "-":
		return n, errors.New("sign with no value")
	}

	n.Kind = Positive
	ns := s
	switch s[0] {
	case '+':
		ns = s[1:]
	case '-':
		n.Kind = Negative
		ns = s[1:]
	}
	n.Value, err = strconv.ParseUint(ns, 0, 64)
	if err == nil {
		return n, nil
	}

	return DecimalValueFromString(s, -1)
}

// DecimalValueFromString returns a decimal Number representation of inStr.
// If fracDigRequired is >= 0, the number is represented with fracDigRequired
// fractional digits, regardless of the precision of numStr, otherwise the
// precision of numStr is used to set the number of fractional digits.
// numStr must conform to Section 9.3.4.
func DecimalValueFromString(numStr string, fracDigRequired int) (n Number, err error) {
	if fracDigRequired > int(MaxFractionDigits) {
		return n, fmt.Errorf("too many fraction digits %d > max of %d", fracDigRequired, MaxFractionDigits)
	}

	s := numStr
	dx := strings.Index(s, ".")
	fracDig := 0
	if dx >= 0 {
		fracDig = len(s) - 1 - dx
		// remove first decimal, if dx > 1, will fail ParseInt below
		s = s[:dx] + s[dx+1:]
	}
	if fracDigRequired < 0 {
		fracDigRequired = fracDig
	}
	if fracDig > fracDigRequired {
		return n, fmt.Errorf("%s has too much precision, expect <= %d fractional digits", s, fracDigRequired)
	}

	s += space18[:fracDigRequired-fracDig]

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return n, fmt.Errorf("%s is not a valid decimal number: %s", numStr, err)
	}

	kind := Positive
	if v < 0 {
		kind = Negative
		v = -v
	}

	return Number{Kind: kind, Value: uint64(v), FractionDigits: uint8(fracDig)}, nil
}

// String returns n as a string in decimal.
func (n Number) String() string {
	var out string
	switch n.Kind {
	case MinNumber:
		return "min"
	case MaxNumber:
		return "max"
	}

	out += strconv.FormatUint(n.Value, 10)

	if n.IsDecimal() {
		fd := int(n.FractionDigits)
		if fd > 0 {
			ofd := len(out) - fd
			if ofd <= 0 {
				// We want 0.1 not .1
				out = space18[:-ofd+1] + out
				ofd = 1
			}
			out = out[:ofd] + "." + out[ofd:]
		}
	}
	if n.Kind == Negative {
		out = "-" + out
	}

	return out
}

// Int returns n as an int64.  It returns an error if n overflows an int64 or
// the number is decimal.
func (n Number) Int() (int64, error) {
	nv := n.Value
	if n.IsDecimal() {
		nv = n.Value / uint64(math.Pow10(int(n.FractionDigits)))
	}
	switch n.Kind {
	case MinNumber:
		return MinInt64, nil
	case MaxNumber:
		return MaxInt64, nil
	case Negative:
		switch {
		case nv == AbsMinInt64:
			return MinInt64, nil
		case nv < AbsMinInt64:
			return -int64(nv), nil
		}
	case Positive:
		if n.Value <= MaxInt64 {
			return int64(nv), nil
		}
		return 0, errors.New("signed integer overflow")
	default:
	}
	return 0, errors.New("unknown number type")
}

// add adds i to n without checking overflow.  We really only need to be
// able to add 1 for our code. panics if n is a decimal.
func (n Number) add(i uint64) Number {
	if n.IsDecimal() {
		panic("cannot call add() on decimal number " + n.String())
	}
	switch n.Kind {
	case MinNumber:
		return n
	case MaxNumber:
		return n
	case Negative:
		if n.Value <= i {
			n.Value = i - n.Value
			n.Kind = Positive
		} else {
			n.Value -= i
		}
	case Positive:
		n.Value += i
	default:
		panic("add to unknown number type")
	}
	return n
}

// Less returns true if n is less than m. Panics if n and m are a mix of integer
// and decimal.
func (n Number) Less(m Number) bool {
	switch {
	case m.Kind == MinNumber:
		return false
	case n.Kind == MinNumber:
		return true
	case n.Kind == MaxNumber:
		return false
	case m.Kind == MaxNumber:
		return true
	case n.Kind == Negative && m.Kind != Negative:
		return true
	case n.Kind != Negative && m.Kind == Negative:
		return false
	}

	nt, mt := n.Trunc(), m.Trunc()
	lt := nt < mt
	if nt == mt {
		nf, mf := n.frac(), m.frac()
		if nf == mf {
			return false
		}
		lt = nf < mf
	}

	if n.Kind == Negative {
		return !lt
	}
	return lt
}

// Equal returns true if n is equal to m.
func (n Number) Equal(m Number) bool {
	return !n.Less(m) && !m.Less(n)
}

// Trunc returns the whole part of abs(n) as a signed integer.
func (n Number) Trunc() uint64 {
	nv := n.Value
	e := pow10(n.FractionDigits)
	return nv / e
}

// frac returns the fraction part with a precision of 18 fractional digits.
// E.g. if n is 3.1 then n.frac() returns 100,000,000,000,000,000
func (n Number) frac() uint64 {
	frac := n.FractionDigits
	i := n.Trunc() * pow10(frac)
	return (n.Value - i) * pow10(uint8(18-frac))
}

// YRange is a single range of consecutive numbers, inclusive.
type YRange struct {
	Min Number
	Max Number
}

// Valid returns false if r is not a valid range (min > max).
func (r YRange) Valid() bool {
	return !r.Max.Less(r.Min)
}

// A YangRange is a set of non-overlapping ranges.
type YangRange []YRange

// ParseRanges parses s into a series of ranges.  Each individual range is
// in s is separated by the pipe character (|).  The min and max value of
// a range are separated by "..".  An error is returned if the range is
// invalid.  The resulting range is sorted and coalesced.
func ParseRanges(s string) (YangRange, error) {
	parts := strings.Split(s, "|")
	r := make(YangRange, len(parts))
	for i, s := range parts {
		parts := strings.Split(s, "..")
		min, err := ParseNumber(parts[0])
		if err != nil {
			return nil, err
		}
		var max Number
		switch len(parts) {
		case 1:
			max = min
		case 2:
			max, err = ParseNumber(parts[1])
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("two many ..'s in %s", s)
		}
		if max.Less(min) {
			return nil, fmt.Errorf("%s less than %s", max, min)
		}
		r[i] = YRange{min, max}
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}

	return coalesce(r), nil
}

// coalesce coalesces r into as few ranges as possible.  For example,
// 1..5|6..10 would become 1..10.  r is assumed to be sorted.
// r is assumed to be valid (see Validate)
func coalesce(r YangRange) YangRange {
	// coalesce the ranges if we have more than 1.
	if len(r) < 2 {
		return r
	}
	cr := make(YangRange, len(r))
	i := 0
	cr[i] = r[0]
	for _, r1 := range r[1:] {
		// r1.Min is always at least as large as cr[i].Min
		// Cases are:
		// r1 is contained in cr[i]
		// r1 starts inside of cr[i]
		// r1.Min cr[i].Max+1
		// r1 is beyond cr[i]
		if cr[i].Max.add(1).Less(r1.Min) {
			// r1 starts after cr[i], this is a new range
			i++
			cr[i] = r1
		} else if cr[i].Max.Less(r1.Max) {
			cr[i].Max = r1.Max
		}
	}
	return cr[:i+1]
}

func mustParseRanges(s string) YangRange {
	r, err := ParseRanges(s)
	if err != nil {
		panic(err)
	}
	return r
}

// String returns r as a string using YANG notation, either a simple
// value if min == max or min..max.
func (r YRange) String() string {
	if r.Min.Equal(r.Max) {
		return r.Min.String()
	}
	return r.Min.String() + ".." + r.Max.String()
}

// String returns the ranges r using YANG notation.  Individual ranges
// are separated by pipes (|).
func (r YangRange) String() string {
	s := make([]string, len(r))
	for i, r := range r {
		s[i] = r.String()
	}
	return strings.Join(s, "|")
}

func (r YangRange) Len() int      { return len(r) }
func (r YangRange) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r YangRange) Less(i, j int) bool {
	switch {
	case r[i].Min.Less(r[j].Min):
		return true
	case r[j].Min.Less(r[i].Min):
		return false
	default:
		return r[i].Max.Less(r[j].Max)
	}
}

// Validate sorts r and returns an error if r has either an invalid range or has
// overlapping ranges.
func (r YangRange) Validate() error {
	sort.Sort(r)
	switch {
	case len(r) == 0:
		return nil
	case !r[0].Valid():
		return errors.New("invalid number")
	}
	p := r[0]

	for _, n := range r[1:] {
		if n.Min.Less(p.Max) {
			return errors.New("overlapping ranges")
		}
	}
	return nil
}

// Equal returns true if ranges r and q are identically equivalent.
// TODO(borman): should we coalesce ranges in the comparison?
func (r YangRange) Equal(q YangRange) bool {
	if len(r) != len(q) {
		return false
	}
	for i, r := range r {
		if r != q[i] {
			return false
		}
	}
	return true
}

// Contains returns true if all possible values in s are also possible values
// in r.  An empty range is assumed to be min..max.
func (r YangRange) Contains(s YangRange) bool {
	if len(s) == 0 || len(r) == 0 {
		return true
	}

	rc := make(chan YRange)
	go func() {
		for _, v := range r {
			rc <- v
		}
		close(rc)
	}()

	// All ranges are sorted and coalesced which means each range
	// in s must exist

	// We know rc will always produce at least one value
	rr, ok := <-rc
	for _, ss := range s {
		// min is always within range
		if ss.Min.Kind != MinNumber {
			for rr.Max.Less(ss.Min) {
				rr, ok = <-rc
				if !ok {
					return false
				}
			}
		}
		if (ss.Max.Kind == MaxNumber) || (ss.Min.Kind == MinNumber) {
			continue
		}
		if ss.Min.Less(rr.Min) || rr.Max.Less(ss.Max) {
			return false
		}
	}
	return true
}

// Frac returns the fractional part of f.
func Frac(f float64) float64 {
	return f - math.Trunc(f)
}

// pow10 returns 10^e without checking for overflow.
func pow10(e uint8) uint64 {
	var out uint64 = 1
	for i := uint8(0); i < e; i++ {
		out *= 10
	}
	return out
}
