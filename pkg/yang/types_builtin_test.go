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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/errdiff"
)

const (
	useMin = -999
	useMax = 999
)

// R is a test helper for creating an int-based YRange.
func R(a, b int64) YRange {
	n1 := FromInt(a)
	n2 := FromInt(b)
	if a == useMin {
		n1 = minNumber
	}
	if b == useMax {
		n2 = maxNumber
	}
	return YRange{n1, n2}
}

// Rf is a test helper for creating a float-based YRange.
func Rf(a, b int64, fracDig uint8) YRange {
	n1 := Number{Value: uint64(a), FractionDigits: fracDig}
	n2 := Number{Value: uint64(b), FractionDigits: fracDig}
	if a < 0 {
		n1.Value = uint64(-a)
		n1.Kind = Negative
	}
	if b < 0 {
		n2.Value = uint64(-b)
		n2.Kind = Negative
	}
	if a == useMin {
		n1 = minNumber
	}
	if b == useMax {
		n2 = maxNumber
	}
	return YRange{n1, n2}
}

func TestRangeEqual(t *testing.T) {
	for x, tt := range []struct {
		r1, r2 YangRange
		ok     bool
	}{
		{ok: true},                          // empty range contained in empty range
		{r1: YangRange{R(1, 2)}, ok: false}, // empty range contained in range
		{r2: YangRange{R(1, 2)}, ok: false}, // range contained in empty range
		{
			YangRange{R(1, 2)},
			YangRange{R(1, 2)},
			true,
		},
		{
			YangRange{R(1, 3)},
			YangRange{R(1, 2)},
			false,
		},
		{
			YangRange{R(1, 2), R(4, 5)},
			YangRange{R(1, 2), R(4, 5)},
			true,
		},
		{
			YangRange{R(1, 2), R(4, 6)},
			YangRange{R(1, 2), R(4, 5)},
			false,
		},
		{
			YangRange{R(1, 2)},
			YangRange{R(1, 2), R(4, 5)},
			false,
		},
		{
			YangRange{R(1, 2), R(4, 5)},
			YangRange{R(1, 2)},
			false,
		},
	} {
		if ok := tt.r1.Equal(tt.r2); ok != tt.ok {
			t.Errorf("#%d: got %v, want %v", x, ok, tt.ok)
		}
	}
}

func TestRangeContains(t *testing.T) {
	for x, tt := range []struct {
		r1, r2 YangRange
		ok     bool
	}{
		{ok: true},
		{r1: YangRange{R(1, 2)}, ok: true},
		{r2: YangRange{R(1, 2)}, ok: true},
		{
			r1: YangRange{R(1, 2)},
			r2: YangRange{R(1, 2)},
			ok: true,
		},
		{
			r1: YangRange{R(1, 5)},
			r2: YangRange{R(2, 3)},
			ok: true,
		},
		{
			r1: YangRange{R(2, 3)},
			r2: YangRange{R(1, 5)},
			ok: false,
		},
		{
			r1: YangRange{R(1, 10)},
			r2: YangRange{R(1, 2), R(4, 5), R(7, 10)},
			ok: true,
		},
		{
			r1: YangRange{R(1, 10)},
			r2: YangRange{R(1, 2), R(7, 11)},
			ok: false,
		},
		{
			r1: YangRange{R(1, 9), R(11, 19), R(21, 29)},
			r2: YangRange{R(23, 25)},
			ok: true,
		},
		{
			r1: YangRange{R(1, 9), R(11, 19), R(21, 29)},
			r2: YangRange{R(23, 23)},
			ok: true,
		},
		{
			r1: YangRange{R(1, 9), R(11, 19), R(21, 29)},
			r2: YangRange{R(20, 20)},
			ok: false,
		},
		{
			r1: YangRange{R(1, 10)},
			r2: YangRange{R(useMin, useMax)},
			ok: true,
		},
		{
			r1: YangRange{R(useMin, useMax)},
			r2: YangRange{R(1, 10)},
			ok: true,
		},
		{
			r1: YangRange{R(1024, 65535)},
			r2: YangRange{R(useMin, 4096), R(5120, useMax)},
			ok: true,
		},
		{
			r1: YangRange{R(1024, 65535)},
			r2: YangRange{R(-999999, 4096), R(5120, useMax)},
			ok: false,
		},
		{
			r1: YangRange{R(1024, 65535)},
			r2: YangRange{R(useMin, 4096), R(5120, 999999)},
			ok: false,
		},
	} {
		if ok := tt.r1.Contains(tt.r2); ok != tt.ok {
			t.Errorf("#%d: got %v, want %v", x, ok, tt.ok)
		}
	}
}

func TestParseRangesInt(t *testing.T) {
	tests := []struct {
		desc             string
		in               string
		want             YangRange
		wantErrSubstring string
	}{{
		desc: "small numbers, coalescing",
		in:   "0|2..3|4..5",
		want: YangRange{R(0, 0), R(2, 5)},
	}, {
		desc: "small numbers, out of order, coalescing",
		in:   "4..5|0|2..3",
		want: YangRange{R(0, 0), R(2, 5)},
	}, {
		desc:             "invalid input: too many ..s",
		in:               "0|2..3|4..5..6",
		wantErrSubstring: "too many '..' in 4..5..6",
	}, {
		desc:             "invalid input: range boundaries out of order",
		in:               "0|2..3|5..4",
		wantErrSubstring: "range boundaries out of order",
	}, {
		desc: "range with min",
		in:   "min..0|2..3|4..5",
		want: YangRange{R(useMin, 0), R(2, 5)},
	}, {
		desc: "range with max",
		in:   "min..0|2..3|4..5|7..max",
		want: YangRange{R(useMin, 0), R(2, 5), R(7, useMax)},
	}, {
		desc: "coalescing from min to max",
		in:   "min..0|1..max",
		want: YangRange{R(useMin, useMax)},
	}, {
		desc:             "spelling error",
		in:               "mean..0|1..max",
		wantErrSubstring: "invalid syntax",
	}, {
		desc: "big numbers, coalescing",
		in:   "0..69|4294967294|4294967295",
		want: YangRange{R(0, 69), R(4294967294, 4294967295)},
	}, {
		desc: "no ranges",
		in:   "250|500|1000",
		want: YangRange{R(250, 250), R(500, 500), R(1000, 1000)},
	}, {
		desc: "no ranges unsorted",
		in:   "1000|500|250",
		want: YangRange{R(250, 250), R(500, 500), R(1000, 1000)},
	}, {
		desc: "negative numbers",
		in:   "-31..-1|1..31",
		want: YangRange{R(-31, -1), R(1, 31)},
	}, {
		desc: "spaces",
		in:   "-22 | -15 | -7 | 0",
		want: YangRange{R(-22, -22), R(-15, -15), R(-7, -7), R(0, 0)},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := ParseRangesInt(tt.in)
			if err != nil {
				if diff := errdiff.Substring(err, tt.wantErrSubstring); diff != "" {
					t.Fatalf("did not get expected error, %s", diff)
				}
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}

func TestCoalesce(t *testing.T) {
	for x, tt := range []struct {
		in, out YangRange
	}{
		{},
		{YangRange{R(1, 4)}, YangRange{R(1, 4)}},
		{YangRange{R(1, 2), R(3, 4)}, YangRange{R(1, 4)}},
		{YangRange{Rf(10, 25, 1), Rf(30, 40, 1)}, YangRange{Rf(10, 25, 1), Rf(30, 40, 1)}},
		{YangRange{Rf(10, 29, 1), Rf(30, 40, 1)}, YangRange{Rf(10, 40, 1)}},
		{YangRange{R(1, 2), R(2, 4)}, YangRange{R(1, 4)}},
		{YangRange{R(1, 2), R(4, 5)}, YangRange{R(1, 2), R(4, 5)}},
		{YangRange{R(1, 3), R(2, 5)}, YangRange{R(1, 5)}},
		{YangRange{R(1, 10), R(2, 5)}, YangRange{R(1, 10)}},
		{YangRange{R(1, 10), R(1, 2), R(4, 5), R(7, 8)}, YangRange{R(1, 10)}},
		{YangRange{Rf(1, 10, 3), Rf(1, 2, 3), Rf(4, 5, 3), Rf(7, 8, 3)}, YangRange{Rf(1, 10, 3)}},
	} {
		out := coalesce(tt.in)
		if !out.Equal(tt.out) {
			t.Errorf("#%d: got %v, want %v", x, out, tt.out)
		}
	}
}

func TestParseRangesDecimal(t *testing.T) {
	tests := []struct {
		desc             string
		in               string
		inFracDig        uint8
		want             YangRange
		wantErrSubstring string
	}{{
		desc:      "small decimals",
		in:        "0.0|2.0..30.0|1.34..1.99",
		inFracDig: 2,
		want:      YangRange{Rf(0, 0, 2), Rf(134, 3000, 2)},
	}, {
		desc:      "small decimals with coalescing",
		in:        "0.0|2.0..30.0",
		inFracDig: 1,
		want:      YangRange{Rf(0, 0, 1), Rf(20, 300, 1)},
	}, {
		desc:             "fractional digit cannot be too high",
		in:               "0.0|2.0..30.0",
		inFracDig:        19,
		wantErrSubstring: "invalid number of fraction digits",
	}, {
		desc:             "fractional digit cannot be 0",
		in:               "0.0|2.0..30.0",
		inFracDig:        0,
		wantErrSubstring: "invalid number of fraction digits",
	}, {
		desc:      "big decimals",
		in:        "0.0..69|4294967294.1234|4294967295.1234",
		inFracDig: 4,
		want:      YangRange{Rf(0, 690000, 4), Rf(42949672941234, 42949672941234, 4), Rf(42949672951234, 42949672951234, 4)},
	}, {
		desc:      "small decimals, out of order",
		in:        "4.0..5.55|0|2.32..3.23",
		inFracDig: 3,
		want:      YangRange{Rf(0, 0, 3), Rf(2320, 3230, 3), Rf(4000, 5550, 3)},
	}, {
		desc:             "invalid input: too many ..s",
		in:               "4.0..5.55..6.66|0|2.32..3.23",
		inFracDig:        3,
		wantErrSubstring: "too many '..'",
	}, {
		desc:             "invalid input: range boundaries out of order",
		in:               "5..4.0|0|2.32..3.23",
		inFracDig:        3,
		wantErrSubstring: "range boundaries out of order",
	}, {
		desc:      "range with min",
		in:        "4.0..5.55|min..0|2.32..3.23",
		inFracDig: 3,
		want:      YangRange{Rf(useMin, 0, 3), Rf(2320, 3230, 3), Rf(4000, 5550, 3)},
	}, {
		desc:      "range with max",
		in:        "4.0..max|min..0|2.32..3.23",
		inFracDig: 3,
		want:      YangRange{Rf(useMin, 0, 3), Rf(2320, 3230, 3), Rf(4000, useMax, 3)},
	}, {
		desc:      "coalescing from min to max",
		in:        "min..0.9|1..max",
		inFracDig: 1,
		want:      YangRange{Rf(useMin, useMax, 1)},
	}, {
		desc:             "spelling error",
		in:               "min..0.9|1..masks",
		inFracDig:        1,
		wantErrSubstring: "invalid syntax",
	}, {
		desc:      "no ranges",
		in:        "250.55|500.0|1000",
		inFracDig: 2,
		want:      YangRange{Rf(25055, 25055, 2), Rf(50000, 50000, 2), Rf(100000, 100000, 2)},
	}, {
		desc:      "no ranges unsorted",
		in:        "1000|500.0|250.55",
		inFracDig: 2,
		want:      YangRange{Rf(25055, 25055, 2), Rf(50000, 50000, 2), Rf(100000, 100000, 2)},
	}, {
		desc:      "negative decimals",
		in:        "-31.2..-1.5|1.5..31.2",
		inFracDig: 1,
		want:      YangRange{Rf(-312, -15, 1), Rf(15, 312, 1)},
	}, {
		desc:      "spaces",
		in:        "-22.5 | -15 | -7.5 | 0",
		inFracDig: 1,
		want:      YangRange{Rf(-225, -225, 1), Rf(-150, -150, 1), Rf(-75, -75, 1), Rf(0, 0, 1)},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := ParseRangesDecimal(tt.in, tt.inFracDig)
			if err != nil {
				if diff := errdiff.Substring(err, tt.wantErrSubstring); diff != "" {
					t.Fatalf("did not get expected error, %s", diff)
				}
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		desc  string
		inVal Number
		inAdd uint64
		want  Number
	}{{
		desc:  "add one to integer",
		inVal: FromInt(1),
		inAdd: 1,
		want:  FromInt(2),
	}, {
		desc:  "add one to decimal64",
		inVal: FromFloat(1.0),
		inAdd: 1,
		want:  FromFloat(1.1),
	}, {
		desc:  "negative int becomes positive",
		inVal: FromInt(-2),
		inAdd: 3,
		want:  FromInt(1),
	}, {
		desc:  "negative int stays negative",
		inVal: FromInt(-3),
		inAdd: 1,
		want:  FromInt(-2),
	}, {
		desc:  "negative decimal becomes positive",
		inVal: FromFloat(-2),
		inAdd: 35,
		want:  FromFloat(1.5),
	}, {
		desc:  "negative decimal stays negative",
		inVal: FromFloat(-42.22),
		inAdd: 4122,
		want:  FromFloat(-1.0),
	}, {
		desc:  "explicitly set fraction digits",
		inVal: Number{Value: 10000, FractionDigits: 5},
		inAdd: 1,
		want:  Number{Value: 10001, FractionDigits: 5},
	}, {
		desc:  "explicitly set fraction digits - negative",
		inVal: Number{Value: 0, FractionDigits: 3},
		inAdd: 42,
		want:  FromFloat(0.042),
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := tt.inVal.addQuantum(tt.inAdd)
			if !cmp.Equal(got, tt.want) {
				t.Fatalf("did get expected result, got: %s, want: %s", got.String(), tt.want.String())
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		desc             string
		inStr            string
		want             Number
		wantErrSubstring string
	}{{
		desc:             "invalid string supplied",
		inStr:            "fish",
		wantErrSubstring: "valid syntax",
	}, {
		desc:  "negative int",
		inStr: "-42",
		want:  FromInt(-42),
	}, {
		desc:  "positive int",
		inStr: "42",
		want:  FromInt(42),
	}, {
		desc:  "positive int with plus sign",
		inStr: "+42",
		want:  FromInt(42),
	}, {
		desc:  "zero",
		inStr: "0",
		want:  FromInt(0),
	}, {
		desc:  "min",
		inStr: "min",
		want:  minNumber,
	}, {
		desc:  "max",
		inStr: "max",
		want:  maxNumber,
	}, {
		desc:             "just a sign",
		inStr:            "-",
		wantErrSubstring: "sign with no value",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := ParseInt(tt.inStr)
			if err != nil {
				if diff := errdiff.Substring(err, tt.wantErrSubstring); diff != "" {
					t.Fatalf("did not get expected error, %s", diff)
				}
				return
			}

			if !cmp.Equal(got, tt.want) {
				t.Errorf("did not get expected Number, got: %s, want: %s", got, tt.want)
			}

			if got.IsDecimal() {
				t.Errorf("Got decimal value instead of int: %v", got)
			}
		})
	}
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		desc                    string
		inStr                   string
		inFracDig               uint8
		skipFractionDigitsCheck bool
		want                    Number
		wantErrSubstring        string
	}{{
		desc:             "too few fractional digits",
		inStr:            "1.000",
		inFracDig:        0,
		wantErrSubstring: "invalid number of fraction digits",
	}, {
		desc:             "too many fraction digits",
		inStr:            "1.000",
		inFracDig:        24,
		wantErrSubstring: "invalid number of fraction digits",
	}, {
		desc:             "more digits supplied",
		inStr:            "1.14242",
		inFracDig:        2,
		wantErrSubstring: "has too much precision",
	}, {
		desc:      "single digit precision",
		inStr:     "1.1",
		inFracDig: 1,
		want:      Number{Value: 11, FractionDigits: 1},
	}, {
		desc:                    "max precision",
		inStr:                   "0.100000000000000000",
		inFracDig:               18,
		skipFractionDigitsCheck: true,
		want:                    FromFloat(0.1),
	}, {
		desc:                    "max precision but not supplied",
		inStr:                   "0.1",
		inFracDig:               4,
		skipFractionDigitsCheck: true,
		want:                    FromFloat(0.1),
	}, {
		desc:             "invalid string supplied",
		inStr:            "fish",
		inFracDig:        17,
		wantErrSubstring: "not a valid decimal number",
	}, {
		desc:      "negative number",
		inStr:     "-42.0",
		inFracDig: 1,
		want:      FromFloat(-42),
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := ParseDecimal(tt.inStr, tt.inFracDig)
			if err != nil {
				if diff := errdiff.Substring(err, tt.wantErrSubstring); diff != "" {
					t.Fatalf("did not get expected error, %s", diff)
				}
				return
			}

			if !cmp.Equal(got, tt.want) {
				t.Errorf("did not get expected Number, got: %s, want: %s", got, tt.want)
			}

			if !tt.skipFractionDigitsCheck {
				if got, want := got.FractionDigits, tt.want.FractionDigits; got != want {
					t.Errorf("fractional digits not equal, got: %d, want: %d", got, want)
				}
			}

			if !got.IsDecimal() {
				t.Errorf("Got non-decimal value: %v", got)
			}
		})
	}
}

func TestNumberString(t *testing.T) {
	tests := []struct {
		desc string
		in   Number
		want string
	}{{
		desc: "min",
		in:   Number{Kind: MinNumber},
		want: "min",
	}, {
		desc: "max",
		in:   Number{Kind: MaxNumber},
		want: "max",
	}, {
		desc: "integer",
		in:   Number{Value: 1},
		want: "1",
	}, {
		desc: "negative integer",
		in:   Number{Value: 1, Kind: Negative},
		want: "-1",
	}, {
		desc: "decimal, fractional digits = 1",
		in:   Number{Value: 1, FractionDigits: 1},
		want: "0.1",
	}, {
		desc: "decimal, fractional digits = 18",
		in:   Number{Value: 123456789012345678, FractionDigits: 18},
		want: "0.123456789012345678",
	}, {
		desc: "negative decimal",
		in:   Number{Value: 100, FractionDigits: 2, Kind: Negative},
		want: "-1.00",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Fatalf("did not get expected number, got: %s, want: %s", got, tt.want)
			}
		})
	}
}
