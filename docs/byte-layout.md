# Byte layout, generated from `schema/ops.yaml`

> Read the declaration, not this file: `go run ./cmd/layout > docs/byte-layout.md`.
> Wire format is big-endian throughout, and there are no tags and no field ids - the order
> of the declaration *is* the format. That is the cost of this codec, and the reason it is
> the smallest: a fixed-size message costs exactly the sum below, known before it is sent.

## Everything (opcode 20)

| field | declared type | on the wire | bytes |
|---|---|---|---|
| `a_u8` | `u8` | u8 | 1 |
| `a_i16` | `i16` | i16 BE | 2 |
| `a_u16` | `u16` | u16 BE | 2 |
| `a_i32` | `i32` | i32 BE | 4 |
| `a_u32` | `u32` | u32 BE | 4 |
| `a_bool` | `bool` | u8 | 1 |
| `a_metres` | `metres` | i16 BE (cm) | 2 |
| `a_deg` | `deg` | u16 BE (1/65535 turn) | 2 |
| `a_str` | `str` | u16 BE length + UTF-8 | 2 + N |
| `a_raw` | `raw` | opaque, to the end | N |

## Move (opcode 1)

| field | declared type | on the wire | bytes |
|---|---|---|---|
| `pos` | `pos` | i16 x,y,z BE (cm) + u16 BE (yaw) | 8 |

## Ready (opcode 6)

| field | declared type | on the wire | bytes |
|---|---|---|---|
| `ready` | `bool` | u8 | 1 |

## Say (opcode 9)

| field | declared type | on the wire | bytes |
|---|---|---|---|
| `text` | `str` | u16 BE length + UTF-8 | 2 + N |

## SetColor (opcode 2)

| field | declared type | on the wire | bytes |
|---|---|---|---|
| `r` | `u8` | u8 | 1 |
| `g` | `u8` | u8 | 1 |
| `b` | `u8` | u8 | 1 |


---

## Where the golden bytes come from

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
