# TrafficDiff yaml.v3 fork

Copyright 2026 Teodor Dodita. Licensed under Apache-2.0.

This public dependency fork provides opt-in collection-member retention seams
and pre-composition parsing limits. Ordinary yaml.v3 decoder behavior remains
unchanged when no seam is installed.

## Source identity

- Maintained upstream: `https://github.com/yaml/go-yaml`
- Historical module: `gopkg.in/yaml.v3`
- Upstream release: `v3.0.1`
- Upstream commit: `f6f7691b1fdeb513f56608cd2c32c51f8194bf51`
- Upstream tree: `1cb2e60a039c6b3cfdfbd33cc2d049bf68c7db00`
- Fork module: `github.com/tdodita/yaml/v3`

The upstream `LICENSE` and `NOTICE` files are preserved byte-for-byte.

## Narrow delta

- `go.mod` gives the fork its direct module identity.
- `parse_limits.go` adds optional stream-cumulative node/depth/document limits
  and anchor/alias syntax refusal, configured before decoding starts.
  `ErrParseLimit` identifies sticky limit failures. See
  [PARSING_LIMITS.md](PARSING_LIMITS.md) for semantics and state/payload limits.
- `yaml.go` exports the original exact-root `SequenceItemFilter` plus one
  recursive `CollectionMemberFilter` carrying ephemeral logical paths,
  original indexes, parent identities, preorder tokens, and collection-end
  facts, and one pre-value `MappingValueFilter` for keys that have already
  established a closed-shape or duplicate counterexample.
- `decode.go` invokes registered filters after each selected exact root-sequence
  item or recursive collection member is composed but before it is attached to
  the parent tree. A rejected mapping key suppresses descendant retention before
  its value is composed, while parsing and recursive callbacks continue. Nested
  filters run even below an ancestor that is later discarded. Dropped anchor
  targets become lightweight sentinels so known and unknown aliases remain
  distinct without retaining discarded subtrees.
- Focused tests cover default identity, root-only selection, block and flow
  forms, recursive paths, original indexes, preorder and collection-end facts,
  aliases, pre-value suppression, malformed later input, later documents, and
  bounded parent retention. Existing external-package test imports follow the
  fork module identity, and `go.sum` pins the unchanged test dependency.

The scanner refuses configured anchor/alias syntax before name retention and
checks its existing fixed stack ceilings before growth when limits are active.
The compositor charges encountered nodes, including discarded descendants,
before allocation. Parser grammar, scalar resolver, encoder, emitter and
ordinary unmarshal behavior remain unchanged. The fork remains schema-neutral;
callers own schema validation, error precedence and retention decisions. This
fork adds no YAML grammar, product behavior, network access, platform-specific
code, or release.

## Ownership and security updates

TrafficDiff maintainers own this delta and must monitor both
`yaml/go-yaml` and the historical `gopkg.in/yaml.v3` identity for security
updates, because scanners may not automatically associate advisories with the
fork module path. To update, replay only this documented delta on a reviewed
upstream security base, pin the new full commit in TrafficDiff, refresh sums and
attribution, and rerun fork compatibility, TrafficDiff differential/resource,
offline, provenance, and supported-platform checks.
