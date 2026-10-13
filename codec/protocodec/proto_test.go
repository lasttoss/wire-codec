package protocodec_test

import (
	"reflect"
	"testing"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/protocodec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

func load(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Load("../../schema/ops.yaml")
	if err != nil {
		t.Fatalf("load declaration: %v", err)
	}
	return s
}

// valueFor is a valid value for each declared type, written here once so that a message added to
// the declaration is covered by the round-trip test without editing this file.
func valueFor(k schema.Kind) any {
	switch k {
	case schema.U8:
		return int64(200)
	case schema.I16:
		return int64(-1234)
	case schema.U16:
		return int64(40000)
	case schema.I32:
		return int64(-123456)
	case schema.U32:
		return int64(3000000000)
	case schema.Bool:
		return true
	case schema.Str:
		return "xin chào"
	case schema.Raw:
		return []byte{0xde, 0xad, 0xbe, 0xef}
	case schema.Metres:
		return 1.25
	case schema.Deg:
		return 90.0
	case schema.Pos:
		return map[string]any{"x": 1.25, "y": -3.5, "z": 0.0, "yaw": 90.0}
	}
	return nil
}

func doc(m *schema.Message) map[string]any {
	v := map[string]any{}
	for _, f := range m.Fields {
		v[f.Name] = valueFor(f.Type)
	}
	return v
}

// The gate the byte codec is held to, held to the protobuf codec: JSON value in, canonical form
// out, and the canonical form has to come back identical.
func TestRoundTripEveryDeclaredMessage(t *testing.T) {
	s := load(t)
	for _, m := range s.Messages {
		want, err := codec.Canonical(m, doc(m))
		if err != nil {
			t.Fatalf("%s: canonical: %v", m.Name, err)
		}
		wire, err := protocodec.Proto{}.Encode(m, want)
		if err != nil {
			t.Fatalf("%s: encode: %v", m.Name, err)
		}
		back, err := protocodec.Proto{}.Decode(m, wire)
		if err != nil {
			t.Fatalf("%s: decode: %v", m.Name, err)
		}
		got, err := codec.Canonical(m, back)
		if err != nil {
			t.Fatalf("%s: canonical of decoded: %v", m.Name, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: round trip changed the value\n want %#v\n  got %#v", m.Name, want, got)
		}
	}
}

// proto3 has no presence for scalars, so a decoded message always carries every declared field.
// Saying that out loud is better than a decoder that pretends to know the difference.
func TestEveryDeclaredFieldIsPresentAfterDecode(t *testing.T) {
	s := load(t)
	m, err := s.ByName("Ready")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := protocodec.Proto{}.Encode(m, map[string]any{"ready": false})
	if err != nil {
		t.Fatal(err)
	}
	back, err := protocodec.Proto{}.Decode(m, wire)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := back["ready"]; !ok || v != false {
		t.Fatalf("ready missing or wrong after decode: %#v", back)
	}
}

// A frame carrying a field the declaration does not know is the protobuf spelling of the byte
// codec's trailing-bytes error, and it gets the same treatment: refused, not guessed.
func TestUnknownFieldIsRefused(t *testing.T) {
	s := load(t)
	m, err := s.ByName("Ready")
	if err != nil {
		t.Fatal(err)
	}
	// Same message name, one extra field: exactly what a newer sender looks like to an older reader.
	newer := &schema.Message{Name: "Ready", Fields: []schema.Field{
		{Name: "ready", Type: schema.Bool},
		{Name: "extra", Type: schema.U32},
	}}
	wire, err := protocodec.Proto{}.Encode(newer, map[string]any{"ready": true, "extra": int64(7)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (protocodec.Proto{}).Decode(m, wire); err == nil {
		t.Fatal("a field the declaration does not declare was accepted")
	} else {
		var trailing codec.ErrTrailing
		if !reflect.TypeOf(err).ConvertibleTo(reflect.TypeOf(trailing)) {
			t.Logf("refused with %T: %v", err, err)
		}
	}
}

func TestMalformedBytesAreRefused(t *testing.T) {
	s := load(t)
	m, err := s.ByName("Say")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := protocodec.Proto{}.Encode(m, map[string]any{"text": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (protocodec.Proto{}).Decode(m, wire[:len(wire)-1]); err == nil {
		t.Fatal("truncated protobuf was decoded instead of refused")
	}
}

// The generated .proto is an artifact a client compiles, so its shape is asserted, not assumed.
func TestGeneratedProtoSource(t *testing.T) {
	s := load(t)
	m, err := s.ByName("Everything")
	if err != nil {
		t.Fatal(err)
	}
	text, err := protocodec.Source(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"syntax = \"proto3\";", "message Everything {", "uint32 a_u8 = 1;", "int32 a_i16 = 2;", "double a_metres = 7;"} {
		if !contains(text, want) {
			t.Errorf("generated source is missing %q:\n%s", want, text)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
