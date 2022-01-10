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
	"testing"
)

func TestYangTypeEqual(t *testing.T) {

	tests := []struct {
		name      string
		inLeft    *YangType
		inRight   *YangType
		wantEqual bool
	}{{
		name:      "both-nil",
		inLeft:    nil,
		inRight:   nil,
		wantEqual: true,
	}, {
		name: "one-nil",
		inLeft: &YangType{
			Kind:           Ydecimal64,
			FractionDigits: 5,
		},
		inRight:   nil,
		wantEqual: false,
	}, {
		name: "name-unequal",
		inLeft: &YangType{
			Name:           "foo",
			Kind:           Ydecimal64,
			FractionDigits: 5,
		},
		inRight: &YangType{
			Name:           "bar",
			Kind:           Ydecimal64,
			FractionDigits: 5,
		},
		wantEqual: true,
	}, {
		name: "fraction-digits-unequal",
		inLeft: &YangType{
			Name:           "foo",
			Kind:           Ydecimal64,
			FractionDigits: 5,
		},
		inRight: &YangType{
			Name:           "foo",
			Kind:           Ydecimal64,
			FractionDigits: 4,
		},
		wantEqual: false,
	}, {
		name: "types-unequal",
		inLeft: &YangType{
			Name:           "foo",
			Kind:           Ydecimal64,
			FractionDigits: 5,
		},
		inRight: &YangType{
			Name: "foo",
			Kind: Yint64,
		},
		wantEqual: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotEqual := tt.inLeft.Equal(tt.inRight); gotEqual != tt.wantEqual {
				t.Errorf("gotEqual: %v, wantEqual: %v", gotEqual, tt.wantEqual)
			}
			// Must be symmetric
			if reverseEqual := tt.inRight.Equal(tt.inLeft); reverseEqual != tt.wantEqual {
				t.Errorf("got reverseEqual: %v, wantEqual: %v", reverseEqual, tt.wantEqual)
			}
		})
	}
}
