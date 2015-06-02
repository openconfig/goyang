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

import "testing"

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
	} {
		ok := tt.n1.Less(tt.n2)
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
		{Number{Positive, 0}, 1, Number{Positive, 1}},
		{Number{Negative, 1}, 1, Number{Positive, 0}},
		{Number{Positive, 5}, 12, Number{Positive, 17}},
		{Number{Negative, 3}, 10, Number{Positive, 7}},
	} {
		out := tt.in.add(tt.add)
		if !out.Equal(tt.out) {
			t.Errorf("#%d: got %v, want %v", x, out, tt.out)
		}
	}
}
