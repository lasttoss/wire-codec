// Package flatbufcodec speaks the declaration in FlatBuffers, without flatc.
//
// Two things make this possible and both are worth stating, because they are what the format is for:
// a write path that slots values by their declaration index, and a read path that walks the vtable
// itself. The wire is then a buffer a client with a generated .fbs (or a hand-written reader) can
// read with no parsing at all - which is the reason to choose it over protobuf for a 30 Hz tick.
//
// One honest difference from the byte codec: a FlatBuffers buffer is self-describing, so there is no
// way to notice bytes appended after it. Trailing-bytes detection exists on the byte wire and on
// protobuf (as unknown fields) and cannot exist here. Saying so beats pretending.
package flatbufcodec

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

type Flatbuf struct{}

func (Flatbuf) Name() string { return "flatbuf" }

// fieldSlot is the vtable entry for field i. Writing uses the same number as the declaration index
// (PrependXSlot), reading addresses the vtable directly as 4+2i - the two are equal for a schema
// written in one pass, and a comment here is cheaper than the bug when they stop being equal.
func slot(i int) flatbuffers.VOffsetT { return flatbuffers.VOffsetT(4 + 2*i) }

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// Encode builds the buffer bottom-up, which is what a FlatBuffers builder requires: nested tables and
// strings first, then the table that points at them.
func (Flatbuf) Encode(m *schema.Message, v map[string]any) ([]byte, error) {
	b := flatbuffers.NewBuilder(0)

	// nested values first, in declaration order, remembering their offsets
	offsets := make([]flatbuffers.UOffsetT, len(m.Fields))
	for i, f := range m.Fields {
		raw, present := v[f.Name]
		if !present {
			continue
		}
		switch f.Type {
		case schema.Pos:
			p, ok := raw.(map[string]any)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not an object with x, y, z and yaw"}
			}
			vals := make([]float64, 0, 4)
			for _, axis := range []string{"x", "y", "z", "yaw"} {
				x, ok := asFloat(p[axis])
				if !ok {
					return nil, codec.ErrValue{Field: f.Name + "." + axis, Reason: "not a number"}
				}
				vals = append(vals, x)
			}
			b.StartObject(4)
			b.PrependFloat64Slot(3, vals[3], 0)
			b.PrependFloat64Slot(2, vals[2], 0)
			b.PrependFloat64Slot(1, vals[1], 0)
			b.PrependFloat64Slot(0, vals[0], 0)
			offsets[i] = b.EndObject()
		case schema.Str:
			s, ok := raw.(string)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a string"}
			}
			offsets[i] = b.CreateString(s)
		case schema.Raw:
			var by []byte
			switch x := raw.(type) {
			case []byte:
				by = x
			case string:
				by = []byte(x)
			default:
				return nil, codec.ErrValue{Field: f.Name, Reason: "not raw bytes"}
			}
			offsets[i] = b.CreateByteVector(by)
		}
	}

	b.StartObject(len(m.Fields))
	// reverse order: a builder writes backwards, so the last field is placed first
	for i := len(m.Fields) - 1; i >= 0; i-- {
		f := m.Fields[i]
		raw, present := v[f.Name]
		if !present {
			continue
		}
		switch f.Type {
		case schema.U8:
			n, ok := asInt(raw)
			if !ok || n < 0 || n > 255 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a u8"}
			}
			b.PrependUint8Slot(i, uint8(n), 0)
		case schema.U16:
			n, ok := asInt(raw)
			if !ok || n < 0 || n > 65535 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a u16"}
			}
			b.PrependUint16Slot(i, uint16(n), 0)
		case schema.U32:
			n, ok := asInt(raw)
			if !ok || n < 0 || n > 4294967295 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a u32"}
			}
			b.PrependUint32Slot(i, uint32(n), 0)
		case schema.I16:
			n, ok := asInt(raw)
			if !ok || n < -32768 || n > 32767 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not an i16"}
			}
			b.PrependInt16Slot(i, int16(n), 0)
		case schema.I32:
			n, ok := asInt(raw)
			if !ok || n < -2147483648 || n > 2147483647 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not an i32"}
			}
			b.PrependInt32Slot(i, int32(n), 0)
		case schema.Bool:
			x, ok := raw.(bool)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a boolean"}
			}
			b.PrependBoolSlot(i, x, false)
		case schema.Metres, schema.Deg:
			x, ok := asFloat(raw)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a number"}
			}
			b.PrependFloat64Slot(i, x, 0)
		case schema.Str, schema.Raw, schema.Pos:
			b.PrependUOffsetTSlot(i, offsets[i], 0)
		default:
			return nil, codec.ErrValue{Field: f.Name, Reason: "no flatbuffers type for " + string(f.Type)}
		}
	}
	root := b.EndObject()
	b.Finish(root)
	return b.FinishedBytes(), nil
}

// table is the reader: a bounds-checked view, so that a truncated buffer is an error and not a panic.
type table struct {
	b   []byte
	pos uint32
}

func (t *table) vtableField(i int) (uint32, error) {
	// soffset at pos points back to the vtable
	if int(t.pos)+4 > len(t.b) {
		return 0, fmt.Errorf("table at %d is past the end of a %d byte buffer", t.pos, len(t.b))
	}
	so := int32(binary.LittleEndian.Uint32(t.b[t.pos:]))
	vt := int64(t.pos) - int64(so)
	if vt < 0 || vt+4 > int64(len(t.b)) {
		return 0, fmt.Errorf("vtable of the table at %d is outside the buffer", t.pos)
	}
	vtsize := int(binary.LittleEndian.Uint16(t.b[vt:]))
	// A vtable smaller than its own header (vtable size + table size) is not a table with every
	// field absent, it is a buffer that does not describe anything: refusing beats defaults.
	if vtsize < 4 {
		return 0, fmt.Errorf("vtable of the table at %d has an impossible size %d", t.pos, vtsize)
	}
	off := 4 + 2*i
	if off+2 > vtsize {
		return 0, nil // the field is not in the vtable: absent
	}
	if vt+int64(off)+2 > int64(len(t.b)) {
		return 0, fmt.Errorf("vtable entry %d is past the end of the buffer", off)
	}
	o := binary.LittleEndian.Uint16(t.b[vt+int64(off):])
	return uint32(o), nil
}

func (t *table) at(off uint32, size int) ([]byte, error) {
	start := int(t.pos) + int(off)
	if start < 0 || start+size > len(t.b) {
		return nil, fmt.Errorf("field at +%d needs %d bytes, buffer is %d", off, size, len(t.b))
	}
	return t.b[start : start+size], nil
}

func (t *table) scalar(i int, size int) ([]byte, bool, error) {
	o, err := t.vtableField(i)
	if err != nil || o == 0 {
		return nil, false, err
	}
	b, err := t.at(o, size)
	return b, true, err
}

// indirect reads a UOffsetT pointing forward from a field, and returns the absolute position it
// points at: this is how strings, vectors and nested tables hang off a table.
func (t *table) indirect(i int) (uint32, bool, error) {
	o, err := t.vtableField(i)
	if err != nil || o == 0 {
		return 0, false, err
	}
	raw, err := t.at(o, 4)
	if err != nil {
		return 0, false, err
	}
	rel := binary.LittleEndian.Uint32(raw)
	abs := int(t.pos) + int(o) + int(rel)
	if abs < 0 || abs > len(t.b) {
		return 0, false, fmt.Errorf("indirect field %d points outside the buffer", i)
	}
	return uint32(abs), true, nil
}

func (Flatbuf) Decode(m *schema.Message, b []byte) (map[string]any, error) {
	if len(b) < 8 {
		return nil, fmt.Errorf("%s: %d bytes is not a FlatBuffers buffer", m.Name, len(b))
	}
	root := binary.LittleEndian.Uint32(b[:4])
	// The first four bytes are the offset of the root table, so a root of zero points at itself.
	if root == 0 || int(root) >= len(b) {
		return nil, fmt.Errorf("%s: root table at %d is past the end of a %d byte buffer", m.Name, root, len(b))
	}
	t := &table{b: b, pos: root}
	out := make(map[string]any, len(m.Fields))
	for i, f := range m.Fields {
		switch f.Type {
		case schema.U8:
			raw, ok, err := t.scalar(i, 1)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = int64(0)
			} else {
				out[f.Name] = int64(raw[0])
			}
		case schema.U16:
			raw, ok, err := t.scalar(i, 2)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = int64(0)
			} else {
				out[f.Name] = int64(binary.LittleEndian.Uint16(raw))
			}
		case schema.U32:
			raw, ok, err := t.scalar(i, 4)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = int64(0)
			} else {
				out[f.Name] = int64(binary.LittleEndian.Uint32(raw))
			}
		case schema.I16:
			raw, ok, err := t.scalar(i, 2)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = int64(0)
			} else {
				out[f.Name] = int64(int16(binary.LittleEndian.Uint16(raw)))
			}
		case schema.I32:
			raw, ok, err := t.scalar(i, 4)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = int64(0)
			} else {
				out[f.Name] = int64(int32(binary.LittleEndian.Uint32(raw)))
			}
		case schema.Bool:
			raw, ok, err := t.scalar(i, 1)
			if err != nil {
				return nil, err
			}
			out[f.Name] = ok && raw[0] != 0
		case schema.Metres, schema.Deg:
			raw, ok, err := t.scalar(i, 8)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = float64(0)
			} else {
				out[f.Name] = float64FromBits(binary.LittleEndian.Uint64(raw))
			}
		case schema.Str:
			abs, ok, err := t.indirect(i)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = ""
				continue
			}
			s, err := readString(b, abs)
			if err != nil {
				return nil, err
			}
			out[f.Name] = s
		case schema.Raw:
			abs, ok, err := t.indirect(i)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = []byte{}
				continue
			}
			by, err := readVector(b, abs)
			if err != nil {
				return nil, err
			}
			out[f.Name] = by
		case schema.Pos:
			abs, ok, err := t.indirect(i)
			if err != nil {
				return nil, err
			}
			if !ok {
				out[f.Name] = map[string]any{"x": 0.0, "y": 0.0, "z": 0.0, "yaw": 0.0}
				continue
			}
			p := map[string]any{}
			sub := &table{b: b, pos: abs}
			for j, axis := range []string{"x", "y", "z", "yaw"} {
				raw, ok, err := sub.scalar(j, 8)
				if err != nil {
					return nil, err
				}
				if !ok {
					p[axis] = 0.0
					continue
				}
				p[axis] = float64FromBits(binary.LittleEndian.Uint64(raw))
			}
			out[f.Name] = p
		default:
			return nil, codec.ErrValue{Field: f.Name, Reason: "no flatbuffers type for " + string(f.Type)}
		}
	}
	return out, nil
}

func readString(b []byte, abs uint32) (string, error) {
	if int(abs)+4 > len(b) {
		return "", fmt.Errorf("string at %d is past the end of the buffer", abs)
	}
	n := binary.LittleEndian.Uint32(b[abs:])
	start := int(abs) + 4
	if start+int(n) > len(b) {
		return "", fmt.Errorf("string of %d bytes at %d is past the end of the buffer", n, abs)
	}
	return string(b[start : start+int(n)]), nil
}

func readVector(b []byte, abs uint32) ([]byte, error) {
	if int(abs)+4 > len(b) {
		return nil, fmt.Errorf("vector at %d is past the end of the buffer", abs)
	}
	n := binary.LittleEndian.Uint32(b[abs:])
	start := int(abs) + 4
	if start+int(n) > len(b) {
		return nil, fmt.Errorf("vector of %d bytes at %d is past the end of the buffer", n, abs)
	}
	return append([]byte(nil), b[start:start+int(n)]...), nil
}

func float64FromBits(u uint64) float64 { return math.Float64frombits(u) }

// Source generates the .fbs text, the artifact a client with a FlatBuffers toolchain compiles.
func Source(m *schema.Message) (string, error) {
	var b strings.Builder
	b.WriteString("// generated from the declaration; flatc compiles this for any language\n")
	b.WriteString("namespace wire;\n\n")
	b.WriteString("table Pos { x:double; y:double; z:double; yaw:double; }\n\n")
	fmt.Fprintf(&b, "table %s {\n", m.Name)
	for _, f := range m.Fields {
		var t string
		switch f.Type {
		case schema.U8:
			t = "ubyte"
		case schema.U16:
			t = "ushort"
		case schema.U32:
			t = "uint"
		case schema.I16:
			t = "short"
		case schema.I32:
			t = "int"
		case schema.Bool:
			t = "bool"
		case schema.Str:
			t = "string"
		case schema.Raw:
			t = "[ubyte]"
		case schema.Metres, schema.Deg:
			t = "double"
		case schema.Pos:
			t = "Pos"
		default:
			return "", fmt.Errorf("%s: no flatbuffers type for %q", f.Name, f.Type)
		}
		fmt.Fprintf(&b, "  %s:%s;\n", f.Name, t)
	}
	b.WriteString("}\n\nroot_type " + m.Name + ";\n")
	return b.String(), nil
}
