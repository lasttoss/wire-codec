// Package bytecodec is the hand-declared binary format: big-endian, fixed sizes, no tags. It
// exists because a game's realtime path pays for every byte, and because a declared layout is
// the only format whose cost is visible in the declaration itself.
package bytecodec

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

type Byte struct{}

func (Byte) Name() string { return "byte" }

func (Byte) Encode(m *schema.Message, v map[string]any) ([]byte, error) {
	canon, err := codec.Canonical(m, v)
	if err != nil {
		return nil, err
	}
	var out []byte
	for _, f := range m.Fields {
		switch f.Type {
		case schema.U8:
			out = append(out, byte(canon[f.Name].(int64)))
		case schema.Bool:
			if canon[f.Name].(bool) {
				out = append(out, 1)
			} else {
				out = append(out, 0)
			}
		case schema.I16:
			out = binary.BigEndian.AppendUint16(out, uint16(int16(canon[f.Name].(int64))))
		case schema.U16:
			out = binary.BigEndian.AppendUint16(out, uint16(canon[f.Name].(int64)))
		case schema.I32:
			out = binary.BigEndian.AppendUint32(out, uint32(int32(canon[f.Name].(int64))))
		case schema.U32:
			out = binary.BigEndian.AppendUint32(out, uint32(canon[f.Name].(int64)))
		case schema.Str:
			s := canon[f.Name].(string)
			out = binary.BigEndian.AppendUint16(out, uint16(len(s)))
			out = append(out, s...)
		case schema.Raw:
			out = append(out, canon[f.Name].([]byte)...)
		case schema.Metres:
			cm, err := codec.WireCenti(canon[f.Name].(float64))
			if err != nil {
				return nil, codec.ErrValue{Field: f.Name, Reason: err.Error()}
			}
			out = binary.BigEndian.AppendUint16(out, uint16(cm))
		case schema.Deg:
			d, err := codec.WireDeg(canon[f.Name].(float64))
			if err != nil {
				return nil, codec.ErrValue{Field: f.Name, Reason: err.Error()}
			}
			out = binary.BigEndian.AppendUint16(out, d)
		case schema.Pos:
			obj := canon[f.Name].(map[string]any)
			for _, axis := range []string{"x", "y", "z"} {
				cm, err := codec.WireCenti(obj[axis].(float64))
				if err != nil {
					return nil, codec.ErrValue{Field: f.Name + "." + axis, Reason: err.Error()}
				}
				out = binary.BigEndian.AppendUint16(out, uint16(cm))
			}
			d, err := codec.WireDeg(obj["yaw"].(float64))
			if err != nil {
				return nil, codec.ErrValue{Field: f.Name + ".yaw", Reason: err.Error()}
			}
			out = binary.BigEndian.AppendUint16(out, d)
		default:
			return nil, fmt.Errorf("byte codec: no wire type for %s", f.Type)
		}
	}
	return out, nil
}

type reader struct {
	b   []byte
	off int
}

func (r *reader) need(field string, n int) error {
	if r.off+n > len(r.b) {
		return codec.ErrShort{Field: field, Need: n, Have: len(r.b) - r.off}
	}
	return nil
}

func (r *reader) u8(f string) (uint8, error) {
	if err := r.need(f, 1); err != nil {
		return 0, err
	}
	v := r.b[r.off]
	r.off++
	return v, nil
}

func (r *reader) u16(f string) (uint16, error) {
	if err := r.need(f, 2); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint16(r.b[r.off:])
	r.off += 2
	return v, nil
}

func (r *reader) u32(f string) (uint32, error) {
	if err := r.need(f, 4); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint32(r.b[r.off:])
	r.off += 4
	return v, nil
}

// Decode returns logical values (metres, degrees) and then checks that nothing is left over.
// The trailing check is the point: a frame with bytes the declaration does not explain means
// the sender and this schema disagree, and a decoder that ignores that tail is how a version
// skew becomes a wrong position months later.
func (Byte) Decode(m *schema.Message, b []byte) (map[string]any, error) {
	r := &reader{b: b}
	out := make(map[string]any, len(m.Fields))
	for _, f := range m.Fields {
		switch f.Type {
		case schema.U8:
			v, err := r.u8(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = int64(v)
		case schema.Bool:
			v, err := r.u8(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = v != 0
		case schema.I16:
			v, err := r.u16(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = int64(int16(v))
		case schema.U16:
			v, err := r.u16(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = int64(v)
		case schema.I32:
			v, err := r.u32(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = int64(int32(v))
		case schema.U32:
			v, err := r.u32(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = int64(v)
		case schema.Str:
			n, err := r.u16(f.Name + ".len")
			if err != nil {
				return nil, err
			}
			if err := r.need(f.Name, int(n)); err != nil {
				return nil, err
			}
			out[f.Name] = string(r.b[r.off : r.off+int(n)])
			r.off += int(n)
		case schema.Raw:
			out[f.Name] = append([]byte(nil), r.b[r.off:]...)
			r.off = len(r.b)
		case schema.Metres:
			v, err := r.u16(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = codec.CentiMetres(int16(v))
		case schema.Deg:
			v, err := r.u16(f.Name)
			if err != nil {
				return nil, err
			}
			out[f.Name] = codec.Degrees(v)
		case schema.Pos:
			obj := map[string]any{}
			for _, axis := range []string{"x", "y", "z"} {
				v, err := r.u16(f.Name + "." + axis)
				if err != nil {
					return nil, err
				}
				obj[axis] = codec.CentiMetres(int16(v))
			}
			v, err := r.u16(f.Name + ".yaw")
			if err != nil {
				return nil, err
			}
			obj["yaw"] = codec.Degrees(v)
			out[f.Name] = obj
		default:
			return nil, fmt.Errorf("byte codec: no wire type for %s", f.Type)
		}
	}
	if left := len(b) - r.off; left > 0 {
		return nil, codec.ErrTrailing{Bytes: left}
	}
	return out, nil
}

// Size returns the exact byte cost of a message, which is the number that makes this format
// worth the trouble: it is known before the first packet is sent.
func Size(m *schema.Message) (int, error) {
	n := 0
	for i, f := range m.Fields {
		switch f.Type {
		case schema.U8, schema.Bool:
			n++
		case schema.I16, schema.U16, schema.Deg, schema.Metres:
			n += 2
		case schema.I32, schema.U32:
			n += 4
		case schema.Pos:
			n += 8
		case schema.Str:
			return 0, fmt.Errorf("%s is variable length; only the fixed part is known (%d bytes so far)", f.Name, n)
		case schema.Raw:
			return 0, fmt.Errorf("%s is open-ended; only the fixed part is known (%d bytes so far)", f.Name, n)
		}
		_ = i
	}
	return n, nil
}

var _ = math.Round
