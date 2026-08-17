// Copyright 2026 Teodor Dodita
// SPDX-License-Identifier: Apache-2.0

package yaml_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tdodita/yaml/v3"
)

func TestRootSequenceItemFilterIsOptInAndRootScoped(t *testing.T) {
	const input = `cases:
  - value: zero
  - value: one
  - value: two
nested:
  cases: [nested-zero, nested-one, nested-two]
---
cases: [second-zero, second-one, second-two]
`

	var ordinary yaml.Node
	if err := yaml.NewDecoder(strings.NewReader(input)).Decode(&ordinary); err != nil {
		t.Fatal(err)
	}

	decoder := yaml.NewDecoder(strings.NewReader(input))
	var indexes []int
	decoder.SetRootSequenceItemFilter("cases", func(index int, item *yaml.Node) bool {
		indexes = append(indexes, index)
		return index != 1
	})
	var filtered yaml.Node
	if err := decoder.Decode(&filtered); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(indexes, []int{0, 1, 2}) {
		t.Fatalf("filter indexes = %v, want original monotonic indexes", indexes)
	}
	root := filtered.Content[0]
	if got := mappingValue(t, root, "cases"); got.Kind != yaml.SequenceNode || len(got.Content) != 2 {
		t.Fatalf("filtered root cases = %#v, want two retained items", got)
	}
	if got := mappingValue(t, mappingValue(t, root, "nested"), "cases"); got.Kind != yaml.SequenceNode || len(got.Content) != 3 {
		t.Fatalf("nested cases = %#v, want all three unfiltered items", got)
	}

	decoder.SetRootSequenceItemFilter("", nil)
	var second yaml.Node
	if err := decoder.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if got := mappingValue(t, second.Content[0], "cases"); got.Kind != yaml.SequenceNode || len(got.Content) != 3 {
		t.Fatalf("second-document cases = %#v, want all three items", got)
	}

	var ordinaryAgain yaml.Node
	if err := yaml.NewDecoder(strings.NewReader(input)).Decode(&ordinaryAgain); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ordinary, ordinaryAgain) {
		t.Fatal("default decoder behavior changed without an installed filter")
	}
}

func TestRootSequenceItemFilterPreservesFlowAndAliasParsing(t *testing.T) {
	const input = `cases: [&base {value: one}, *base, {value: three}]
`
	decoder := yaml.NewDecoder(strings.NewReader(input))
	var kinds []yaml.Kind
	decoder.SetRootSequenceItemFilter("cases", func(index int, item *yaml.Node) bool {
		kinds = append(kinds, item.Kind)
		return false
	})
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(kinds, []yaml.Kind{yaml.MappingNode, yaml.AliasNode, yaml.MappingNode}) {
		t.Fatalf("filtered item kinds = %v, want resolved flow and alias nodes", kinds)
	}
	if got := mappingValue(t, document.Content[0], "cases"); len(got.Content) != 0 {
		t.Fatalf("retained cases = %d, want zero", len(got.Content))
	}
}

func TestRootSequenceItemFilterReleasesDroppedAnchorTargets(t *testing.T) {
	const input = `cases:
  - &first {payload: [one, two, three]}
  - *first
`
	decoder := yaml.NewDecoder(strings.NewReader(input))
	decoder.SetRootSequenceItemFilter("cases", func(index int, item *yaml.Node) bool {
		if index == 1 {
			if item.Kind != yaml.AliasNode || item.Alias == nil {
				t.Fatalf("second item = %#v, want a resolved known alias", item)
			}
			if item.Alias.Kind != 0 || len(item.Alias.Content) != 0 {
				t.Fatalf("dropped anchor target = %#v, want a lightweight known-anchor sentinel", item.Alias)
			}
		}
		return false
	})
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}

	unknown := yaml.NewDecoder(strings.NewReader("cases: [*missing]\n"))
	unknown.SetRootSequenceItemFilter("cases", func(index int, item *yaml.Node) bool { return false })
	if err := unknown.Decode(&document); err == nil {
		t.Fatal("unknown alias Decode() error = nil, want unchanged parser failure")
	}
}

func TestRootSequenceItemFilterDoesNotMaskMalformedDroppedInput(t *testing.T) {
	decoder := yaml.NewDecoder(strings.NewReader("cases: [{value: one}, {value: two}, {\n"))
	decoder.SetRootSequenceItemFilter("cases", func(index int, item *yaml.Node) bool { return false })
	var document yaml.Node
	if err := decoder.Decode(&document); err == nil {
		t.Fatal("Decode() error = nil, want malformed later item failure")
	}
}

func TestRootSequenceItemFilterBoundsRetainedParentItems(t *testing.T) {
	const itemCount = 65_536
	var input strings.Builder
	input.Grow(len("cases:\n") + itemCount*len("  - {}\n"))
	input.WriteString("cases:\n")
	for index := 0; index < itemCount; index++ {
		input.WriteString("  - {}\n")
	}

	decoder := yaml.NewDecoder(strings.NewReader(input.String()))
	seen := 0
	decoder.SetRootSequenceItemFilter("cases", func(index int, item *yaml.Node) bool {
		if index != seen {
			t.Fatalf("filter index = %d, want %d", index, seen)
		}
		seen++
		return index < 9
	})
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if seen != itemCount {
		t.Fatalf("filter saw %d items, want every parsed item %d", seen, itemCount)
	}
	if got := mappingValue(t, document.Content[0], "cases"); len(got.Content) != 9 {
		t.Fatalf("retained cases = %d, want 9", len(got.Content))
	}
}

func mappingValue(t *testing.T, mapping *yaml.Node, key string) *yaml.Node {
	t.Helper()
	if mapping.Kind != yaml.MappingNode || len(mapping.Content)%2 != 0 {
		t.Fatalf("node = %#v, want exact mapping", mapping)
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	t.Fatalf("mapping does not contain key %q", key)
	return nil
}
