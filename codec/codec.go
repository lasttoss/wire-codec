// Package codec is the adapter: one interface, one declaration, three wire formats. A game's
// logic talks to this interface and never to bytes, which is what makes the wire format a
// configuration decision rather than a rewrite.
package codec

import (
	"encoding/json"
	"fmt"

	"github.com/lasttoss/wire-codec/internal/schema"
)

type Codec interface {
	// Name is the wire format, and it is what appears in configuration: "byte", "proto", "flatbuf".
	Name() string
	Encode(m *schema.Message, v map[string]any) ([]byte, error)
	Decode(m *schema.Message, b []byte) (map[string]any, error)
}

// ErrShort, ErrTrailing and ErrValue are separate types because each one means a different
// failure: a truncated frame, a frame with bytes nobody declared, and a value that cannot be
// represented. A decoder that returns a zero value for any of them is the thing this repository
// exists to avoid - see the fault tests.
type ErrShort struct {
	Field  string
	Need   int
	Have   int
}

func (e ErrShort) Error() string {
	return fmt.Sprintf("%s: need %d bytes, have %d", e.Field, e.Need, e.Have)
}

type ErrTrailing struct{ Bytes int }

func (e ErrTrailing) Error() string {
	return fmt.Sprintf("%d bytes after the last declared field: the declaration and the sender disagree", e.Bytes)
}

type ErrValue struct {
	Field  string
	Reason string
}

func (e ErrValue) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

// EncodeFromJSON is the seam the requirement is about: JSON in, wire bytes out. It is defined
// once here so that every codec gets it, and a format added later cannot forget it.
func EncodeFromJSON(c Codec, m *schema.Message, doc []byte) ([]byte, error) {
	var v map[string]any
	dec := json.NewDecoder(bytesReader(doc))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: %w", m.Name, err)
	}
	canon, err := Canonical(m, v)
	if err != nil {
		return nil, err
	}
	return c.Encode(m, canon)
}

// DecodeToJSON is the other direction: wire bytes in, JSON out.
func DecodeToJSON(c Codec, m *schema.Message, wire []byte) ([]byte, error) {
	v, err := c.Decode(m, wire)
	if err != nil {
		return nil, err
	}
	canon, err := Canonical(m, v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canon)
}
