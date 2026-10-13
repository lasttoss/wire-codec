// Package jsoncodec is the reference format. It is not a wire format for players - it is the
// interchange a tool, a test or a client library speaks, and the thing the other codecs are
// checked against.
package jsoncodec

import (
	"encoding/json"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

type JSON struct{}

func (JSON) Name() string { return "json" }

func (JSON) Encode(m *schema.Message, v map[string]any) ([]byte, error) {
	canon, err := codec.Canonical(m, v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canon)
}

func (JSON) Decode(m *schema.Message, b []byte) (map[string]any, error) {
	var v map[string]any
	dec := json.NewDecoder(byteReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, codec.ErrValue{Field: m.Name, Reason: "not JSON: " + err.Error()}
	}
	return codec.Canonical(m, v)
}
