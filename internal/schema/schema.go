// Package schema reads the one file that declares a message set, and knows what each declared
// type means on the wire. Every codec in this repository is driven by this, so the declaration
// is the only thing that has to stay true.
package schema

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Kind is a declared logical type, not a wire type: the wire type follows from the codec.
type Kind string

const (
	U8     Kind = "u8"
	I16    Kind = "i16"
	U16    Kind = "u16"
	I32    Kind = "i32"
	U32    Kind = "u32"
	Bool   Kind = "bool"
	Str    Kind = "str"
	Raw    Kind = "raw"
	Pos    Kind = "pos"    // x, y, z in metres plus yaw in degrees
	Deg    Kind = "deg"    // a turn, quantised to a uint16
	Metres Kind = "metres" // a distance, quantised to int16 centimetres
)

var kinds = map[Kind]bool{U8: true, I16: true, U16: true, I32: true, U32: true,
	Bool: true, Str: true, Raw: true, Pos: true, Deg: true, Metres: true}

type Field struct {
	Name string `yaml:"name"`
	Type Kind   `yaml:"type"`
}

type Message struct {
	Name   string
	Opcode uint8
	Fields []Field
}

type Schema struct {
	Messages []*Message
}

func (s *Schema) ByName(name string) (*Message, error) {
	for _, m := range s.Messages {
		if m.Name == name {
			return m, nil
		}
	}
	return nil, fmt.Errorf("no message named %q in the declaration", name)
}

type rawMessage struct {
	Opcode uint8   `yaml:"opcode"`
	Fields []Field `yaml:"fields"`
}

// Load reads the declaration and refuses to return a schema it cannot defend: a duplicate
// opcode, an unknown type, a repeated field name and a `raw` that is not last are all errors
// here, because each of them would otherwise become a wrong byte somewhere later.
func Load(path string) (*Schema, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Messages map[string]rawMessage `yaml:"messages"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	names := make([]string, 0, len(raw.Messages))
	for n := range raw.Messages {
		names = append(names, n)
	}
	sort.Strings(names)

	s := &Schema{}
	seenOpcode := map[uint8]string{}
	for _, n := range names {
		rm := raw.Messages[n]
		if prev, dup := seenOpcode[rm.Opcode]; dup {
			return nil, fmt.Errorf("opcode %d is claimed by both %s and %s", rm.Opcode, prev, n)
		}
		seenOpcode[rm.Opcode] = n
		m := &Message{Name: n, Opcode: rm.Opcode, Fields: rm.Fields}
		seenField := map[string]bool{}
		for i, f := range m.Fields {
			if f.Name == "" {
				return nil, fmt.Errorf("%s: field %d has no name", n, i)
			}
			if seenField[f.Name] {
				return nil, fmt.Errorf("%s: field %q declared twice", n, f.Name)
			}
			seenField[f.Name] = true
			if !kinds[f.Type] {
				return nil, fmt.Errorf("%s.%s: unknown type %q", n, f.Name, f.Type)
			}
			if f.Type == Raw && i != len(m.Fields)-1 {
				return nil, fmt.Errorf("%s.%s: raw has no length prefix, so it can only be the last field", n, f.Name)
			}
		}
		s.Messages = append(s.Messages, m)
	}
	if len(s.Messages) == 0 {
		return nil, fmt.Errorf("%s: no messages declared", path)
	}
	return s, nil
}
