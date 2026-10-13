# The golden bytes, and the mutations this repository is checked against

Hand-written - and deliberately **not** part of `docs/byte-layout.md`: that file is the output of
`go run ./cmd/layout`, and CI diffs it against a fresh run, so anything hand-written in it breaks
that check. That is exactly what happened on the first CI run of this repository, which is a
better introduction to this file than any sentence I could write.

The short program below is a second implementation of the same declaration. It is not shipped, it
is not imported, and it shares no code with the Go codec - which is the only reason it is useful.

```python
import struct, math

def rnd(x):                       # Go's math.Round: half away from zero
    return math.floor(x + 0.5) if x >= 0 else math.ceil(x - 0.5)

def cm(metres):  return struct.pack(">h", int(rnd(metres * 100)))
def deg(d):      return struct.pack(">H", int(rnd(((d % 360) + 360) % 360 / 360 * 65535)))
def string(t):   b = t.encode(); return struct.pack(">H", len(b)) + b

# Move: pos = i16 x, i16 y, i16 z (centimetres) + u16 yaw
cm(1.25) + cm(-3.5) + cm(0) + deg(90)      # -> 007d fea2 0000 4000
```

## Mutations this document has been checked against

`make mutate` runs three plausible mistakes and requires the suite to go red for each:

| mutation | caught by |
|---|---|
| write `i16` little-endian while reading big-endian | `TestGoldenBytes` (the `Everything` message) |
| write `pos` as `z, y, x` | the round trip, and the golden bytes |
| length prefix little-endian | the golden bytes for `Say` and `Everything` |

The first mutation is the reason gate 4 exists: it is symmetric, so a round trip through the same
implementation cannot see it.
