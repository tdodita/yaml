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
	"strings"
	"testing"
)

func limitedDecoder(t *testing.T, input string, limits ParseLimits) *Decoder {
	t.Helper()
	dec := NewDecoder(strings.NewReader(input))
	if err := dec.SetParseLimits(limits); err != nil {
		t.Fatal(err)
	}
	return dec
}

func assertLimitRefusal(t *testing.T, dec *Decoder) {
	t.Helper()
	out := Node{Kind: ScalarNode, Value: "unchanged"}
	err := dec.Decode(&out)
	if !errors.Is(err, ErrParseLimit) {
		t.Fatalf("wanted parse-limit refusal, got %v", err)
	}
	if out.Kind != ScalarNode || out.Value != "unchanged" || len(out.Content) != 0 {
		t.Fatalf("failing document changed caller value: %#v", out)
	}
	if again := dec.Decode(&out); again != err {
		t.Fatalf("limit failure was not terminal: first=%v next=%v", err, again)
	}
}

func TestParseLimitsNodeBoundaryAndFilters(t *testing.T) {
	// x:[N scalars] has N+4 nodes: document, mapping, key, sequence, N.
	for filters := 0; filters < 8; filters++ {
		for _, count := range []int{1020, 1021} {
			t.Run(fmt.Sprintf("filters%d-items%d", filters, count), func(t *testing.T) {
				input := "x: [" + strings.Repeat("a,", count-1) + "a]\n"
				dec := limitedDecoder(t, input, ParseLimits{MaxNodes: 1024})
				if filters&1 != 0 {
					dec.SetRootSequenceItemFilter("x", func(int, *Node) bool { return false })
				}
				if filters&2 != 0 {
					dec.SetCollectionMemberFilter(func(CollectionMember) bool { return false })
				}
				if filters&4 != 0 {
					dec.SetMappingValueFilter(func(MappingValue) bool { return false })
				}
				if count == 1021 {
					assertLimitRefusal(t, dec)
					return
				}
				var out Node
				if err := dec.Decode(&out); err != nil {
					t.Fatal(err)
				}
				if filters == 0 && len(out.Content[0].Content[1].Content) != 1020 {
					t.Fatal("exact-limit input was clipped")
				}
			})
		}
	}
}

func TestParseLimitsDepthBoundary(t *testing.T) {
	for _, root := range []string{"value", "key", "block", "root"} {
		for _, depth := range []int{8, 9} {
			t.Run(fmt.Sprintf("%s-depth%d", root, depth), func(t *testing.T) {
				var input string
				switch root {
				case "value":
					input = "x: " + strings.Repeat("[", depth-2) + "a" + strings.Repeat("]", depth-2)
				case "key":
					input = "? " + strings.Repeat("[", depth-2) + "a" + strings.Repeat("]", depth-2) + "\n: b\n"
				case "block":
					input = strings.Repeat("- ", depth-1) + "a\n"
				case "root":
					input = strings.Repeat("[", depth-1) + "a" + strings.Repeat("]", depth-1)
				}
				dec := limitedDecoder(t, input, ParseLimits{MaxDepth: 8})
				dec.SetCollectionMemberFilter(func(CollectionMember) bool { return false })
				dec.SetMappingValueFilter(func(MappingValue) bool { return false })
				if depth == 9 {
					assertLimitRefusal(t, dec)
					return
				}
				var out Node
				if err := dec.Decode(&out); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	// Flow sequence mapping shorthand creates an extra graph collection.
	assertLimitRefusal(t, limitedDecoder(t, "[a: [b]]", ParseLimits{MaxDepth: 3}))
	var out Node
	if err := limitedDecoder(t, "[a: [b]]", ParseLimits{MaxDepth: 4}).Decode(&out); err != nil {
		t.Fatal(err)
	}
}

func TestParseLimitsEmptyAndNullNodes(t *testing.T) {
	for _, tc := range []struct {
		input string
		nodes int
	}{
		{"---\n", 2}, {"{}", 2}, {"[]", 2}, {"a:\nb: []\n", 6}, {"-\n-\n-\n", 5},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var out Node
			if err := limitedDecoder(t, tc.input, ParseLimits{MaxNodes: tc.nodes}).Decode(&out); err != nil {
				t.Fatal(err)
			}
			assertLimitRefusal(t, limitedDecoder(t, tc.input, ParseLimits{MaxNodes: tc.nodes - 1}))
		})
	}
}

func TestParseLimitsStreamAndAtomicReplacement(t *testing.T) {
	dec := limitedDecoder(t, "[a]\n---\n[b]\n---\nc\n", ParseLimits{MaxNodes: 6})
	for i := 0; i < 2; i++ {
		var out Node
		if err := dec.Decode(&out); err != nil {
			t.Fatal(err)
		}
	}
	assertLimitRefusal(t, dec)
	for _, tail := range []string{"b", "", strings.Repeat("[", 512) + "a" + strings.Repeat("]", 512)} {
		dec := limitedDecoder(t, "a\n---\n"+tail, ParseLimits{MaxDocuments: 1})
		var out Node
		if err := dec.Decode(&out); err != nil {
			t.Fatal(err)
		}
		assertLimitRefusal(t, dec)
	}
	dec = limitedDecoder(t, "a: b", ParseLimits{MaxNodes: 2})
	if err := dec.SetParseLimits(ParseLimits{MaxNodes: 100, MaxDepth: -1}); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	assertLimitRefusal(t, dec)
	dec = limitedDecoder(t, "a\n...\n# end\n", ParseLimits{MaxNodes: 2, MaxDocuments: 1})
	var out Node
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&out); err != io.EOF {
		t.Fatalf("trailing comments: %v", err)
	}
}

func TestParseLimitsAnchorAliasLocations(t *testing.T) {
	for _, input := range []string{"&a value", "&a {key: value}", "&a [value]", "&a key: value", "key: &a value", "x: [&a value]"} {
		t.Run(input, func(t *testing.T) {
			dec := limitedDecoder(t, input, ParseLimits{RejectAnchors: true})
			dec.SetCollectionMemberFilter(func(CollectionMember) bool { return false })
			dec.SetMappingValueFilter(func(MappingValue) bool { return false })
			dec.SetRootSequenceItemFilter("x", func(int, *Node) bool { return false })
			assertLimitRefusal(t, dec)
		})
	}
	for _, input := range []string{"*missing", "[&a value, *a]", "&a key: *a", "x: [&a value, *a]"} {
		t.Run(input, func(t *testing.T) {
			dec := limitedDecoder(t, input, ParseLimits{RejectAliases: true})
			dec.SetCollectionMemberFilter(func(CollectionMember) bool { return false })
			dec.SetMappingValueFilter(func(MappingValue) bool { return false })
			assertLimitRefusal(t, dec)
		})
	}
}
