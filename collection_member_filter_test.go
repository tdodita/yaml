// Copyright 2026 Teodor Dodita
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"reflect"
	"strings"
	"testing"
)

func TestCollectionMemberFilterIsRecursiveAndPreservesKnownAliases(t *testing.T) {
	input := "root:\n  keep: [one, two]\n  drop:\n    nested: [&inside value, other]\nafter: *inside\n"
	decoder := NewDecoder(strings.NewReader(input))
	var paths [][]PathElement
	decoder.SetCollectionMemberFilter(func(member CollectionMember) bool {
		path := append([]PathElement(nil), member.Path...)
		paths = append(paths, path)
		if len(path) >= 2 && path[0].Kind == MappingValuePath && path[0].Key == "root" &&
			path[1].Kind == MappingValuePath && path[1].Key == "drop" {
			return false
		}
		return true
	})

	var document Node
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(paths) != 9 {
		t.Fatalf("filter callback count = %d, want 9", len(paths))
	}
	wantNestedItemPath := []PathElement{
		{Kind: MappingValuePath, Key: "root", Exact: true},
		{Kind: MappingValuePath, Key: "drop", Index: 1, Exact: true},
		{Kind: MappingValuePath, Key: "nested", Exact: true},
		{Kind: SequenceItemPath, Index: 0},
	}
	if !containsPath(paths, wantNestedItemPath) {
		t.Fatalf("filter paths = %#v, want nested item path %#v", paths, wantNestedItemPath)
	}

	root := document.Content[0]
	rootValue := root.Content[1]
	if rootValue.Kind != MappingNode || len(rootValue.Content) != 2 || rootValue.Content[0].Value != "keep" {
		t.Fatalf("filtered root value = %#v, want only retained keep member", rootValue)
	}
	after := root.Content[3]
	if after.Kind != AliasNode || after.Alias == nil || len(after.Alias.Content) != 0 {
		t.Fatalf("alias after dropped subtree = %#v, want known lightweight sentinel", after)
	}
}

func TestCollectionMemberFilterReportsStableOrderAndCollectionEnd(t *testing.T) {
	decoder := NewDecoder(strings.NewReader("outer: {first: [a, b], second: c}\n"))
	var members []CollectionMember
	decoder.SetCollectionMemberFilter(func(member CollectionMember) bool {
		member.Path = append([]PathElement(nil), member.Path...)
		members = append(members, member)
		return true
	})
	var document Node
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(members) != 5 {
		t.Fatalf("members = %d, want 5", len(members))
	}
	for index, member := range members {
		if member.ValueOrder == 0 || member.Key != nil && member.KeyOrder == 0 {
			t.Fatalf("member[%d] orders = key %d value %d, want nonzero", index, member.KeyOrder, member.ValueOrder)
		}
		if member.Key != nil && member.KeyOrder >= member.ValueOrder {
			t.Fatalf("member[%d] orders = key %d value %d, want key before value", index, member.KeyOrder, member.ValueOrder)
		}
	}
	if members[0].Last || !members[1].Last || members[2].Last || !members[3].Last || !members[4].Last {
		t.Fatalf("collection end flags = %#v, want final member of every nonempty collection marked", members)
	}
}

func TestCollectionMemberFilterNilPreservesOrdinaryDecode(t *testing.T) {
	input := "root: {flow: [one, two]}\n---\nnext: value\n"
	ordinary := NewDecoder(strings.NewReader(input))
	filtered := NewDecoder(strings.NewReader(input))
	filtered.SetCollectionMemberFilter(nil)
	for document := 0; document < 2; document++ {
		var want, got Node
		if err := ordinary.Decode(&want); err != nil {
			t.Fatalf("ordinary Decode(%d) error = %v", document, err)
		}
		if err := filtered.Decode(&got); err != nil {
			t.Fatalf("filtered Decode(%d) error = %v", document, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("filtered Decode(%d) = %#v, want ordinary %#v", document, got, want)
		}
	}
}

func TestCollectionMemberFilterPreservesDedentedFootComments(t *testing.T) {
	input := "a:\n  b: c\n# foot\n\nd: e\n"
	ordinary := NewDecoder(strings.NewReader(input))
	filtered := NewDecoder(strings.NewReader(input))
	filtered.SetCollectionMemberFilter(func(CollectionMember) bool { return true })

	var want, got Node
	if err := ordinary.Decode(&want); err != nil {
		t.Fatalf("ordinary Decode() error = %v", err)
	}
	if err := filtered.Decode(&got); err != nil {
		t.Fatalf("filtered Decode() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered Decode() = %#v, want ordinary %#v", got, want)
	}
}

func TestCollectionMemberFilterDoesNotMaskMalformedDroppedMember(t *testing.T) {
	decoder := NewDecoder(strings.NewReader("root: [{}, {]\n"))
	decoder.SetCollectionMemberFilter(func(CollectionMember) bool { return false })
	var document Node
	if err := decoder.Decode(&document); err == nil {
		t.Fatal("Decode() error = nil, want malformed YAML failure")
	}
}

func containsPath(paths [][]PathElement, want []PathElement) bool {
	for _, path := range paths {
		if reflect.DeepEqual(path, want) {
			return true
		}
	}
	return false
}
