# wire-codec

One declaration, three wire formats, and the tests that make them interchangeable.

A game server's wire format is usually a decision you cannot revisit: the client shipped, the
protocol is in the field, and changing it means a coordinated release. This is the smaller
version of that problem, solved the way I would want it at work: **declare the messages once**,
generate the formats from that declaration, and check that they agree.

```
schema/ops.yaml          the only thing you write by hand
        │
        ├── codec/bytecodec    big-endian layout, no tags, cost known before it is sent
        ├── codec/protocodec   protobuf, compiled from generated .proto at runtime
        └── codec/fbufcodec    FlatBuffers, read without generated code
                    │
        codec.Canonical     the resolution all three share (1 cm, 1/65535 of a turn)
                    │
        JSON in, wire out, wire in, JSON out
```

## Status

| part | state |
|---|---|
| the declaration (`schema/`, YAML, validated) | **done** |
| `byte` codec, from the declaration | **done** |
| `json` codec (the reference) | **done** |
| the gates: cross-format equivalence, faults, sizes, golden bytes | **done, and mutation-checked** |
| `proto` codec | next |
| `flatbuf` codec | next |
| generating `.proto` and `.fbs` for other languages | next |

This table is deliberately not a roadmap: the parts that say "done" are the parts whose tests I
have seen fail for the right reason (`make mutate`).

## Why three, and why one interface

The requirement was not "support protobuf"; it was **"let a project pick one of three, and
inherit the choice"**. So the unit of work is an interface:

```go
type Codec interface {
    Name() string
    Encode(m *schema.Message, v map[string]any) ([]byte, error)
    Decode(m *schema.Message, b []byte) (map[string]any, error)
}
```

Game logic holds a `Codec` and never a `[]byte`. Swapping formats is a configuration change and
`Name()` is what goes in the configuration (`"byte"`, `"proto"`, `"flatbuf"`).

The three formats are not interchangeable in cost, and this repository does not pretend they
are: the byte format is the cheapest and the least flexible (no field ids means no schema
evolution), protobuf pays tags for evolution, FlatBuffers pays a vtable for zero-copy reads. The
point of one declaration is that you can *measure* that trade instead of arguing about it.

## The gates

1. **Every codec agrees on the value.** JSON in, wire out, wire back in, JSON out - and the value
   must survive. This is what lets a project change format without changing its game.
2. **Broken frames are refused.** A truncated frame, an empty frame, bytes the declaration does not
   explain, a string shorter than its own length prefix. A decoder that returns a plausible zero
   for any of these is the failure this repository exists to prevent.
3. **Values that do not fit are refused, not clamped.** 400 metres is not 327.67 metres.
4. **Golden bytes from a second implementation.** The bytes in `codec/golden_test.go` were produced
   by the short Python in `docs/byte-layout.md`, not by this code. A round trip through one
   implementation proves only that implementation agrees with itself - which is precisely what a
   byte-order bug does. This test is the one that disagrees. It has already caught a real bug:
   `a_raw` arriving as base64 and being encoded as the eight characters of its own spelling.
5. **Every declared message has a case.** Adding a message and forgetting to test it fails a test
   instead of leaving a wire type nobody has ever encoded.

```bash
make test          # all of the above
make mutate        # break the codec on purpose, and require the gate to notice
```

## Layout

```bash
go run ./cmd/layout > docs/byte-layout.md   # the document is generated, not maintained
```
