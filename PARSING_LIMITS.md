# Opt-in parsing limits

`Decoder.SetParseLimits` configures limits before the first `Decode` attempt.
Ordinary `Decoder` and `Unmarshal` behavior remains unchanged when limits are
not configured. The grammar, scalar resolution, `KnownFields`, and composition
filters retain their existing meanings.

```go
dec := yaml.NewDecoder(reader)
err := dec.SetParseLimits(yaml.ParseLimits{
    MaxNodes: 1024,
    MaxDepth: 8,
    MaxDocuments: 1,
    RejectAnchors: true,
    RejectAliases: true,
})
```

Handle the configuration error before decoding. Bound the input reader
separately; these settings are not a byte limit or a universal memory/CPU
ceiling for an unbounded reader. Use `errors.Is(err, yaml.ErrParseLimit)` to
identify a configured parsing-limit refusal. Syntax and value-conversion
errors remain distinct.

## Accounting and lifecycle

Every document, collection, key, value, scalar, empty/null scalar and alias
node counts before allocation. Counts span the entire decoder stream and are
not refunded when a composition filter discards a node. Complex keys and
discarded descendants are included. Node order tokens and retained tree sizes
are separate from this encountered-node count.

Depth uses document=0, root=1, and one increment for each child edge. The
prospective excessive node is refused before allocation or composition of its
descendants. Empty documents count. A second document is refused before its
graph is composed, although directive metadata and scanner lookahead may
already have been read. Anchor and alias rejection happens on actual syntax
indicators, before scanning their names or retaining their associations;
structural-looking text inside scalars or comments remains text.

Zero disables only its numeric dimension. Settings replacement is atomic:
negative numeric limits fail without replacing the previous configuration.
Any first `Decode` attempt locks settings, including one returning EOF, a
syntax error or a type error. A parse-limit error remains terminal: subsequent
`Decode` calls return the same error, never a clipped result or misleading EOF.
The partially composed failing document is not unmarshaled into the caller's
value.

This is a streaming decoder. A valid first document can be returned before a
later call discovers a disallowed second document or malformed tail. A caller
requiring complete-stream admission must stage its value and require EOF before
using it. Generic streaming rollback and rollback on type/`KnownFields` errors
are not provided. Numeric composition limits alone do not bound alias expansion
into Go values when aliases remain allowed.

## Auxiliary state and payload assumptions

The compositor charges every allocation at its common node builder. Its node
methods bracket depth, including direct root mappings and root-filtered value
sequences. With a positive depth bound D, at most D+1 node frames (including
the document) begin. The event parser may retain a prospective child event
and one return state before that child's composition is refused. Parser states
are at most P+1 and marks at most P, where P is the smaller enabled node/depth
bound. A filtered path can include one pending child edge, reaching D entries.
No prospective excessive graph node is allocated.

The scanner keeps its existing fixed grammar ceilings of 10,000 flow levels
and 10,000 indentation entries. Active configurations check those ceilings
before growing the stacks. Flow level is not graph depth: flow shorthand maps
and indentless sequences demonstrate why these are separate counts. The
simple-key stack is at most 10,001 entries; expired optional key indexes are
removed under active limits. Lookahead retains the existing single-line,
1,024-Unicode-character simple-key window, with a possible final payload
overshoot before the window is tested again.

Token headers are finite under those grammar/lookahead ceilings. Independently,
over B source bytes there are at most 4B+4 token insertions: one primary token,
at most one KEY and BLOCK-START per consuming fetch, paired BLOCK-ENDs, and
stream sentinels. Consumed token prefixes are compacted. This counts logical
headers, not exact slice capacity or allocated memory.

Scalar, comment, tag and directive payloads are a different dimension. Comment
headers and directive tables can grow with B even before a tiny first graph;
they are not node-only bounded. Reader scratch buffers are fixed at 512 raw
and 1,536 decoded bytes, and comment position peeks at 512 positions, while
payload scanning may consume the complete text. Expanded custom tags can copy a
long directive prefix into each tagged node, so their logical payload can be
O(N*B) with node limit N. Allowed anchors can likewise retain source-byte names
and at most N associations; filter bookkeeping can carry O(N*P) references.
These facts require a separate finite input-byte budget and are not an RSS,
elapsed-time or alias-expansion guarantee.

The tests use independently counted exact/plus-one nodes and depths, internal
allocation/state witnesses, all filter combinations, real syntax/name scanning,
stream lifecycle, and default/under-limit grammar and callback parity. They do
not use timing thresholds as resource proof.
