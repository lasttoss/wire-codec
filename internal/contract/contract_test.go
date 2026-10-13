package contract_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/bytecodec"
	"github.com/lasttoss/wire-codec/codec/fbufcodec"
	"github.com/lasttoss/wire-codec/codec/jsoncodec"
	"github.com/lasttoss/wire-codec/codec/protocodec"
	"github.com/lasttoss/wire-codec/internal/contract"
	"github.com/lasttoss/wire-codec/internal/schema"
)

const (
	schemaPath = "../../schema/ops.yaml"
	clientDir  = "../../examples/contract"
	tsPath     = "../../examples/ts-client/contract.ts"
)

func load(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Load(schemaPath)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

// TestArtefactsAreCurrent is the drift gate: the committed artefacts are compared byte for byte
// with what the declaration generates now. A client compiling yesterday's .proto against today's
// server is the failure this test exists to make impossible, and it is a diff rather than a review
// because reviews are the part that does not happen.
func TestArtefactsAreCurrent(t *testing.T) {
	s := load(t)
	dir := t.TempDir()
	if err := contract.Generate(s, dir); err != nil {
		t.Fatalf("generate: %v", err)
	}
	got, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, e := range got {
		if e.IsDir() {
			for _, sub := range []string{"proto", "fbs"} {
				names, err := os.ReadDir(filepath.Join(dir, sub))
				if err != nil {
					t.Fatal(err)
				}
				for _, n := range names {
					compare(t, filepath.Join(dir, sub, n.Name()), filepath.Join(clientDir, sub, n.Name()))
					compared++
				}
			}
			continue
		}
		compare(t, filepath.Join(dir, e.Name()), filepath.Join(clientDir, e.Name()))
		compared++
	}
	if compared == 0 {
		t.Fatal("nothing was compared; the artefact list is empty, which is not the same as current")
	}
}

func compare(t *testing.T, generated, committed string) {
	t.Helper()
	want, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	have, err := os.ReadFile(committed)
	if err != nil {
		t.Fatalf("%s: %v (run `make contract`)", committed, err)
	}
	if !bytes.Equal(want, have) {
		t.Errorf("%s is not what the declaration generates (run `make contract`)", committed)
	}
}

// TestDocumentsSurviveEveryCodec is the client's half of the interchange gate: the JSON a client
// sends is put through every codec and comes back as the same value. A fixture that only the JSON
// codec can read is a fixture that would fail in production, at the client.
func TestDocumentsSurviveEveryCodec(t *testing.T) {
	s := load(t)
	raw, err := os.ReadFile(filepath.Join(clientDir, "documents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var docs []contract.Document
	if err := json.Unmarshal(raw, &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != len(s.Messages) {
		t.Fatalf("documents.json covers %d messages, the declaration has %d", len(docs), len(s.Messages))
	}
	codecs := []codec.Codec{bytecodec.Byte{}, jsoncodec.JSON{}, protocodec.Proto{}, fbufcodec.Flatbuf{}}
	for _, d := range docs {
		m, err := s.ByName(d.Message)
		if err != nil {
			t.Fatal(err)
		}
		want, err := codec.Canonical(m, decode(t, d.JSON))
		if err != nil {
			t.Fatalf("%s: canonical: %v", d.Message, err)
		}
		for _, c := range codecs {
			wire, err := codec.EncodeFromJSON(c, m, d.JSON)
			if err != nil {
				t.Errorf("%s/%s: encode: %v", c.Name(), d.Message, err)
				continue
			}
			back, err := codec.DecodeToJSON(c, m, wire)
			if err != nil {
				t.Errorf("%s/%s: decode: %v", c.Name(), d.Message, err)
				continue
			}
			got, err := codec.Canonical(m, decode(t, back))
			if err != nil {
				t.Errorf("%s/%s: canonical: %v", c.Name(), d.Message, err)
				continue
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("%s/%s: %v != %v", c.Name(), d.Message, want, got)
			}
		}
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestTypescriptMirrorsTheDeclaration keeps the file a client team reads from describing a
// contract that no longer exists. It is a grep, and it is a grep because the alternative - reading
// the TypeScript - is something nobody does on the day they add a field.
func TestTypescriptMirrorsTheDeclaration(t *testing.T) {
	missing, err := contract.Missing(load(t), tsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("%s does not mention %v", tsPath, missing)
	}
}
