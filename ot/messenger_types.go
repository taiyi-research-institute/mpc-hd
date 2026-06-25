package ot

// Message and session types shared between the messenger client and server.
//
// Plain Go structs serialized over the HTTP transport (gob envelope). This
// mirrors the Rust `svarog-messenger` crate, which uses plain serde structs
// over an axum/HTTP transport (bincode envelope) after dropping protobuf/gRPC.

type SessionConfig struct {
	Operation       string
	SesmanUrl       string
	SessionId       string
	Threshold       uint64
	Players         map[string]bool
	PlayersReshared map[string]bool
	Logged          bool
}

type SessionId struct {
	Value string
}

type Message struct {
	Sid   string
	Topic string
	Src   uint64
	Dst   uint64
	Seq   uint64
	Val   []byte
}

type VecMessage struct {
	Values []*Message
}

type EchoMessage struct {
	Value string
}

type Void struct{}
