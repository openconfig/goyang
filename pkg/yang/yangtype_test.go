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
