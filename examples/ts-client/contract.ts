// The client side of the contract, as a client team would write it: hand-written types that mirror
// the declaration, compiled against the generated descriptors in ../contract/.
//
// This file is NOT a client implementation, and it is not a wire encoder. It is what a server
// engineer owes a client engineer: the names, the types, and the two rules that cannot be left
// implicit. The Go suite checks this file mentions every declared message and field (see
// internal/contract: TestTypescriptMirrorsTheDeclaration), so it cannot quietly describe last
// quarter's contract.
//
// Compile the descriptors:
//   protoc --ts_out=. ../contract/proto/Move.proto        # protobuf-ts, or any protobuf runtime
//   flatc --ts ../contract/fbs/Move.fbs                   # FlatBuffers, if the client reads that
//
// One opcode per message, from the declaration in schema/ops.yaml.

export const Opcode = {
  Move: 1,
  SetColor: 2,
  Ready: 6,
  Say: 9,
  Everything: 20,
} as const;

/** The quantisation every codec agrees on: 1 cm, and 1/65535 of a turn. */
export const Resolution = {
  centimetre: 0.01,
  degree: 360 / 65535,
} as const;

/** The position: metres plus yaw in degrees, quantised by the transport. */
export interface Pos {
  x: number;
  y: number;
  z: number;
  yaw: number;
}

/** Move (opcode 1). The hot message: 8 bytes on the declared layout, 29 as protobuf. */
export interface Move {
  pos: Pos;
}

/** SetColor (opcode 2): three channels, 0-255. */
export interface SetColor {
  r: number;
  g: number;
  b: number;
}

/** Ready (opcode 6): the handshake, before anything else is sent. */
export interface Ready {
  ready: boolean;
}

/** Say (opcode 9): UTF-8 text, length-prefixed on the wire. */
export interface Say {
  text: string;
}

/**
 * Everything (opcode 20): every declared type in one message. A client uses it to test its own
 * codec against the server's, in one round trip, before it ships.
 */
export interface Everything {
  a_u8: number;
  a_i16: number;
  a_u16: number;
  a_i32: number;
  a_u32: number;
  a_bool: boolean;
  a_metres: number;
  a_deg: number;
  a_str: string;
  /** base64 on the JSON side; bytes on the wire. */
  a_raw: string;
}

// Two rules the client cannot infer from the types, which is why they are written down here:
//
// 1. A values that does not fit is an error, not a clamp. 400 metres is refused, not sent as 327.67.
// 2. The server's decoder returns a plausible value for a broken frame never. If your client sees
//    a default-looking position where it sent a real one, the frame was short and the error was
//    swallowed somewhere in your own stack - not here.
