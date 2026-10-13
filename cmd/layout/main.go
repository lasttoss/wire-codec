// Command layout prints the byte layout of every declared message. The document that ships in
// docs/ is this command's output, so a layout table cannot drift away from the declaration: it
// is regenerated, not maintained.
package main

import (
	"fmt"
	"os"

	"github.com/lasttoss/wire-codec/internal/schema"
)

func wire(t schema.Kind) (string, string) {
	switch t {
	case schema.U8, schema.Bool:
		return "u8", "1"
	case schema.I16:
		return "i16 BE", "2"
	case schema.U16:
		return "u16 BE", "2"
	case schema.I32:
		return "i32 BE", "4"
	case schema.U32:
		return "u32 BE", "4"
	case schema.Metres:
		return "i16 BE (cm)", "2"
	case schema.Deg:
		return "u16 BE (1/65535 turn)", "2"
	case schema.Pos:
		return "i16 x,y,z BE (cm) + u16 BE (yaw)", "8"
	case schema.Str:
		return "u16 BE length + UTF-8", "2 + N"
	case schema.Raw:
		return "opaque, to the end", "N"
	}
	return "?", "?"
}

func main() {
	path := "schema/ops.yaml"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	s, err := schema.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("# Byte layout, generated from `%s`\n\n", path)
	fmt.Println("> Read the declaration, not this file: `go run ./cmd/layout > docs/byte-layout.md`.")
	fmt.Println("> Wire format is big-endian throughout, and there are no tags and no field ids - the order")
	fmt.Println("> of the declaration *is* the format. That is the cost of this codec, and the reason it is")
	fmt.Println("> the smallest: a fixed-size message costs exactly the sum below, known before it is sent.")
	for _, m := range s.Messages {
		fmt.Printf("\n## %s (opcode %d)\n\n| field | declared type | on the wire | bytes |\n|---|---|---|---|\n", m.Name, m.Opcode)
		for _, f := range m.Fields {
			w, n := wire(f.Type)
			fmt.Printf("| `%s` | `%s` | %s | %s |\n", f.Name, f.Type, w, n)
		}
	}
}
