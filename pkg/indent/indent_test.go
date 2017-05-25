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

package indent

import (
	"bytes"
	"errors"
	"testing"
)

var tests = []struct {
	prefix, in, out string
}{
	{
		"", "", "",
	}, {
		"--", "", "",
	}, {
		"", "x\nx", "x\nx",
	}, {
		"--", "x", "--x",
	}, {
		"--", "\n", "--\n",
	}, {
		"--", "\n\n", "--\n--\n",
	}, {
		"--", "x\n", "--x\n",
	}, {
		"--", "\nx", "--\n--x",
	}, {
		"--", "two\nlines\n", "--two\n--lines\n",
	}, {
		"--", "\nempty\nfirst\n", "--\n--empty\n--first\n",
	}, {
		"--", "empty\nlast\n\n", "--empty\n--last\n--\n",
	}, {
		"--", "empty\n\nmiddle\n", "--empty\n--\n--middle\n",
	},
}

func TestIndent(t *testing.T) {
	for x, tt := range tests {
		out := String(tt.prefix, tt.in)
		if out != tt.out {
			t.Errorf("#%d: got %q, want %q", x, out, tt.out)
		}
		bout := string(Bytes([]byte(tt.prefix), []byte(tt.in)))
		if bout != out {
			t.Errorf("#%d: Bytes got %q\n String got %q", x, bout, out)
		}
	}
}

func TestWriter(t *testing.T) {
Test:
	for x, tt := range tests {
		for size := 1; size < 64; size <<= 1 {
			var b bytes.Buffer
			w := NewWriter(&b, tt.prefix)
			data := []byte(tt.in)
			for len(data) > size {
				if _, err := w.Write(data[:size]); err != nil {
					t.Errorf("#%d: %v", x, err)
					continue Test
				}
				data = data[size:]
			}
			if _, err := w.Write(data); err != nil {
				t.Errorf("#%d/%d: %v", x, size, err)
				continue Test
			}

			out := b.String()
			if out != tt.out {
				t.Errorf("#%d/%d: got %q, want %q", x, size, out, tt.out)
			}
		}
	}
}

func TestWrittenSize(t *testing.T) {
	for x, tt := range tests {
		var b bytes.Buffer
		w := NewWriter(&b, tt.prefix)
		data := []byte(tt.in)
		if n, _ := w.Write(data); n != len(data) {
			t.Errorf("#%d: got %d, want %d", x, n, len(data))
		}
	}
}

func TestWrittenSizeWithError(t *testing.T) {
	table := []struct {
		prefix   string
		input    string
		underlay int
		expected int
	}{
		{"--", "two\nlines\n", 0, 0},
		{"--", "two\nlines\n", 1, 0},   // -
		{"--", "two\nlines\n", 2, 0},   // -
		{"--", "two\nlines\n", 3, 1},   // t
		{"--", "two\nlines\n", 4, 2},   // w
		{"--", "two\nlines\n", 5, 3},   // o
		{"--", "two\nlines\n", 6, 4},   // \n
		{"--", "two\nlines\n", 7, 4},   // -
		{"--", "two\nlines\n", 8, 4},   // -
		{"--", "two\nlines\n", 9, 5},   // l
		{"--", "two\nlines\n", 10, 6},  // i
		{"--", "two\nlines\n", 11, 7},  // n
		{"--", "two\nlines\n", 12, 8},  // e
		{"--", "two\nlines\n", 13, 9},  // s
		{"--", "two\nlines\n", 14, 10}, // \n
		{"--", "two\nlines\n", 15, 10}, // -
		{"--", "two\nlines\n", 16, 10}, // -
	}

	for _, d := range table {
		uw := errorWriter{d.underlay}
		w := NewWriter(uw, d.prefix)
		data := []byte(d.input)
		if n, _ := w.Write(data); n != d.expected {
			t.Errorf("underlay: %d, got %d, want %d, err: ", d.underlay, n, d.expected)
		}
	}
}

type errorWriter struct {
	ret int
}

func (w errorWriter) Write(buf []byte) (int, error) {
	return w.ret, errors.New("error")
}
