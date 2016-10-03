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
	"reflect"
	"testing"
)

func TestTypeResolve(t *testing.T) {
	for x, tt := range []struct {
		in  *Type
		err string
		out *YangType
	}{
		{
			in: &Type{
				Name: "int64",
			},
			out: &YangType{
				Name:  "int64",
				Kind:  Yint64,
				Range: Int64Range,
			},
		},
		{
			in: &Type{
				Name:           "boolean",
				FractionDigits: &Value{Name: "42"},
			},
			err: "unknown: fraction-digits only allowed for decimal64 values",
		},
		{
			in: &Type{
				Name: "decimal64",
			},
			err: "unknown: value is required in the range of [1..18]",
		},
		{
			in: &Type{
				Name: "identityref",
			},
			err: "unknown: an identityref must specify a base",
		},
		{
			in: &Type{
				Name:           "decimal64",
				FractionDigits: &Value{Name: "42"},
			},
			err: "unknown: value 42 out of range [1..18]",
		},
		{
			in: &Type{
				Name:           "decimal64",
				FractionDigits: &Value{Name: "7"},
			},
			out: &YangType{
				Name:           "decimal64",
				Kind:           Ydecimal64,
				FractionDigits: 7,
				Range:          Decimal64Range,
			},
		},
		// TODO(borman): Add in more tests as we honor more fields
		// in Type.
	} {
		// We can initialize a value to ourself, so to it here.
		errs := tt.in.resolve()

		// TODO(borman):  Do not hack out Root and Base.  These
		// are hacked out for now because they can be self-referential,
		// making construction of them difficult.
		tt.in.YangType.Root = nil
		tt.in.YangType.Base = nil

		switch {
		case tt.err == "" && len(errs) > 0:
			t.Errorf("#%d: unexpected errors: %v", x, errs)
		case tt.err != "" && len(errs) == 0:
			t.Errorf("#%d: did not get expected errors: %v", x, tt.err)
		case len(errs) > 1:
			t.Errorf("#%d: too many errors: %v", x, errs)
		case len(errs) == 1 && errs[0].Error() != tt.err:
			t.Errorf("#%d: got error %v, want %s", x, errs[0], tt.err)
		case len(errs) != 0:
		case !reflect.DeepEqual(tt.in.YangType, tt.out):
			t.Errorf("#%d: got %#v, want %#v", x, tt.in.YangType, tt.out)
		}
	}
}
