package codec

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/lasttoss/wire-codec/internal/schema"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// The canonical form is the byte wire's resolution. It is the smallest of the three, so a value
// that survives the round trip here survives the other two, and the equivalence test has one
// place to compare instead of three.

func WireCenti(metres float64) (int16, error) {
	cm := math.Round(metres * 100)
	if cm < -32768 || cm > 32767 {
		return 0, fmt.Errorf("%.4f metres is %.0f cm, outside the int16 centimetre wire type", metres, cm)
	}
	return int16(cm), nil
}

func CentiMetres(cm int16) float64 { return float64(cm) / 100 }

func WireDeg(deg float64) (uint16, error) {
	d := math.Mod(deg, 360)
	if d < 0 {
		d += 360
	}
	return uint16(math.Round(d / 360 * 65535)), nil
}

func Degrees(d uint16) float64 { return float64(d) / 65535 * 360 }

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	f, ok := asFloat(v)
	if !ok || f != math.Trunc(f) {
		return 0, false
	}
	return int64(f), true
}

// Canonical quantises every field to the resolution all three codecs share, and refuses a value
// that does not fit the wire type instead of clamping it. Clamping is how a wrong position
// becomes a plausible one.
func Canonical(m *schema.Message, v map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(m.Fields))
	for _, f := range m.Fields {
		raw, present := v[f.Name]
		if !present {
			return nil, ErrValue{Field: f.Name, Reason: "missing"}
		}
		switch f.Type {
		case schema.Bool:
			b, ok := raw.(bool)
			if !ok {
				return nil, ErrValue{Field: f.Name, Reason: "not a boolean"}
			}
			out[f.Name] = b
		case schema.Str:
			s, ok := raw.(string)
			if !ok {
				return nil, ErrValue{Field: f.Name, Reason: "not a string"}
			}
			if len(s) > 65535 {
				return nil, ErrValue{Field: f.Name, Reason: fmt.Sprintf("%d bytes, longer than the u16 length prefix", len(s))}
			}
			out[f.Name] = s
		case schema.Raw:
			// JSON carries raw bytes the same way encoding/json does: base64. Accepting the
			// literal string instead is how "3q2+7w==" becomes the eight bytes of its own
			// spelling - the golden test caught exactly that, from the other implementation.
			switch b := raw.(type) {
			case []byte:
				out[f.Name] = b
			case string:
				decoded, err := base64.StdEncoding.DecodeString(b)
				if err != nil {
					return nil, ErrValue{Field: f.Name, Reason: "not base64: " + err.Error()}
				}
				out[f.Name] = decoded
			default:
				return nil, ErrValue{Field: f.Name, Reason: "not raw bytes or base64"}
			}
		case schema.Metres:
			fv, ok := asFloat(raw)
			if !ok {
				return nil, ErrValue{Field: f.Name, Reason: "not a number"}
			}
			cm, err := WireCenti(fv)
			if err != nil {
				return nil, ErrValue{Field: f.Name, Reason: err.Error()}
			}
			out[f.Name] = CentiMetres(cm)
		case schema.Deg:
			fv, ok := asFloat(raw)
			if !ok {
				return nil, ErrValue{Field: f.Name, Reason: "not a number"}
			}
			d, err := WireDeg(fv)
			if err != nil {
				return nil, ErrValue{Field: f.Name, Reason: err.Error()}
			}
			out[f.Name] = Degrees(d)
		case schema.Pos:
			obj, ok := raw.(map[string]any)
			if !ok {
				return nil, ErrValue{Field: f.Name, Reason: "not an object with x, y, z and yaw"}
			}
			canon := map[string]any{}
			for _, axis := range []string{"x", "y", "z"} {
				av, ok := asFloat(obj[axis])
				if !ok {
					return nil, ErrValue{Field: f.Name + "." + axis, Reason: "not a number"}
				}
				cm, err := WireCenti(av)
				if err != nil {
					return nil, ErrValue{Field: f.Name + "." + axis, Reason: err.Error()}
				}
				canon[axis] = CentiMetres(cm)
			}
			yv, ok := asFloat(obj["yaw"])
			if !ok {
				return nil, ErrValue{Field: f.Name + ".yaw", Reason: "not a number"}
			}
			d, err := WireDeg(yv)
			if err != nil {
				return nil, ErrValue{Field: f.Name + ".yaw", Reason: err.Error()}
			}
			canon["yaw"] = Degrees(d)
			out[f.Name] = canon
		default:
			i, ok := asInt(raw)
			if !ok {
				return nil, ErrValue{Field: f.Name, Reason: "not an integer"}
			}
			lo, hi := int64(0), int64(255)
			switch f.Type {
			case schema.I16:
				lo, hi = -32768, 32767
			case schema.U16:
				lo, hi = 0, 65535
			case schema.I32:
				lo, hi = -2147483648, 2147483647
			case schema.U32:
				lo, hi = 0, 4294967295
			}
			if i < lo || i > hi {
				return nil, ErrValue{Field: f.Name, Reason: fmt.Sprintf("%d does not fit %s", i, f.Type)}
			}
			out[f.Name] = i
		}
	}
	for k := range v {
		found := false
		for _, f := range m.Fields {
			if f.Name == k {
				found = true
				break
			}
		}
		if !found {
			return nil, ErrValue{Field: k, Reason: "not declared in the message"}
		}
	}
	return out, nil
}
