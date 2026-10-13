// Package contract emits what a client team compiles: the .proto, the .fbs and the canonical JSON
// for every declared message. The Go codecs are the server side of the same declaration, so a
// client that compiles these files and a server that encodes with codec/ cannot disagree about a
// field name without a test failing first - which is the difference between a contract and a
// document that used to be true.
package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/fbufcodec"
	"github.com/lasttoss/wire-codec/codec/protocodec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

// Document is one message's canonical JSON. It is produced by decoding the example and quantising
// it through codec.Canonical, never by hand: a hand-written fixture is a second opinion about the
// wire shape, and the second opinion is the one that drifts.
type Document struct {
	Message string          `json:"message"`
	Opcode  uint8           `json:"opcode"`
	JSON    json.RawMessage `json:"json"`
}

// examples are the values a client would actually send. Everything is here on purpose: it is the
// message that carries every declared type, so a fixture that covers it covers the wire types.
var examples = map[string]string{
	"Move":       `{"pos":{"x":1.25,"y":-3.5,"z":0,"yaw":90}}`,
	"SetColor":   `{"r":1,"g":2,"b":255}`,
	"Ready":      `{"ready":true}`,
	"Say":        `{"text":"xin chào"}`,
	"Everything": `{"a_u8":7,"a_i16":-300,"a_u16":40000,"a_i32":-70000,"a_u32":3000000000,"a_bool":true,"a_metres":12.34,"a_deg":45.5,"a_str":"everything","a_raw":"AQIDBA=="}`,
}

// byOpcode is the declaration's own order: the wire order. Iterating a map would put a different
// diff in front of a reviewer on every run.
func byOpcode(s *schema.Schema) []*schema.Message {
	out := append([]*schema.Message(nil), s.Messages...)
	sort.Slice(out, func(i, j int) bool { return out[i].Opcode < out[j].Opcode })
	return out
}

// Generate writes the client-side artefacts for every declared message under dir.
func Generate(s *schema.Schema, dir string) error {
	for _, sub := range []string{"proto", "fbs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	docs := make([]Document, 0, len(s.Messages))
	for _, m := range byOpcode(s) {
		proto, err := protocodec.Source(m)
		if err != nil {
			return err
		}
		fbs, err := fbufcodec.Source(m)
		if err != nil {
			return err
		}
		// The comment says where the file came from, because the first question a reviewer asks
		// about a generated file is which file to edit instead.
		proto = "// generated from the declaration by cmd/contract; edit schema/ops.yaml, not this\n" + proto
		if err := writeFile(filepath.Join(dir, "proto", m.Name+".proto"), proto); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, "fbs", m.Name+".fbs"), fbs); err != nil {
			return err
		}
		doc, ok := examples[m.Name]
		if !ok {
			return fmt.Errorf("no example value for declared message %q: a fixture is part of the contract", m.Name)
		}
		v, err := codec.Canonical(m, mustDecode(m.Name, doc))
		if err != nil {
			return err
		}
		canon, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		docs = append(docs, Document{Message: m.Name, Opcode: m.Opcode, JSON: canon})
	}
	// One trailing newline, so the file is a file and not the output of a program that was cut off.
	buf, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "documents.json"), string(buf)+"\n")
}

func mustDecode(name, doc string) map[string]any {
	var v map[string]any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		panic(fmt.Sprintf("%s: the example value is not JSON: %v", name, err))
	}
	return v
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

// Missing returns the message and field names the client-side TypeScript does not mention. The
// TypeScript is hand-written, because it is the file a client team reads and edits; the check is
// what keeps it from becoming a description of last quarter's contract.
func Missing(s *schema.Schema, tsPath string) ([]string, error) {
	b, err := os.ReadFile(tsPath)
	if err != nil {
		return nil, err
	}
	text := string(b)
	var missing []string
	for _, m := range byOpcode(s) {
		if !strings.Contains(text, m.Name) {
			missing = append(missing, m.Name)
		}
		for _, f := range m.Fields {
			if !strings.Contains(text, f.Name) {
				missing = append(missing, m.Name+"."+f.Name)
			}
		}
	}
	return missing, nil
}
