package codec_test

import (
	"bytes"
	"testing"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/bytecodec"
)

// These bytes come from a second, independent implementation (the short Python in
// docs/golden-bytes.md), not from this codec. A round trip through one implementation only proves
// that implementation agrees with itself - which is exactly what a byte-order bug does. This is
// the test that disagrees when the two implementations disagree.
var golden = []struct {
	message string
	doc     string
	wire    []byte
	hex     string
}{
	{"Move", `{"pos":{"x":1.25,"y":-3.5,"z":0,"yaw":90}}`, []byte{0x00, 0x7d, 0xfe, 0xa2, 0x00, 0x00, 0x40, 0x00}, "007dfea200004000"},
	{"Move", `{"pos":{"x":-327.68,"y":327.67,"z":0.01,"yaw":0}}`, []byte{0x80, 0x00, 0x7f, 0xff, 0x00, 0x01, 0x00, 0x00}, "80007fff00010000"},
	{"SetColor", `{"r":1,"g":2,"b":255}`, []byte{0x01, 0x02, 0xff}, "0102ff"},
	{"Ready", `{"ready":true}`, []byte{0x01}, "01"},
	{"Say", `{"text":"xin chào"}`, []byte{0x00, 0x09, 0x78, 0x69, 0x6e, 0x20, 0x63, 0x68, 0xc3, 0xa0, 0x6f}, "000978696e206368c3a06f"},
	{"Everything", `{"a_u8":7,"a_i16":-1234,"a_u16":60000,"a_i32":-70000,"a_u32":4000000000,"a_bool":true,"a_metres":12.34,"a_deg":271.5,"a_str":"hello","a_raw":"3q2+7w=="}`, []byte{0x07, 0xfb, 0x2e, 0xea, 0x60, 0xff, 0xfe, 0xee, 0x90, 0xee, 0x6b, 0x28, 0x00, 0x01, 0x04, 0xd2, 0xc1, 0x10, 0x00, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xde, 0xad, 0xbe, 0xef}, "07fb2eea60fffeee90ee6b28000104d2c110000568656c6c6fdeadbeef"},
}

func TestGoldenBytes(t *testing.T) {
	s := loadSchema(t)
	for _, g := range golden {
		m, err := s.ByName(g.message)
		if err != nil {
			t.Fatal(err)
		}
		got, err := codec.EncodeFromJSON(bytecodec.Byte{}, m, []byte(g.doc))
		if err != nil {
			t.Fatalf("%s: %v", g.message, err)
		}
		if !bytes.Equal(got, g.wire) {
			t.Errorf("%s: the wire bytes disagree with the independent implementation: got %x, want %s", g.message, got, g.hex)
		}
		if _, err := codec.DecodeToJSON(bytecodec.Byte{}, m, g.wire); err != nil {
			t.Errorf("%s: decoding the golden bytes failed: %v", g.message, err)
		}
	}
}

// A declaration is only tested where a case exists. This asserts that every message in the
// declaration has one, so adding a message and forgetting its golden bytes fails here instead of
// leaving a wire type that no test has ever encoded.
func TestEveryDeclaredMessageHasAGoldenCase(t *testing.T) {
	s := loadSchema(t)
	covered := map[string]bool{}
	for _, g := range golden {
		covered[g.message] = true
	}
	for _, m := range s.Messages {
		if !covered[m.Name] {
			t.Errorf("message %s is declared but no golden case encodes it", m.Name)
		}
	}
}
