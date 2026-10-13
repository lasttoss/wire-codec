// Package protocodec speaks the declaration in protobuf, without protoc and without a generated
// file per message: the descriptor is compiled from generated .proto text at run time, so adding a
// message to the declaration is the only edit needed. The generated text is a real .proto, which is
// what a C#, TypeScript or Unity client compiles with its own toolchain.
package protocodec

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/bufbuild/protocompile"
	"github.com/lasttoss/wire-codec/codec"
	"github.com/lasttoss/wire-codec/internal/schema"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Proto is stateless on purpose: the declaration arrives with the message, so the same value can be
// used from a config file without a constructor argument that has to be kept in step.
type Proto struct{}

func (Proto) Name() string { return "proto" }

var (
	mu    sync.Mutex
	cache = map[string]protoreflect.MessageDescriptor{}
)

// WireType is the protobuf type each logical type maps to. Nothing here is uint8 or int16: proto3
// has neither, so the logical range is what the declaration enforces, and the type is what exists.
func WireType(k schema.Kind) string {
	switch k {
	case schema.U8, schema.U16, schema.U32:
		return "uint32"
	case schema.I16, schema.I32:
		return "int32"
	case schema.Bool:
		return "bool"
	case schema.Str:
		return "string"
	case schema.Raw:
		return "bytes"
	case schema.Metres, schema.Deg:
		return "double"
	case schema.Pos:
		return "Pos"
	}
	return ""
}

// Source generates the .proto text for one message. It is exported because the text is an artifact
// in its own right: a client team compiles this, and a diff of it is the contract change.
func Source(m *schema.Message) (string, error) {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\n\nmessage Pos { double x = 1; double y = 2; double z = 3; double yaw = 4; }\n\n")
	fmt.Fprintf(&b, "message %s {\n", m.Name)
	for i, f := range m.Fields {
		t := WireType(f.Type)
		if t == "" {
			return "", fmt.Errorf("%s: no protobuf type for %q", f.Name, f.Type)
		}
		fmt.Fprintf(&b, "  %s %s = %d;\n", t, f.Name, i+1)
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// key is the name *and* the fields: two declarations can share a name and differ, and a cache
// keyed by name alone would hand one of them the other's descriptor.
func key(m *schema.Message) string {
	parts := make([]string, 0, len(m.Fields))
	for _, f := range m.Fields {
		parts = append(parts, f.Name+":"+string(f.Type))
	}
	return m.Name + "|" + strings.Join(parts, ",")
}

func descriptor(m *schema.Message) (protoreflect.MessageDescriptor, error) {
	k := key(m)
	mu.Lock()
	md, ok := cache[k]
	mu.Unlock()
	if ok {
		return md, nil
	}
	text, err := Source(m)
	if err != nil {
		return nil, err
	}
	comp := protocompile.Compiler{Resolver: protocompile.WithStandardImports(
		&protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(
			map[string]string{"wire.proto": text})})}
	files, err := comp.Compile(context.Background(), "wire.proto")
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", m.Name, err)
	}
	var found protoreflect.MessageDescriptor
	msgs := files[0].Messages()
	for i := 0; i < msgs.Len(); i++ {
		if string(msgs.Get(i).Name()) == m.Name {
			found = msgs.Get(i)
		}
	}
	if found == nil {
		return nil, fmt.Errorf("%s: not found after compiling its own source", m.Name)
	}
	mu.Lock()
	cache[k] = found
	mu.Unlock()
	return found, nil
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case uint64:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// Encode writes the canonical value as protobuf. The value has already been through
// codec.Canonical, so the only checks here are about this codec's own types.
func (Proto) Encode(m *schema.Message, v map[string]any) ([]byte, error) {
	md, err := descriptor(m)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	for _, f := range m.Fields {
		fd := md.Fields().ByName(protoreflect.Name(f.Name))
		if fd == nil {
			return nil, codec.ErrValue{Field: f.Name, Reason: "not in the compiled descriptor"}
		}
		raw, present := v[f.Name]
		if !present {
			continue
		}
		switch f.Type {
		case schema.Bool:
			b, ok := raw.(bool)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a boolean"}
			}
			msg.Set(fd, protoreflect.ValueOfBool(b))
		case schema.Str:
			s, ok := raw.(string)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a string"}
			}
			msg.Set(fd, protoreflect.ValueOfString(s))
		case schema.Raw:
			switch b := raw.(type) {
			case []byte:
				msg.Set(fd, protoreflect.ValueOfBytes(b))
			case string:
				msg.Set(fd, protoreflect.ValueOfBytes([]byte(b)))
			default:
				return nil, codec.ErrValue{Field: f.Name, Reason: "not raw bytes"}
			}
		case schema.U8, schema.U16, schema.U32:
			n, ok := asInt(raw)
			if !ok || n < 0 || n > 4294967295 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not an unsigned 32-bit number"}
			}
			msg.Set(fd, protoreflect.ValueOfUint32(uint32(n)))
		case schema.I16, schema.I32:
			n, ok := asInt(raw)
			if !ok || n < -2147483648 || n > 2147483647 {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a signed 32-bit number"}
			}
			msg.Set(fd, protoreflect.ValueOfInt32(int32(n)))
		case schema.Metres, schema.Deg:
			x, ok := asFloat(raw)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not a number"}
			}
			msg.Set(fd, protoreflect.ValueOfFloat64(x))
		case schema.Pos:
			p, ok := raw.(map[string]any)
			if !ok {
				return nil, codec.ErrValue{Field: f.Name, Reason: "not an object with x, y, z and yaw"}
			}
			sub := msg.Mutable(fd).Message()
			for _, axis := range []string{"x", "y", "z", "yaw"} {
				afd := sub.Descriptor().Fields().ByName(protoreflect.Name(axis))
				x, ok := asFloat(p[axis])
				if !ok {
					return nil, codec.ErrValue{Field: f.Name + "." + axis, Reason: "not a number"}
				}
				sub.Set(afd, protoreflect.ValueOfFloat64(x))
			}
		default:
			return nil, codec.ErrValue{Field: f.Name, Reason: "no protobuf type for " + string(f.Type)}
		}
	}
	return proto.Marshal(msg)
}

// Decode reads a protobuf frame back into the canonical shape. Every declared field is present in
// the result, because proto3 has no way to tell "absent" from "zero" for a scalar - saying so here
// is more honest than a decoder that pretends to know.
func (Proto) Decode(m *schema.Message, b []byte) (map[string]any, error) {
	md, err := descriptor(m)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(b, msg); err != nil {
		return nil, fmt.Errorf("%s: %w", m.Name, err)
	}
	// A frame carrying fields the declaration does not know is the same failure as trailing bytes
	// on the byte wire: the sender and the declaration disagree, and guessing would hide it.
	if unknown := msg.GetUnknown(); len(unknown) > 0 {
		return nil, codec.ErrTrailing{Bytes: len(unknown)}
	}
	out := make(map[string]any, len(m.Fields))
	for _, f := range m.Fields {
		fd := md.Fields().ByName(protoreflect.Name(f.Name))
		if fd == nil {
			return nil, codec.ErrValue{Field: f.Name, Reason: "not in the compiled descriptor"}
		}
		val := msg.Get(fd)
		switch f.Type {
		case schema.Bool:
			out[f.Name] = val.Bool()
		case schema.Str:
			out[f.Name] = val.String()
		case schema.Raw:
			out[f.Name] = append([]byte(nil), val.Bytes()...)
		case schema.U8, schema.U16, schema.U32:
			out[f.Name] = int64(val.Uint())
		case schema.I16, schema.I32:
			out[f.Name] = int64(val.Int())
		case schema.Metres, schema.Deg:
			out[f.Name] = val.Float()
		case schema.Pos:
			sub := val.Message()
			p := map[string]any{}
			for _, axis := range []string{"x", "y", "z", "yaw"} {
				p[axis] = sub.Get(sub.Descriptor().Fields().ByName(protoreflect.Name(axis))).Float()
			}
			out[f.Name] = p
		default:
			return nil, codec.ErrValue{Field: f.Name, Reason: "no protobuf type for " + string(f.Type)}
		}
	}
	return out, nil
}

// Names is the declaration's message names, sorted: used by the tests and by anything that wants to
// show what this codec can speak.
func Names(s *schema.Schema) []string {
	names := make([]string, 0, len(s.Messages))
	for _, m := range s.Messages {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}
