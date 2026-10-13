// Command contract writes the client-side artefacts: the .proto and .fbs a client team compiles
// with its own toolchain, and the canonical JSON fixtures. It is the answer to "where do I get the
// files" without adding a protoc or flatc dependency to this repository.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lasttoss/wire-codec/internal/contract"
	"github.com/lasttoss/wire-codec/internal/schema"
)

func main() {
	schemaPath := flag.String("schema", "schema/ops.yaml", "the declaration")
	out := flag.String("out", "examples/contract", "directory to write the artefacts into")
	check := flag.String("check", "examples/ts-client/contract.ts", "the client TypeScript to check against the declaration")
	flag.Parse()

	s, err := schema.Load(*schemaPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract:", err)
		os.Exit(1)
	}
	if err := contract.Generate(s, *out); err != nil {
		fmt.Fprintln(os.Stderr, "contract:", err)
		os.Exit(1)
	}
	missing, err := contract.Missing(s, *check)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract:", err)
		os.Exit(1)
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "contract: %s does not mention %v\n", *check, missing)
		fmt.Fprintln(os.Stderr, "the TypeScript mirror is part of the contract; update it with the declaration")
		os.Exit(1)
	}
	fmt.Printf("  %d messages: proto/ and fbs/ written, documents.json written, %s covers every name\n",
		len(s.Messages), *check)
}
