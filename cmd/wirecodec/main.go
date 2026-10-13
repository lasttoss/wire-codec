// Command wirecodec is the program the container, the compose lab and the chart run. It is the
// library with a command line: encode a JSON message to wire bytes, decode wire bytes back to
// JSON, and run the interchange gate over every declared codec.
//
// The lab matters more than the commands: two containers, one that encodes and one that decodes,
// talking a format neither of them was compiled for - which is the whole promise of the library
// stated as something you can watch.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/codec/bytecodec"
	"github.com/lasttoss/wire-codec/codec/flatbufcodec"
	"github.com/lasttoss/wire-codec/codec/jsoncodec"
	"github.com/lasttoss/wire-codec/codec/protocodec"
	"github.com/lasttoss/wire-codec/internal/schema"
)

func registry() map[string]codec.Codec {
	return map[string]codec.Codec{
		"byte":    bytecodec.Byte{},
		"json":    jsoncodec.JSON{},
		"flatbuf": flatbufcodec.Flatbuf{},
		"proto":   protocodec.Proto{},
	}
}

func codecNames() []string {
	names := make([]string, 0, len(registry()))
	for n := range registry() {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "wirecodec:", err)
	os.Exit(1)
}

func pick(name string) codec.Codec {
	c, ok := registry()[name]
	if !ok {
		fatal(fmt.Errorf("no codec named %q; this build has %s", name, strings.Join(codecNames(), ", ")))
	}
	return c
}

func loadSchema(path string) *schema.Schema {
	s, err := schema.Load(path)
	if err != nil {
		fatal(err)
	}
	return s
}

func message(s *schema.Schema, name string) *schema.Message {
	m, err := s.ByName(name)
	if err != nil {
		fatal(err)
	}
	return m
}

func usage() {
	fmt.Fprint(os.Stderr, `wirecodec - one declaration, several wire formats

  encode -codec byte -message Move -json '{"pos":{"x":1.25,"y":-3.5,"z":0,"yaw":90}}'
  decode -codec byte -message Move -hex 007dfea200004000
  gate                              every codec, every declared message: JSON in, wire, JSON out
  listen -addr :9100 -codec byte -expect 3     decode frames until it has seen this many
  send   -addr host:9100 -codec byte -message Move -json '...'

Flags shared by the wire commands: -schema (default $WIRECODEC_SCHEMA or schema/ops.yaml).
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "encode":
		cmdEncode(os.Args[2:])
	case "decode":
		cmdDecode(os.Args[2:])
	case "gate":
		os.Exit(cmdGate(os.Args[2:]))
	case "listen":
		cmdListen(os.Args[2:])
	case "send":
		cmdSend(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "wirecodec: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func schemaFlag(fs *flag.FlagSet) *string {
	def := os.Getenv("WIRECODEC_SCHEMA")
	if def == "" {
		def = "schema/ops.yaml"
	}
	return fs.String("schema", def, "the declaration to load")
}

func cmdEncode(args []string) {
	fs := flag.NewFlagSet("encode", flag.ExitOnError)
	sc := schemaFlag(fs)
	c := fs.String("codec", "byte", "wire format: "+strings.Join(codecNames(), ", "))
	m := fs.String("message", "", "message name")
	doc := fs.String("json", "", "the message as JSON")
	_ = fs.Parse(args)
	if *m == "" || *doc == "" {
		fs.Usage()
		os.Exit(2)
	}
	s := loadSchema(*sc)
	wire, err := codec.EncodeFromJSON(pick(*c), message(s, *m), []byte(*doc))
	if err != nil {
		fatal(err)
	}
	fmt.Printf("%s %s %d bytes %s\n", *c, *m, len(wire), hex.EncodeToString(wire))
}

func cmdDecode(args []string) {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	sc := schemaFlag(fs)
	c := fs.String("codec", "byte", "wire format")
	m := fs.String("message", "", "message name")
	h := fs.String("hex", "", "the frame, hex encoded")
	_ = fs.Parse(args)
	if *m == "" || *h == "" {
		fs.Usage()
		os.Exit(2)
	}
	wire, err := hex.DecodeString(strings.TrimPrefix(*h, "0x"))
	if err != nil {
		fatal(fmt.Errorf("-hex is not hex: %w", err))
	}
	s := loadSchema(*sc)
	out, err := codec.DecodeToJSON(pick(*c), message(s, *m), wire)
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(out))
}

// The gate the compose lab and the chart job run: the same value has to survive JSON -> wire ->
// JSON for every codec, and a truncated frame has to be refused rather than decoded into
// something plausible.
var gateCases = []struct{ message, doc string }{
	{"Move", `{"pos":{"x":1.25,"y":-3.5,"z":0,"yaw":90}}`},
	{"Move", `{"pos":{"x":-327.68,"y":327.67,"z":0.01,"yaw":0}}`},
	{"SetColor", `{"r":1,"g":2,"b":255}`},
	{"Ready", `{"ready":true}`},
	{"Say", `{"text":"xin chào"}`},
}

func cmdGate(args []string) int {
	fs := flag.NewFlagSet("gate", flag.ExitOnError)
	sc := schemaFlag(fs)
	_ = fs.Parse(args)
	s := loadSchema(*sc)
	fails, checks := 0, 0

	for _, name := range codecNames() {
		c := pick(name)
		for _, tc := range gateCases {
			m := message(s, tc.message)
			checks++
			want, err := expected(m, tc.doc)
			if err != nil {
				fmt.Printf("  FAIL %-6s %-9s %v\n", c.Name(), tc.message, err)
				fails++
				continue
			}
			wire, err := codec.EncodeFromJSON(c, m, []byte(tc.doc))
			if err != nil {
				fmt.Printf("  FAIL %-6s %-9s encode: %v\n", c.Name(), tc.message, err)
				fails++
				continue
			}
			back, err := codec.DecodeToJSON(c, m, wire)
			if err != nil {
				fmt.Printf("  FAIL %-6s %-9s decode: %v\n", c.Name(), tc.message, err)
				fails++
				continue
			}
			got, err := expected(m, string(back))
			if err != nil {
				fmt.Printf("  FAIL %-6s %-9s %v\n", c.Name(), tc.message, err)
				fails++
				continue
			}
			if !reflect.DeepEqual(want, got) {
				fmt.Printf("  FAIL %-6s %-9s %v -> %v\n", c.Name(), tc.message, want, got)
				fails++
				continue
			}
			fmt.Printf("  ok   %-6s %-9s %2d bytes\n", c.Name(), tc.message, len(wire))
		}
	}

	// A truncated frame is the failure that matters: a decoder that returns a plausible value for
	// one is worse than a decoder that returns an error.
	m := message(s, "Move")
	// Hoisted: Go will not take a composite literal directly in an if header, which is a rule
	// about parsers rather than about codecs, but it has to be obeyed either way.
	byteCodec := bytecodec.Byte{}
	whole, err := codec.EncodeFromJSON(byteCodec, m, []byte(gateCases[0].doc))
	if err != nil {
		fatal(err)
	}
	checks++
	if _, err := byteCodec.Decode(m, whole[:len(whole)-2]); err == nil {
		fmt.Println("  FAIL byte   truncated frame was decoded instead of refused")
		fails++
	} else {
		var short codec.ErrShort
		if errors.As(err, &short) {
			fmt.Printf("  ok   byte   truncated frame refused (%v)\n", err)
		} else {
			fmt.Printf("  FAIL byte   truncated frame gave %T: %v\n", err, err)
			fails++
		}
	}
	if _, err := byteCodec.Decode(m, append(append([]byte(nil), whole...), 0x00)); err == nil {
		fmt.Println("  FAIL byte   a frame with bytes the declaration does not explain was accepted")
		fails++
	} else {
		fmt.Printf("  ok   byte   trailing bytes refused (%v)\n", err)
	}

	fmt.Printf("%d checks, %d failed\n", checks, fails)
	return map[bool]int{true: 1, false: 0}[fails > 0]
}

func expected(m *schema.Message, doc string) (map[string]any, error) {
	var v map[string]any
	dec := json.NewDecoder(strings.NewReader(doc))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return codec.Canonical(m, v)
}

// The frame on the wire between the two containers: a u32 length, then the payload. The library
// says nothing about how frames are delimited - that belongs to the transport - so the lab picks
// the simplest thing that works and says so.
func writeFrame(w io.Writer, payload []byte) error {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(payload)))
	if _, err := w.Write(n[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	var n [4]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(n[:])
	if size > 1<<20 {
		return nil, fmt.Errorf("frame of %d bytes is larger than this lab allows", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func cmdListen(args []string) {
	fs := flag.NewFlagSet("listen", flag.ExitOnError)
	sc := schemaFlag(fs)
	addr := fs.String("addr", ":9100", "address to listen on")
	c := fs.String("codec", "byte", "wire format to decode")
	expect := fs.Int("expect", 0, "exit successfully after this many frames (0 = forever)")
	_ = fs.Parse(args)

	s := loadSchema(*sc)
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("listening on %s, decoding %s\n", ln.Addr(), *c)
	seen := 0
	for {
		conn, err := ln.Accept()
		if err != nil {
			fatal(err)
		}
		seen++
		r := bufio.NewReader(conn)
		frame, err := readFrame(r)
		if err != nil {
			fatal(fmt.Errorf("frame %d: %w", seen, err))
		}
		name := frameName(s, frame)
		if name == "" {
			fatal(fmt.Errorf("frame %d does not fit any declared message: %s", seen, hex.EncodeToString(frame)))
		}
		out, err := codec.DecodeToJSON(pick(*c), message(s, name), frame)
		if err != nil {
			fatal(fmt.Errorf("frame %d as %s: %w", seen, name, err))
		}
		fmt.Printf("frame %d: %s %s -> %s\n", seen, name, hex.EncodeToString(frame), out)
		_ = conn.Close()
		if *expect > 0 && seen >= *expect {
			fmt.Printf("decoded %d frames, exiting\n", seen)
			return
		}
	}
}

// frameName finds the declared message a frame is a valid encoding of. The byte layout carries no
// message id, so this is how the lab decides what it just received - and the honest way to say it
// is that the transport owes us that byte, not the codec.
func frameName(s *schema.Schema, frame []byte) string {
	byteCodec := bytecodec.Byte{}
	for _, m := range s.Messages {
		if _, err := byteCodec.Decode(m, frame); err == nil {
			return m.Name
		}
	}
	return ""
}

func cmdSend(args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	sc := schemaFlag(fs)
	addr := fs.String("addr", "127.0.0.1:9100", "address to send to")
	c := fs.String("codec", "byte", "wire format to encode in")
	m := fs.String("message", "", "message name")
	doc := fs.String("json", "", "the message as JSON")
	tries := fs.Int("retries", 20, "connection attempts before giving up")
	_ = fs.Parse(args)
	if *m == "" || *doc == "" {
		fs.Usage()
		os.Exit(2)
	}

	s := loadSchema(*sc)
	wire, err := codec.EncodeFromJSON(pick(*c), message(s, *m), []byte(*doc))
	if err != nil {
		fatal(err)
	}

	// No shell in the image means no retry loop in YAML, so the retry lives here: the sender may
	// be up before the listener is.
	var conn net.Conn
	for i := 0; i < *tries; i++ {
		conn, err = net.DialTimeout("tcp", *addr, 2*time.Second)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		fatal(fmt.Errorf("could not reach %s in %d attempts: %w", *addr, *tries, err))
	}
	defer conn.Close()
	if err := writeFrame(conn, wire); err != nil {
		fatal(err)
	}
	fmt.Printf("sent %s %s as %s: %d bytes %s\n", *c, *m, *addr, len(wire), hex.EncodeToString(wire))
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		fatal(err)
	}
}
