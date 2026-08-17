# TrafficDiff yaml.v3 fork

Copyright 2026 Teodor Dodita. Licensed under Apache-2.0.

This public dependency fork exists only to give TrafficDiff an opt-in,
pre-parent-composition sequence-item retention seam. Ordinary yaml.v3 decoder
behavior remains unchanged when the seam is not installed.

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
- `yaml.go` exports `SequenceItemFilter` and
  `Decoder.SetRootSequenceItemFilter`.
- `decode.go` invokes that filter after each selected root-sequence item is
  composed but before it is attached to the parent tree. Dropped anchor targets
  become lightweight sentinels so known and unknown aliases remain distinct
  without retaining discarded subtrees.
- Focused tests cover default identity, root-only selection, block and flow
  forms, original indexes, aliases, malformed later input, later documents,
  and bounded parent retention. Existing external-package test imports follow
  the fork module identity, and `go.sum` pins the unchanged test dependency.

The scanner, parser grammar, scalar resolver, encoder, emitter, and ordinary
unmarshal behavior are not modified. This fork adds no YAML grammar, validation
framework, product behavior, network access, platform-specific code, or release.

## Ownership and security updates

TrafficDiff maintainers own this delta and must monitor both
`yaml/go-yaml` and the historical `gopkg.in/yaml.v3` identity for security
updates, because scanners may not automatically associate advisories with the
fork module path. To update, replay only this documented delta on a reviewed
upstream security base, pin the new full commit in TrafficDiff, refresh sums and
attribution, and rerun fork compatibility, TrafficDiff differential/resource,
offline, provenance, and supported-platform checks.
