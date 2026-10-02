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

import "errors"

// ErrParseLimit identifies a configured parsing-limit refusal. Use errors.Is
// to distinguish it from syntax and value-conversion errors.
var ErrParseLimit = errors.New("yaml: parsing limit exceeded")

// ParseLimits optionally bounds the structures encountered while composing
// YAML documents, including nodes that composition filters discard. Zero
// disables an individual numeric limit. The zero value preserves ordinary
// parsing. Textual payloads still require a separately bounded input reader;
// numeric limits alone do not bound alias expansion when aliases are allowed.
type ParseLimits struct {
	// MaxNodes counts all document, collection, key, value and scalar nodes
	// across the stream, before their allocation.
	MaxNodes int
	// MaxDepth uses document=0, root=1 and one increment per child edge.
	MaxDepth int
	// MaxDocuments includes empty documents and spans Decode calls.
	MaxDocuments  int
	RejectAnchors bool
	RejectAliases bool
}

// SetParseLimits atomically replaces the decoder's parsing limits. Negative
// numeric values are invalid. Configuration cannot change after any Decode
// attempt, including one returning EOF or an error. A limit failure is
// terminal for the decoder; a partially composed document is not decoded.
func (dec *Decoder) SetParseLimits(limits ParseLimits) error {
	if dec.started {
		return errors.New("yaml: parsing limits cannot change after decoding starts")
	}
	if limits.MaxNodes < 0 || limits.MaxDepth < 0 || limits.MaxDocuments < 0 {
		return errors.New("yaml: parsing limits must not be negative")
	}
	dec.parser.parser.parse_limits = limits
	return nil
}

func (p *yaml_parser_t) parsingLimitsEnabled() bool {
	return p.parse_limits != (ParseLimits{})
}

func (p *yaml_parser_t) refuseParseLimit() bool {
	if p.parse_limit_error == nil {
		p.parse_limit_error = ErrParseLimit
	}
	return false
}

func (p *parser) beginNode(kind Kind) {
	if !p.parser.parsingLimitsEnabled() {
		return
	}
	limits := p.parser.parse_limits
	depth := p.parseDepth + 1
	if limits.MaxDepth > 0 && depth > limits.MaxDepth {
		p.parser.refuseParseLimit()
		fail(p.parser.parse_limit_error)
	}
	if kind == DocumentNode {
		if limits.MaxDocuments > 0 && p.parseDocuments >= uint64(limits.MaxDocuments) {
			p.parser.refuseParseLimit()
			fail(p.parser.parse_limit_error)
		}
		p.parseDocuments++
	}
	p.parseDepth = depth
}

func (p *parser) endNode() {
	if p.parser.parsingLimitsEnabled() {
		p.parseDepth--
	}
}

func (p *parser) chargeNode() {
	if !p.parser.parsingLimitsEnabled() {
		return
	}
	max := p.parser.parse_limits.MaxNodes
	if max > 0 && p.parseNodes >= uint64(max) {
		p.parser.refuseParseLimit()
		fail(p.parser.parse_limit_error)
	}
	p.parseNodes++
}
