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
	"io"
	"strings"
	"testing"
)

func TestParseLimitsConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name           string
		valid, invalid ParseLimits
	}{
		{"nodes", ParseLimits{MaxNodes: 8}, ParseLimits{MaxNodes: -1}},
		{"depth", ParseLimits{MaxDepth: 8}, ParseLimits{MaxDepth: -1}},
		{"documents", ParseLimits{MaxDocuments: 8}, ParseLimits{MaxDocuments: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := NewDecoder(strings.NewReader("a"))
			if err := dec.SetParseLimits(tc.valid); err != nil {
				t.Fatal(err)
			}
			if err := dec.SetParseLimits(tc.invalid); err == nil {
				t.Fatal("negative limit was accepted")
			}
			if err := dec.SetParseLimits(ParseLimits{}); err != nil {
				t.Fatalf("zero configuration: %v", err)
			}
		})
	}
	for _, input := range []string{"a", "", "["} {
		t.Run("started-"+input, func(t *testing.T) {
			dec := NewDecoder(strings.NewReader(input))
			var out Node
			_ = dec.Decode(&out)
			if err := dec.SetParseLimits(ParseLimits{}); err == nil {
				t.Fatal("limits changed after Decode attempt")
			}
		})
	}
}

func TestParseLimitsCopiedDecoderStartLock(t *testing.T) {
	for _, tc := range []struct {
		name, input string
	}{
		{"success", "a\n---\nb\n"},
		{"eof", ""},
		{"syntax", "["},
	} {
		for _, decodeCopy := range []bool{false, true} {
			name := tc.name + "-original-decodes"
			if decodeCopy {
				name = tc.name + "-copy-decodes"
			}
			t.Run(name, func(t *testing.T) {
				original := limitedDecoder(t, tc.input, ParseLimits{MaxDocuments: 1})
				copied := *original
				active, other := original, &copied
				if decodeCopy {
					active, other = other, active
				}
				var out Node
				err := active.Decode(&out)
				switch tc.name {
				case "success":
					if err != nil {
						t.Fatal(err)
					}
				case "eof":
					if err != io.EOF {
						t.Fatalf("wanted EOF, got %v", err)
					}
				case "syntax":
					if err == nil || err == io.EOF || err == ErrParseLimit {
						t.Fatalf("wanted syntax error, got %v", err)
					}
				}
				if err := other.SetParseLimits(ParseLimits{}); err == nil {
					t.Error("copy reconfigured shared parser after Decode attempt")
				}
				if tc.name == "success" {
					assertLimitRefusal(t, active)
				}
			})
		}
	}
}
