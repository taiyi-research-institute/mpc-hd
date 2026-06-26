package ot

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

const SESSION_TIMEOUT = time.Second * 60

// MAX_MESSAGE_SIZE bounds the request body size, mirroring the Rust crate's
// `MAX_MESSAGE_SIZE` (128 MiB).
const MAX_MESSAGE_SIZE = 128 * 1024 * 1024

// SpawnServer starts the messenger HTTP server and blocks forever.
//
// Routes mirror the Rust axum server:
//
//	POST /new_session         body=SessionConfig -> SessionId
//	POST /get_session_config  body=SessionId     -> SessionConfig (404 if absent)
//	POST /inbox               body=VecMessage    -> Void
//	POST /outbox              body=VecMessage    -> VecMessage (long-poll)
//	GET  /ping                                   -> EchoMessage
func SpawnServer(host string, port uint16) {
	addr := fmt.Sprintf("%s:%d", host, port)
	srv := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /new_session", srv.handleNewSession)
	mux.HandleFunc("POST /get_session_config", srv.handleGetSessionConfig)
	mux.HandleFunc("POST /inbox", srv.handleInbox)
	mux.HandleFunc("POST /outbox", srv.handleOutbox)
	mux.HandleFunc("GET /ping", srv.handlePing)

	httpServer := &http.Server{Addr: addr, Handler: mux}
	if err := httpServer.ListenAndServe(); err != nil {
		panic(err)
	}
}

type MessengerServer struct {
	db *cache.Cache
}

func NewServer() *MessengerServer {
	s := &MessengerServer{}
	s.db = cache.New(SESSION_TIMEOUT, 180*time.Second)
	return s
}

// wireEncode/wireDecode serialize the HTTP request and response bodies with
// CBOR, matching the Rust crate's ciborium envelope so the Go garbled client
// and the Rust messenger server interoperate.
func wireEncode(w io.Writer, v any) error {
	return cbor.NewEncoder(w).Encode(v)
}

func wireDecode(r io.Reader, v any) error {
	return cbor.NewDecoder(r).Decode(v)
}

func writeWire(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/octet-stream")
	_ = wireEncode(w, v)
}

// sessionConfigKey derives the cache key for a session config, matching the
// Rust convention `primary_key(sid, "session config", 0, 0, 0)`.
func sessionConfigKey(sid string) string {
	return PrimaryKey(sid, "session config", 0, 0, 0)
}

func (s *MessengerServer) handleNewSession(w http.ResponseWriter, r *http.Request) {
	var cfg SessionConfig
	if err := wireDecode(http.MaxBytesReader(w, r.Body, MAX_MESSAGE_SIZE), &cfg); err != nil {
		http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
		return
	}

	// If SessionId is not provided, create one with UUID-v7, in lowercase hex
	// string WITHOUT hyphens.
	if cfg.SessionId == "" {
		uuidv7, _ := uuid.NewV7()
		buf, _ := uuidv7.MarshalBinary()
		cfg.SessionId = hex.EncodeToString(buf)
	}

	key := sessionConfigKey(cfg.SessionId)
	s.db.Set(key, &cfg, cache.DefaultExpiration)

	logSessionCreated(&cfg)

	writeWire(w, &SessionId{Value: cfg.SessionId})
}

func (s *MessengerServer) handleGetSessionConfig(w http.ResponseWriter, r *http.Request) {
	var req SessionId
	if err := wireDecode(http.MaxBytesReader(w, r.Body, MAX_MESSAGE_SIZE), &req); err != nil {
		http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
		return
	}

	key := sessionConfigKey(req.Value)
	obj, found := s.db.Get(key)
	if !found {
		http.Error(w, fmt.Sprintf("Session does not exist: %s", req.Value), http.StatusNotFound)
		return
	}
	cfg := obj.(*SessionConfig)

	if !cfg.Logged {
		// Persist the one-time-logging marker, but keep it out of the value
		// returned to the client (`Logged` is purely server-side bookkeeping).
		stored := *cfg
		stored.Logged = true
		s.db.Set(key, &stored, cache.DefaultExpiration)
		logSessionUsed(cfg)
	}

	writeWire(w, cfg)
}

func (s *MessengerServer) handleInbox(w http.ResponseWriter, r *http.Request) {
	var vec VecMessage
	if err := wireDecode(http.MaxBytesReader(w, r.Body, MAX_MESSAGE_SIZE), &vec); err != nil {
		http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
		return
	}

	for _, msg := range vec.Values {
		if msg.Val == nil {
			http.Error(w, "inbox refuses val==nil", http.StatusBadRequest)
			return
		}
		key := PrimaryKey(msg.Sid, msg.Topic, msg.Src, msg.Dst, msg.Seq)
		s.db.Set(key, msg.Val, cache.DefaultExpiration)
	}

	writeWire(w, &Void{})
}

func (s *MessengerServer) handleOutbox(w http.ResponseWriter, r *http.Request) {
	var vec VecMessage
	if err := wireDecode(http.MaxBytesReader(w, r.Body, MAX_MESSAGE_SIZE), &vec); err != nil {
		http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	resp := &VecMessage{Values: make([]*Message, len(vec.Values))}
	for i, idx := range vec.Values {
		key := PrimaryKey(idx.Sid, idx.Topic, idx.Src, idx.Dst, idx.Seq)
		var val []byte
		for {
			if obj, found := s.db.Get(key); found {
				val = obj.([]byte)
				break
			}
			// Long-poll: wait for the value to arrive, aborting if the client
			// disconnects.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		resp.Values[i] = &Message{
			Sid:   idx.Sid,
			Topic: idx.Topic,
			Src:   idx.Src,
			Dst:   idx.Dst,
			Seq:   idx.Seq,
			Val:   val,
		}
	}

	writeWire(w, resp)
}

func (s *MessengerServer) handlePing(w http.ResponseWriter, r *http.Request) {
	writeWire(w, &EchoMessage{Value: "Svarog Messenger server is running."})
}
