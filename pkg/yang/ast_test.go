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
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

type MainNode struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	Field      *Value     `yang:"field"`
	Slice      []*Value   `yang:"slice"`
	ChildNode  *SubNode   `yang:"child_node"`
	ChildSlice []*SubNode `yang:"child_slice"`
	ReqNode    *ReqNode   `yang:"req_node"`
}

func (MainNode) Kind() string             { return "main_node" }
func (m *MainNode) ParentNode() Node      { return m.Parent }
func (m *MainNode) NName() string         { return m.Name }
func (m *MainNode) Statement() *Statement { return m.Source }
func (m *MainNode) Exts() []*Statement    { return m.Extensions }

func (m *MainNode) checkEqual(n Node) string {
	o, ok := n.(*MainNode)
	if !ok {
		return fmt.Sprintf("expected *MainNode, got %T", n)
	}
	if m.Name != o.Name {
		return fmt.Sprintf("got name %s, want %s", o.Name, m.Name)
	}
	if s := m.Source.checkEqual(o.Source); s != "" {
		return s
	}
	if (m.Field == nil) != (o.Field == nil) {
		if m.Field == nil {
			return fmt.Sprintf("unexpected field entry")
		}
		return fmt.Sprintf("missing expected field entry")
	}
	if m.Field != nil {
		if m.Field.Name != o.Field.Name {
			return fmt.Sprintf("got field of %s, want %s", o.Field.Name, m.Field.Name)
		}
	}
	if len(m.Slice) != len(o.Slice) {
		return fmt.Sprintf("got slice of %d, want slice of %d", len(o.Slice), len(m.Slice))
	}
	for x, s1 := range m.Slice {
		s2 := o.Slice[x]
		if s1.Name != s2.Name {
			return fmt.Sprintf("slice[%d] got %s, want %s", x, s2.Name, s1.Name)
		}
	}
	if (m.ChildNode == nil) != (o.ChildNode == nil) {
		if m.ChildNode == nil {
			return fmt.Sprintf("unexpected child_node entry")
		}
		return fmt.Sprintf("missing expected child_node entry")
	}
	if m.ChildNode != nil {
		if s := m.ChildNode.checkEqual(o.ChildNode); s != "" {
			return fmt.Sprintf("child_node: %s", s)
		}
	}
	if len(m.ChildSlice) != len(o.ChildSlice) {
		return fmt.Sprintf("got child_slice of %d, want slice of %d", len(o.ChildSlice), len(m.ChildSlice))
	}
	for x, s1 := range m.ChildSlice {
		s2 := o.ChildSlice[x]
		if s := s1.checkEqual(s2); s != "" {
			return fmt.Sprintf("child_slice[%d]: %s", x, s)
		}
	}
	if (m.ReqNode == nil) != (o.ReqNode == nil) {
		if m.ReqNode == nil {
			return fmt.Sprintf("unexpected req_node entry")
		}
		return fmt.Sprintf("missing expected req_node entry")
	}
	if m.ReqNode != nil {
		if s := m.ReqNode.checkEqual(o.ReqNode); s != "" {
			return fmt.Sprintf("req_node: %s", s)
		}
	}
	return ""
}

type SubNode struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	SubField *Value `yang:"sub_field"`
}

func (SubNode) Kind() string             { return "sub_node" }
func (s *SubNode) ParentNode() Node      { return s.Parent }
func (s *SubNode) NName() string         { return s.Name }
func (s *SubNode) Statement() *Statement { return s.Source }
func (s *SubNode) Exts() []*Statement    { return s.Extensions }

func (s *SubNode) checkEqual(o *SubNode) string {
	if s.Name != o.Name {
		return fmt.Sprintf("got name %s, want %s", o.Name, s.Name)
	}
	if s := s.Source.checkEqual(o.Source); s != "" {
		return s
	}
	if (s.SubField == nil) != (o.SubField == nil) {
		if s.SubField == nil {
			return fmt.Sprintf("unexpected sub_field entry")
		}
		return fmt.Sprintf("missing expected sub_field entry")
	}
	if s.SubField != nil {
		if s.SubField.Name != o.SubField.Name {
			return fmt.Sprintf("got sub_field of %s, want %s", o.SubField.Name, s.SubField.Name)
		}
	}
	return ""
}

type ReqNode struct {
	Name       string       `yang:"Name,nomerge"`
	Source     *Statement   `yang:"Statement,nomerge"`
	Parent     Node         `yang:"Parent,nomerge"`
	Extensions []*Statement `yang:"Ext"`

	ReqField *Value `yang:"req_field,required"`
	Field    *Value `yang:"field"`
}

func (ReqNode) Kind() string             { return "req_node" }
func (s *ReqNode) ParentNode() Node      { return s.Parent }
func (s *ReqNode) NName() string         { return s.Name }
func (s *ReqNode) Statement() *Statement { return s.Source }
func (s *ReqNode) Exts() []*Statement    { return s.Extensions }

func (s *ReqNode) checkEqual(o *ReqNode) string {
	if s.Name != o.Name {
		return fmt.Sprintf("got name %s, want %s", o.Name, s.Name)
	}
	if s := s.Source.checkEqual(o.Source); s != "" {
		return s
	}
	if (s.ReqField == nil) != (o.ReqField == nil) {
		if s.ReqField == nil {
			return fmt.Sprintf("unexpected req_field entry")
		}
		return fmt.Sprintf("missing expected req_field entry")
	}
	if s.ReqField != nil {
		if s.ReqField.Name != o.ReqField.Name {
			return fmt.Sprintf("got req_field of %s, want %s", o.ReqField.Name, s.ReqField.Name)
		}
	}
	return ""
}

func (s *Statement) checkEqual(o *Statement) string {
	if (s == nil) != (o == nil) {
		var b bytes.Buffer
		if s == nil {
			o.Write(&b, "")
			return fmt.Sprintf("unexpected Statement entry\n%s", &b)
		}
		s.Write(&b, "")
		return fmt.Sprintf("missing expected Statement entry\n%s", &b)
	}
	if s == nil {
		return ""
	}
	var b1, b2 bytes.Buffer
	s.Write(&b1, "")
	o.Write(&b2, "")
	ss := b1.String()
	os := b2.String()
	if ss != os {
		return fmt.Sprintf("got statement:\n%swant:\n%s", os, ss)
	}
	return ""
}

func TestAST(t *testing.T) {
	// Teach the AST parser about our testing nodes
	type meta struct {
		MainNode []*MainNode `yang:"main_node"`
	}
	initTypes(reflect.TypeOf(&meta{}))

	for _, tt := range []struct {
		line int
		in   string
		out  *MainNode
		err  string
	}{
		{
			line: line(),
			in: `
main_node the_node {
	// This test is testing to make sure unknown statements, that
	// might be extensions, are properly put in the Extensions slice.
	// When an extension is used, it must be of the form "prefix:name".
	// See https://tools.ietf.org/html/rfc6020#section-7.17
	ex:ext1 value1;
	ex:ext2 value2;
}
`,
			out: &MainNode{
				Source: SA("main_node", "the_node",
					SA("ex:ext1", "value1"),
					SA("ex:ext2", "value2")),
				Name: "the_node",
				Extensions: []*Statement{
					SA("ex:ext1", "value1"),
					SA("ex:ext2", "value2"),
				},
			},
		},
		{
			line: line(),
			in: `
main_node the_node {
	// This test tests fields, slices, and sub-statements.
	field field_value;
	slice sl1;
	slice sl2;
	child_node the_child {
		sub_field val1;
	}
	child_slice element1 {
		sub_field el1;
	}
	child_slice element2 {
		sub_field el2;
	}
}`,
			out: &MainNode{
				Source: SA("main_node", "the_node",
					SA("field", "field_value"),
					SA("slice", "sl1"),
					SA("slice", "sl2"),
					SA("child_node", "the_child",
						SA("sub_field", "val1")),
					SA("child_slice", "element1",
						SA("sub_field", "el1")),
					SA("child_slice", "element2",
						SA("sub_field", "el2")),
				),
				Name: "the_node",
				Field: &Value{
					Name: "field_value",
				},
				Slice: []*Value{
					&Value{
						Name: "sl1",
					},
					&Value{
						Name: "sl2",
					},
				},
				ChildNode: &SubNode{
					Source: SA("child_node", "the_child",
						SA("sub_field", "val1")),
					Name: "the_child",
					SubField: &Value{
						Name: "val1",
					},
				},
				ChildSlice: []*SubNode{
					&SubNode{
						Source: SA("child_slice", "element1",
							SA("sub_field", "el1")),
						Name: "element1",
						SubField: &Value{
							Name: "el1",
						},
					},
					&SubNode{
						Source: SA("child_slice", "element2",
							SA("sub_field", "el2")),
						Name: "element2",
						SubField: &Value{
							Name: "el2",
						},
					},
				},
			},
		},
		{
			line: line(),
			in: `
main_node the_node {
	// This test tests that the absence of a required field.
	// req_node requires the field named "req_field".
	req_node value1 ;
}
`,
			err: `ast.yang:5:2: missing required req_node field: req_field`,
		},
	} {
		ss, err := Parse(tt.in, "ast.yang")
		if err != nil {
			t.Errorf("%d: %v", tt.line, err)
			continue
		}
		if len(ss) != 1 {
			t.Errorf("%d: got %d results, want 1", tt.line, len(ss))
			continue
		}
		ast, err := BuildAST(ss[0])
		switch {
		case err == nil && tt.err == "":
			if s := tt.out.checkEqual(ast); s != "" {
				t.Errorf("%d: %s", tt.line, s)
			}
		case err == nil:
			t.Errorf("%d: did not get expected error %s", tt.line, tt.err)
		case tt.err == "":
			t.Errorf("%d: %v", tt.line, err)
		case err.Error() != tt.err:
			t.Errorf("%d: got error %v, want %s", tt.line, err, tt.err)
		}
	}
}
