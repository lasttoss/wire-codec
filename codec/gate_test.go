package codec_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/bytecodec"
	"github.com/lasttoss/wire-codec/codec/jsoncodec"
	"github.com/lasttoss/wire-codec/codec/protocodec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

func loadSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Load("../schema/ops.yaml")
	if err != nil {
		t.Fatalf("load the declaration: %v", err)
	}
	return s
}

// allCodecs is the list every format test runs over. A new wire format is added here once and
// is then held to the same three gates as the others.
func allCodecs() []codec.Codec {
	return []codec.Codec{jsoncodec.JSON{}, bytecodec.Byte{}, protocodec.Proto{}}
}

func canonical(t *testing.T, m *schema.Message, doc string) map[string]any {
	t.Helper()
	var v map[string]any
	dec := json.NewDecoder(stringsReader(doc))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("bad test case: %v", err)
	}
	canon, err := codec.Canonical(m, v)
	if err != nil {
		t.Fatalf("the declaration rejects the test case %s: %v", doc, err)
	}
	return canon
}

var cases = []struct {
	message string
	doc     string
}{
	{"Move", `{"pos":{"x":1.25,"y":-3.5,"z":0,"yaw":90}}`},
	{"Move", `{"pos":{"x":-327.68,"y":327.67,"z":0.01,"yaw":0}}`},
	{"SetColor", `{"r":1,"g":2,"b":255}`},
	{"Ready", `{"ready":true}`},
	{"Ready", `{"ready":false}`},
	{"Say", `{"text":"xin chào"}`},
	{"Say", `{"text":""}`},
}

// Gate 1: JSON in, wire out, wire back in, JSON out - and the value has to be the same one.
// This is the assertion that lets a project change its wire format without changing its game.
func TestEveryCodecAgreesOnTheValue(t *testing.T) {
	s := loadSchema(t)
	for _, c := range allCodecs() {
		for _, tc := range cases {
			m, err := s.ByName(tc.message)
			if err != nil {
				t.Fatal(err)
			}
			want := canonical(t, m, tc.doc)
			wire, err := codec.EncodeFromJSON(c, m, []byte(tc.doc))
			if err != nil {
				t.Fatalf("%s/%s: encode: %v", c.Name(), tc.message, err)
			}
			back, err := codec.DecodeToJSON(c, m, wire)
			if err != nil {
				t.Fatalf("%s/%s: decode: %v", c.Name(), tc.message, err)
			}
			var got map[string]any
			if err := json.Unmarshal(back, &got); err != nil {
				t.Fatalf("%s/%s: the decoder did not produce JSON: %v", c.Name(), tc.message, err)
			}
			gotCanon, err := codec.Canonical(m, got)
			if err != nil {
				t.Fatalf("%s/%s: %v", c.Name(), tc.message, err)
			}
			if !reflect.DeepEqual(want, gotCanon) {
				t.Errorf("%s/%s: round trip changed the value\n  sent: %#v\n  got:  %#v", c.Name(), tc.message, want, gotCanon)
			}
		}
	}
}

// Gate 2: a decoder that returns a plausible value for a broken frame is worse than one that
// returns an error. Each case below is a different way to be broken.
func TestBrokenFramesAreRefused(t *testing.T) {
	s := loadSchema(t)
	move, _ := s.ByName("Move")
	say, _ := s.ByName("Say")
	c := bytecodec.Byte{}

	good, err := codec.EncodeFromJSON(c, move, []byte(`{"pos":{"x":1,"y":2,"z":3,"yaw":45}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(good) != 8 {
		t.Fatalf("Move should be 8 bytes, got %d", len(good))
	}

	cases := []struct {
		name  string
		msg   *schema.Message
		frame []byte
		want  any
	}{
		{"truncated", move, good[:5], codec.ErrShort{}},
		{"empty", move, nil, codec.ErrShort{}},
		{"trailing bytes", move, append(append([]byte(nil), good...), 0x00), codec.ErrTrailing{}},
		{"string shorter than its length prefix", say, []byte{0x00, 0x05, 'a'}, codec.ErrShort{}},
	}
	for _, tc := range cases {
		_, err := c.Decode(tc.msg, tc.frame)
		if err == nil {
			t.Errorf("%s: decoded a broken frame instead of refusing it", tc.name)
			continue
		}
		switch tc.want.(type) {
		case codec.ErrShort:
			var e codec.ErrShort
			if !errors.As(err, &e) {
				t.Errorf("%s: want a short-frame error, got %T: %v", tc.name, err, err)
			}
		case codec.ErrTrailing:
			var e codec.ErrTrailing
			if !errors.As(err, &e) {
				t.Errorf("%s: want a trailing-bytes error, got %T: %v", tc.name, err, err)
			}
		}
	}
}

// A value that does not fit the wire type is refused rather than clamped: 400 metres is not
// 327.67 metres, and a decoder that says so is the only one worth having.
func TestValuesThatDoNotFitAreRefused(t *testing.T) {
	s := loadSchema(t)
	move, _ := s.ByName("Move")
	if _, err := codec.EncodeFromJSON(bytecodec.Byte{}, move, []byte(`{"pos":{"x":400,"y":0,"z":0,"yaw":0}}`)); err == nil {
		t.Error("400 metres fitted into int16 centimetres without an error")
	}
	if _, err := codec.EncodeFromJSON(bytecodec.Byte{}, move, []byte(`{"pos":{"x":1,"y":2,"z":3,"yaw":45},"nope":1}`)); err == nil {
		t.Error("a field the declaration does not have was accepted")
	}
}

// The byte format's whole argument is that its cost is known. These numbers are asserted, not
// logged, so that a change to the declaration shows up here as a size change.
func TestDeclaredSizes(t *testing.T) {
	s := loadSchema(t)
	want := map[string]int{"Move": 8, "SetColor": 3, "Ready": 1}
	for name, wantN := range want {
		m, err := s.ByName(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := bytecodec.Size(m)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != wantN {
			t.Errorf("%s is %d bytes on the byte wire, expected %d", name, got, wantN)
		}
	}
	say, _ := s.ByName("Say")
	if _, err := bytecodec.Size(say); err == nil {
		t.Error("a variable-length message reported a fixed size")
	}
}
