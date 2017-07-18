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

import (
	"fmt"
	"testing"
)

// DebugString returns n's internal represenatation as a string.
func (n Number) DebugString() string {
	return fmt.Sprintf("{%v, %d, %d}", n.Kind, n.Value, n.FractionDigits)
}

// errToStr outputs e's error string if it is not-nil, or an empty string
// otherwise.
func errToStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func TestNumberParse(t *testing.T) {
	tests := []struct {
		desc      string
		numString string
		want      Number
		wantErr   string
	}{{
		desc:      "+ve int",
		numString: "123",
		want:      Number{Kind: Positive, Value: 123},
	}, {
		desc:      "-ve int",
		numString: "-123",
		want:      Number{Kind: Negative, Value: 123},
	}, {
		desc:      "+ve float",
		numString: "123.123",
		want:      Number{Kind: Positive, Value: 123123, FractionDigits: 3},
	}, {
		desc:      "+ve float, no leading 0",
		numString: ".123",
		want:      Number{Kind: Positive, Value: 123, FractionDigits: 3},
	}, {
		desc:      "+ve float, leading 0",
		numString: "0.123",
		want:      Number{Kind: Positive, Value: 123, FractionDigits: 3},
	}, {
		desc:      "-ve float, small value",
		numString: "-0.0123",
		want:      Number{Kind: Negative, Value: 123, FractionDigits: 4},
	}, {
		desc:      "+ve float, small value",
		numString: "0.0123",
		want:      Number{Kind: Positive, Value: 123, FractionDigits: 4},
	}, {
		desc:      "-ve float",
		numString: "-123.123",
		want:      Number{Kind: Negative, Value: 123123, FractionDigits: 3},
	}, {
		desc:      "bad string",
		numString: "abc",
		want:      Number{},
		wantErr:   `abc is not a valid decimal number: strconv.ParseInt: parsing "abc": invalid syntax`,
	}, {
		desc:      "overflow ParseInt",
		numString: "123456789123456789123456789",
		want:      Number{},
		wantErr:   `123456789123456789123456789 is not a valid decimal number: strconv.ParseInt: parsing "123456789123456789123456789": value out of range`,
	}, {
		desc:      "+ve range edge",
		numString: "922337203685477580.7",
		want:      Number{Kind: Positive, Value: 9223372036854775807, FractionDigits: 1},
	}, {
		desc:      "-ve range edge",
		numString: "-922337203685477580.8",
		want:      Number{Kind: Negative, Value: 9223372036854775808, FractionDigits: 1},
	}, {
		desc:      "overflow range +ve, frac digits 1",
		numString: "922337203685477580.8",
		want:      Number{},
		wantErr:   `922337203685477580.8 is not a valid decimal number: strconv.ParseInt: parsing "9223372036854775808": value out of range`,
	}, {
		desc:      "overflow range -ve, frac digits 1",
		numString: "-922337203685477580.9",
		want:      Number{},
		wantErr:   `-922337203685477580.9 is not a valid decimal number: strconv.ParseInt: parsing "-9223372036854775809": value out of range`,
	}, {
		desc:      "overflow range +ve, frac digits 18",
		numString: "9.223372036854775808",
		want:      Number{},
		wantErr:   `9.223372036854775808 is not a valid decimal number: strconv.ParseInt: parsing "9223372036854775808": value out of range`,
	}, {
		desc:      "overflow range -ve, frac digits 18",
		numString: "-9.223372036854775809",
		want:      Number{},
		wantErr:   `-9.223372036854775809 is not a valid decimal number: strconv.ParseInt: parsing "-9223372036854775809": value out of range`,
	}, {
		desc:      "overflow range, frac digits 19",
		numString: "9.2233720368547758090",
		want:      Number{},
		wantErr:   `9.2233720368547758090 is not a valid decimal number: strconv.ParseInt: parsing "92233720368547758090": value out of range`,
	}}

	for _, tt := range tests {
		n, err := ParseNumber(tt.numString)
		if got, want := errToStr(err), tt.wantErr; got != want {
			t.Errorf("%s: got error: %v, want error: %v", tt.desc, got, want)
		}
		if got, want := n, tt.want; tt.wantErr == "" && !got.Equal(want) {
			t.Errorf("%s: got: %v, want: %v", tt.desc, got, want)
		}
		if err == nil {
			want := tt.numString
			if want[0] == '.' {
				want = "0" + want
			}
			if got := n.String(); got != want {
				t.Errorf("%s: got %q, want %q", tt.desc, got, want)
			}
		}
	}
}

func TestNumberFromFloat(t *testing.T) {
	tests := []struct {
		desc string
		num  float64
		want Number
	}{{
		desc: "+ve integer",
		num:  123,
		want: Number{Kind: Positive, Value: 123},
	}, {
		desc: "-ve integer",
		num:  -123,
		want: Number{Kind: Negative, Value: 123},
	}, {
		desc: "+ve float",
		num:  123.123,
		want: Number{Kind: Positive, Value: 123123, FractionDigits: 3},
	}, {
		desc: "-ve float",
		num:  -123.123,
		want: Number{Kind: Negative, Value: 123123, FractionDigits: 3},
	}, {
		desc: "+ve max",
		num:  float64(MaxInt64),
		want: Number{Kind: Positive, Value: 9223372036854775808},
	}, {
		desc: "-ve max",
		num:  float64(MinInt64),
		want: Number{Kind: Negative, Value: 9223372036854775808},
	}, {
		desc: "+ve overflow",
		num:  999e99,
		want: maxNumber,
	}, {
		desc: "-ve overflow",
		num:  -999e99,
		want: minNumber,
	}}

	for _, tt := range tests {
		n := FromFloat(tt.num)
		if got, want := n, tt.want; !got.Equal(want) {
			t.Errorf("%s: got: %s, want: %s", tt.desc, got.DebugString(), want.DebugString())
		}

	}
}

func TestNumberIsDecimal(t *testing.T) {
	for x, tt := range []struct {
		n  Number
		ok bool
	}{
		{FromInt(42), false},
		{FromInt(-41), false},
		{minNumber, false},
		{maxNumber, false},
		{FromFloat(42.42), true},
		{FromFloat(-42.42), true},
		{Number{Kind: Positive, Value: 42}, false},
	} {
		ok := tt.n.IsDecimal()
		if ok != tt.ok {
			t.Errorf("#%d: got %v, want %v", x, ok, tt.ok)
		}
	}
}

func TestNumberLess(t *testing.T) {
	for x, tt := range []struct {
		n1, n2 Number
		ok     bool
	}{
		{FromInt(42), FromInt(42), false},
		{FromInt(41), FromInt(42), true},
		{FromInt(42), FromInt(41), false},
		{FromInt(-10), FromInt(10), true},
		{FromInt(-10), FromInt(1), true},
		{FromInt(-10), FromInt(-1), true},
		{FromInt(2), FromInt(-1), false},
		{minNumber, minNumber, false},
		{minNumber, maxNumber, true},
		{minNumber, FromInt(42), true},
		{minNumber, FromInt(0), true},
		{minNumber, FromInt(-42), true},
		{maxNumber, maxNumber, false},
		{maxNumber, minNumber, false},
		{maxNumber, FromInt(42), false},
		{maxNumber, FromInt(0), false},
		{FromInt(-42), maxNumber, true},
		{FromInt(0), maxNumber, true},
		{FromInt(42), maxNumber, true},
		{FromFloat(42.42), FromFloat(42.42), false},
		{FromFloat(41.42), FromFloat(42.42), true},
		{FromFloat(42.42), FromFloat(41.42), false},
		{FromFloat(42.42), FromFloat(42.421), true},
		{FromFloat(42.1), FromFloat(42.05), false},
		{FromFloat(41.421), FromFloat(42.42), true},
		{FromFloat(-10.42), FromFloat(10.42), true},
		{FromFloat(-10.42), FromFloat(1.42), true},
		{FromFloat(-10.42), FromFloat(-1.42), true},
		{FromFloat(-10.1), FromFloat(-10.05), true},
		{FromFloat(2.42), FromFloat(-1.42), false},
		{FromFloat(1234567890), FromFloat(0.123456789), false},
		{FromFloat(-1234567890), FromFloat(0.123456789), true},
		{minNumber, FromFloat(42.42), true},
		{minNumber, FromFloat(0.42), true},
		{minNumber, FromFloat(-42.42), true},
		{maxNumber, FromFloat(42.42), false},
		{maxNumber, FromFloat(0.42), false},
		{FromFloat(-42.42), maxNumber, true},
		{FromFloat(0.42), maxNumber, true},
		{FromFloat(42.42), maxNumber, true},
		{FromInt(42), FromFloat(42), false},
		{FromInt(41), FromFloat(42), true},
		{FromInt(42), FromFloat(42.42), true},
		{FromInt(-42), FromFloat(-42), false},
		{FromInt(-42), FromFloat(-41), true},
		{FromInt(-42), FromFloat(-42.42), false},
		{Number{Kind: Positive, Value: 9223372036854775807, FractionDigits: 0},
			Number{Kind: Positive, Value: 9223372036854775807, FractionDigits: 18}, false},
		{Number{Kind: Negative, Value: 9223372036854775808, FractionDigits: 0},
			Number{Kind: Positive, Value: 9223372036854775808, FractionDigits: 18}, true},
		{Number{Kind: Positive, Value: 9223372036854775807, FractionDigits: 1},
			Number{Kind: Positive, Value: 9223372036854775807, FractionDigits: 18}, false},
		{Number{Kind: Negative, Value: 9223372036854775808, FractionDigits: 1},
			Number{Kind: Positive, Value: 9223372036854775808, FractionDigits: 18}, true},
	} {
		ok := tt.n1.Less(tt.n2)
		if ok != tt.ok {
			t.Errorf("#%d: got %v, want %v", x, ok, tt.ok)
		}
	}
}

func TestNumberEqual(t *testing.T) {
	for x, tt := range []struct {
		n1, n2 Number
		ok     bool
	}{
		{FromInt(42), FromInt(42), true},
		{FromInt(41), FromInt(42), false},
		{FromInt(42), FromInt(41), false},
		{FromInt(-10), FromInt(-10), true},
		{FromInt(-10), FromInt(1), false},
		{FromInt(0), FromInt(0), true},
		{minNumber, minNumber, true},
		{minNumber, maxNumber, false},
		{minNumber, FromInt(42), false},
		{minNumber, FromInt(0), false},
		{minNumber, FromInt(-42), false},
		{maxNumber, maxNumber, true},
		{maxNumber, minNumber, false},
		{maxNumber, FromInt(42), false},
		{maxNumber, FromInt(0), false},
		{FromInt(-42), maxNumber, false},
		{FromInt(0), maxNumber, false},
		{FromInt(42), maxNumber, false},
		{FromFloat(42.42), FromFloat(42.42), true},
		{FromFloat(41.42), FromFloat(42.42), false},
		{FromFloat(-10.42), FromFloat(10.42), false},
		{FromFloat(-10.42), FromFloat(-10.42), true},
		{FromFloat(-10.42), FromFloat(1.42), false},
		{FromFloat(-10.42), FromFloat(-1.42), false},
		{FromFloat(2.42), FromFloat(-1.42), false},
		{minNumber, FromFloat(42.42), false},
		{minNumber, FromFloat(0.42), false},
		{minNumber, FromFloat(-42.42), false},
		{maxNumber, FromFloat(42.42), false},
		{maxNumber, FromFloat(0.42), false},
		{FromFloat(-42.42), maxNumber, false},
		{FromFloat(0.42), maxNumber, false},
		{FromFloat(42.42), maxNumber, false},
	} {
		ok := tt.n1.Equal(tt.n2)
		if ok != tt.ok {
			t.Errorf("#%d: got %v, want %v", x, ok, tt.ok)
		}
	}
}

func TestNumberAdd(t *testing.T) {
	for x, tt := range []struct {
		in  Number
		add uint64
		out Number
	}{
		{Number{Positive, 0, 0}, 1, Number{Positive, 1, 0}},
		{Number{Negative, 1, 0}, 1, Number{Positive, 0, 0}},
		{Number{Positive, 5, 0}, 12, Number{Positive, 17, 0}},
		{Number{Negative, 3, 0}, 10, Number{Positive, 7, 0}},
	} {
		out := tt.in.add(tt.add)
		if !out.Equal(tt.out) {
			t.Errorf("#%d: got %v, want %v", x, out, tt.out)
		}
	}
}
