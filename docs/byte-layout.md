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
