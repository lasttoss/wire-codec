package fbufcodec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/fbufcodec"
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

// The same gate the byte and protobuf codecs are held to: the canonical value has to survive.
func TestRoundTripEveryDeclaredMessage(t *testing.T) {
	s := load(t)
	for _, m := range s.Messages {
		doc := map[string]any{}
		for _, f := range m.Fields {
			doc[f.Name] = valueFor(f.Type)
		}
		want, err := codec.Canonical(m, doc)
		if err != nil {
			t.Fatalf("%s: canonical: %v", m.Name, err)
		}
		wire, err := fbufcodec.Flatbuf{}.Encode(m, want)
		if err != nil {
			t.Fatalf("%s: encode: %v", m.Name, err)
		}
		back, err := fbufcodec.Flatbuf{}.Decode(m, wire)
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

// A buffer whose root points past its own end is a truncated frame in this format, and it has to be
// an error rather than a panic: the reader exists for the case where the sender is wrong.
func TestGarbageIsRefusedNotPanicked(t *testing.T) {
	s := load(t)
	m, err := s.ByName("Move")
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{
		"seven bytes":   {1, 2, 3, 4, 5, 6, 7},
		"root past end": {0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0},
		"zeros":         make([]byte, 16),
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked instead of returning an error: %v", name, r)
				}
			}()
			if _, err := (fbufcodec.Flatbuf{}).Decode(m, b); err == nil {
				t.Errorf("%s: refused to fail", name)
			}
		}()
	}
}

func TestGeneratedFbsSource(t *testing.T) {
	s := load(t)
	m, err := s.ByName("Everything")
	if err != nil {
		t.Fatal(err)
	}
	text, err := fbufcodec.Source(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"table Everything {", "a_u8:ubyte;", "a_i16:short;", "a_metres:double;", "root_type Everything;"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated .fbs is missing %q:\n%s", want, text)
		}
	}
}
