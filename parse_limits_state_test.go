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
	"io"
	"strings"
	"testing"
)

func TestParseLimitsScannerChecksBeforeStackGrowth(t *testing.T) {
	p := yaml_parser_t{parse_limits: ParseLimits{MaxDepth: 8}, flow_level: 10000,
		simple_keys: make([]yaml_simple_key_t, 10001)}
	if yaml_parser_increase_flow_level(&p) {
		t.Fatal("excessive flow accepted")
	}
	if p.flow_level != 10000 || len(p.simple_keys) != 10001 {
		t.Fatal("flow stack grew before refusing excess")
	}
	p = yaml_parser_t{parse_limits: ParseLimits{MaxDepth: 8}, indent: 0,
		indents: make([]int, 10000), simple_keys: make([]yaml_simple_key_t, 1)}
	if yaml_parser_roll_indent(&p, 1, -1, yaml_BLOCK_MAPPING_START_TOKEN, yaml_mark_t{}) {
		t.Fatal("excessive indent accepted")
	}
	if p.indent != 0 || len(p.indents) != 10000 {
		t.Fatal("indent stack grew before refusing excess")
	}
}

func TestParseLimitsExpiredOptionalKeyReleasesIndex(t *testing.T) {
	for _, index := range []int{1024, 1025} {
		p := yaml_parser_t{parse_limits: ParseLimits{MaxDepth: 8}, mark: yaml_mark_t{index: index},
			simple_keys:        []yaml_simple_key_t{{possible: true, token_number: 4}},
			simple_keys_by_tok: map[int]int{4: 0}}
		valid, ok := yaml_simple_key_is_valid(&p, &p.simple_keys[0])
		if !ok {
			t.Fatal("optional simple key produced scanner error")
		}
		if valid != (index == 1024) {
			t.Fatalf("key at index%d: valid=%v", index, valid)
		}
		_, retained := p.simple_keys_by_tok[4]
		if retained != valid {
			t.Fatalf("expired key's token index remains: %v", retained)
		}
	}
}

type nameBudgetReader struct {
	reader io.Reader
	read   int
}

func (r *nameBudgetReader) Read(b []byte) (int, error) {
	if r.read >= 4096 {
		return 0, errors.New("name witness exceeded input-read budget")
	}
	if len(b) > 4096-r.read {
		b = b[:4096-r.read]
	}
	n, err := r.reader.Read(b)
	r.read += n
	return n, err
}

func TestParseLimitsLongNamesRefuseBeforeScanningPayload(t *testing.T) {
	for _, indicator := range []string{"&", "*"} {
		reader := &nameBudgetReader{reader: strings.NewReader(indicator + strings.Repeat("a", 128*1024) + " value")}
		dec := NewDecoder(reader)
		if err := dec.SetParseLimits(ParseLimits{RejectAnchors: true, RejectAliases: true}); err != nil {
			t.Fatal(err)
		}
		assertLimitRefusal(t, dec)
		if len(dec.parser.anchors) != 0 {
			t.Fatal("rejected name entered anchor state")
		}
	}
}

func TestParseLimitsAllocationAndPendingState(t *testing.T) {
	dec := limitedDecoder(t, "x: ["+strings.Repeat("a,", 1020)+"a]", ParseLimits{MaxNodes: 1024})
	assertLimitRefusal(t, dec)
	if dec.parser.parseNodes != 1024 {
		t.Fatalf("allocations before refusal: %d", dec.parser.parseNodes)
	}
	if dec.parser.parseDocuments != 1 {
		t.Fatalf("documents: %d", dec.parser.parseDocuments)
	}
	if dec.parser.parseDepth != -1 {
		t.Fatal("composition depth did not unwind")
	}
	dec = limitedDecoder(t, "x: "+strings.Repeat("[", 512)+"a"+strings.Repeat("]", 512), ParseLimits{MaxDepth: 8})
	dec.SetCollectionMemberFilter(func(CollectionMember) bool { return false })
	assertLimitRefusal(t, dec)
	// Document, map, key, and seven sequences begin at depths <=8. The
	// pending depth9 collection cannot allocate its node or descendants.
	if dec.parser.parseNodes != 10 {
		t.Fatalf("depth-refused node allocated: %d", dec.parser.parseNodes)
	}
	if len(dec.parser.parser.states) > 9 || len(dec.parser.parser.marks) > 8 || len(dec.parser.collectionPath) > 8 {
		t.Fatalf("pending parser/path state exceeded depth witness: states%d marks%d path%d",
			len(dec.parser.parser.states), len(dec.parser.parser.marks), len(dec.parser.collectionPath))
	}
	if len(dec.parser.parser.tokens)-dec.parser.parser.tokens_head > 14112 || dec.parser.parser.flow_level > 10000 || len(dec.parser.parser.indents) > 10000 {
		t.Fatal("fixed grammar/lookahead state bound exceeded")
	}
	dec = limitedDecoder(t, "a\n---\n"+strings.Repeat("[", 512)+"a"+strings.Repeat("]", 512), ParseLimits{MaxDocuments: 1})
	var out Node
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	assertLimitRefusal(t, dec)
	if dec.parser.parseNodes != 2 || dec.parser.parseDocuments != 1 {
		t.Fatal("second graph began before document refusal")
	}
}

func TestParseLimitsLongOptionalKeysLeaveNoStaleIndex(t *testing.T) {
	dec := limitedDecoder(t, "["+strings.Repeat(strings.Repeat("a", 1025)+",", 99)+strings.Repeat("a", 1025)+"]", ParseLimits{MaxNodes: 1024, MaxDepth: 8})
	var out Node
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Content[0].Content) != 100 {
		t.Fatal("long scalar sequence changed")
	}
	if len(dec.parser.parser.simple_keys_by_tok) > len(dec.parser.parser.simple_keys) {
		t.Fatal("stale simple-key indexes accumulate")
	}
}
