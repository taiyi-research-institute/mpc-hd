package ot

// Message and session types shared between the messenger client and server.
//
// Plain Go structs serialized over the HTTP transport with CBOR
// (`fxamacker/cbor`). The Rust `svarog-messenger` crate encodes the same shapes
// with `ciborium`, so the `cbor` tags below use the Rust crate's (lower-case)
// field names and `Val` maps to a CBOR byte string, matching Rust's
// `#[serde(with = "serde_bytes")] val`.

type SessionConfig struct {
	Operation       string          `cbor:"operation"`
	SesmanUrl       string          `cbor:"sesman_url"`
	SessionId       string          `cbor:"session_id"`
	Threshold       uint64          `cbor:"threshold"`
	Players         map[string]bool `cbor:"players"`
	PlayersReshared map[string]bool `cbor:"players_reshared"`
	Logged          bool            `cbor:"logged"`
}

type SessionId struct {
	Value string `cbor:"value"`
}

type Message struct {
	Sid   string `cbor:"sid"`
	Topic string `cbor:"topic"`
	Src   uint64 `cbor:"src"`
	Dst   uint64 `cbor:"dst"`
	Seq   uint64 `cbor:"seq"`
	Val   []byte `cbor:"val"`
}

type VecMessage struct {
	Values []*Message `cbor:"values"`
}

type EchoMessage struct {
	Value string `cbor:"value"`
}

type Void struct{}
