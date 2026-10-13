# GitHub metadata (paste into the repository settings)

**Description**
> One declaration, three wire formats: a byte layout, protobuf and FlatBuffers generated from the same YAML, with every codec held to a cross-format equivalence test and to golden bytes produced by a second, independent implementation.

**Topics**
go, codec, serialization, protobuf, flatbuffers, wire-format, binary-protocol, schema, testing, mutation-testing, game-server, library

**Pinned blurb for the profile README**
> `wire-codec` - declare a message once, pick the wire format later, and let a test prove the three formats agree; the byte layout is checked against an implementation that shares no code with it.
