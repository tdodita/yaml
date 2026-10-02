//
// Copyright (c) 2011-2019 Canonical Ltd
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

package yaml

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseLimitsGrammarAndCallbackParity(t *testing.T) {
	inputs := []string{
		"x:\n- a\n- b\nother: {key: value}\n",
		"{x: [a, b], other: {key: value}}",
		`{"x": ["a", null], "other": {"key": 1}}`,
		"'x': \"quoted\\nvalue\"\n",
		"x: multiline\n  plain scalar\n",
		"x: a & b * c [d] {e} %tag --- f\n",
		"x: '&a *a [ { % ---'\n",
		"x: |\n  &a *a [ { % ---\n  literal\n",
		"x: >\n  folded\n  text\n",
		"\ufeffx: value\r\n# tail\r\n",
		"x: !!str 12\n# end\n",
		"%TAG !e! tag:example.test,2026:\n---\nx: !e!kind value\n...\n# tail\n",
		"[a: [b]]", "? [a, b]\n: [c]\n",
	}
	for index, input := range inputs {
		for filters := 0; filters < 8; filters++ {
			t.Run(fmt.Sprintf("input%d-filters%d", index, filters), func(t *testing.T) {
				decode := func(limited bool) (Node, []string) {
					dec := NewDecoder(strings.NewReader(input))
					if limited {
						if err := dec.SetParseLimits(ParseLimits{MaxNodes: 1024, MaxDepth: 8, MaxDocuments: 1, RejectAnchors: true, RejectAliases: true}); err != nil {
							t.Fatal(err)
						}
					}
					var trace []string
					if filters&1 != 0 {
						dec.SetRootSequenceItemFilter("x", func(i int, n *Node) bool {
							trace = append(trace, fmt.Sprintf("root%d/%d/%s", i, n.Kind, n.Value))
							return i%2 == 0
						})
					}
					if filters&2 != 0 {
						dec.SetCollectionMemberFilter(func(m CollectionMember) bool {
							trace = append(trace, fmt.Sprintf("member%v/%d/%d/%d/%d/%d/%t", m.Path, m.ParentKind, m.ParentID, m.Index, m.KeyOrder, m.ValueOrder, m.Last))
							return m.Index%2 == 0
						})
					}
					if filters&4 != 0 {
						dec.SetMappingValueFilter(func(m MappingValue) bool {
							trace = append(trace, fmt.Sprintf("value%v/%d/%d/%d/%s", m.Path, m.ParentID, m.Index, m.KeyOrder, m.Key.Value))
							return m.Index%2 == 0
						})
					}
					var out Node
					if err := dec.Decode(&out); err != nil {
						t.Fatal(err)
					}
					if err := dec.Decode(&Node{}); err != io.EOF {
						t.Fatalf("stream tail: %v", err)
					}
					return out, trace
				}
				plain, plainTrace := decode(false)
				limited, limitedTrace := decode(true)
				if !reflect.DeepEqual(plain, limited) || !reflect.DeepEqual(plainTrace, limitedTrace) {
					t.Fatal("limits changed in-limit nodes or callback semantics")
				}
			})
		}
	}
}

func TestParseLimitsKnownFieldsAndMalformedTail(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		dec := NewDecoder(strings.NewReader("x: value\nunknown: hidden\n"))
		if enabled {
			if err := dec.SetParseLimits(ParseLimits{MaxNodes: 10, MaxDepth: 8}); err != nil {
				t.Fatal(err)
			}
		}
		dec.KnownFields(true)
		var out struct {
			X string `yaml:"x"`
		}
		err := dec.Decode(&out)
		var typeErr *TypeError
		if !errors.As(err, &typeErr) || out.X != "value" || !strings.Contains(err.Error(), "field unknown not found") {
			t.Fatalf("KnownFields behavior: value=%q err=%v", out.X, err)
		}
		if err := dec.SetParseLimits(ParseLimits{}); err == nil {
			t.Fatal("reconfigured after type error")
		}
	}
	dec := limitedDecoder(t, "a\n...\nmalformed tail", ParseLimits{MaxDocuments: 1})
	var out Node
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&out); err == nil || err == io.EOF || errors.Is(err, ErrParseLimit) {
		t.Fatalf("malformed tail lost syntax refusal: %v", err)
	}
}

func TestParseLimitsTextAndDirectivePayloadParity(t *testing.T) {
	var directives strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&directives, "%%TAG !e%d! tag:example.test,2026:prefix%d-\n", i, i)
	}
	inputs := []string{
		strings.Repeat("s", 128*1024),
		"#" + strings.Repeat("c", 128*1024) + "\nvalue\n",
		directives.String() + "---\n!e0!kind value\n",
		"%TAG !e! tag:example.test,2026:" + strings.Repeat("p", 128*1024) + "\n---\n!e!kind value\n",
	}
	for i, input := range inputs {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var plain, limited Node
			if err := NewDecoder(strings.NewReader(input)).Decode(&plain); err != nil {
				t.Fatal(err)
			}
			if err := limitedDecoder(t, input, ParseLimits{MaxNodes: 2, MaxDepth: 1, MaxDocuments: 1, RejectAnchors: true, RejectAliases: true}).Decode(&limited); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plain, limited) {
				t.Fatal("payload metadata changed")
			}
		})
	}
}

func TestParseLimitsDirectRootAndComplexKeyCounts(t *testing.T) {
	for _, tc := range []struct {
		input string
		nodes int
	}{
		{"? [a, b]\n: [c]\n", 7}, {"[a, b, c]", 5}, {"{a: b, c: d}", 6}, {"?\n: value\n", 4},
	} {
		var out Node
		if err := limitedDecoder(t, tc.input, ParseLimits{MaxNodes: tc.nodes}).Decode(&out); err != nil {
			t.Fatal(err)
		}
		assertLimitRefusal(t, limitedDecoder(t, tc.input, ParseLimits{MaxNodes: tc.nodes - 1}))
	}
}

func TestParseLimitsZeroDimensionsAndAllowedAliases(t *testing.T) {
	dec := limitedDecoder(t, "[&a value, *a]", ParseLimits{MaxNodes: 5, MaxDepth: 2})
	var out Node
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Content[0].Content[1].Alias != out.Content[0].Content[0] {
		t.Fatal("allowed alias resolution changed")
	}
	dec = limitedDecoder(t, "[a, b, c, d]", ParseLimits{MaxNodes: 1})
	if err := dec.SetParseLimits(ParseLimits{MaxDepth: 2}); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	dec = limitedDecoder(t, "a\n---\nb\n", ParseLimits{MaxNodes: 4, MaxDocuments: 2})
	for i := 0; i < 2; i++ {
		if err := dec.Decode(&out); err != nil {
			t.Fatal(err)
		}
	}
	if err := dec.Decode(&out); err != io.EOF {
		t.Fatal(err)
	}
}
