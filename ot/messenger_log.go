package ot

import (
	"log"
	"time"
)

// logSessionCreated / logSessionUsed mirror the Rust crate's
// `log_session_created` / `log_session_used` side-effects.

func logSessionCreated(cfg *SessionConfig) {
	now := time.Now().Format(time.RFC3339)
	switch cfg.Operation {
	case "keygen":
		log.Printf("session created: keygen-%s\ncreate time: %s\n", cfg.SessionId, now)
	case "sign":
		log.Printf("session created: sign-%s\ncreate time: %s\n", cfg.SessionId, now)
	case "reshare":
		log.Printf("session created: reshare-%s\ncreate time: %s\n", cfg.SessionId, now)
	}
}

func logSessionUsed(cfg *SessionConfig) {
	now := time.Now().Format(time.RFC3339)
	switch cfg.Operation {
	case "keygen":
		log.Printf("keygen: %s\ntime: %s\nthreshold: %d\nparticipants: %v\n",
			cfg.SessionId, now, cfg.Threshold, cfg.Players)
	case "sign":
		log.Printf("sign: %s\ntime: %s\nsigners: %v\n",
			cfg.SessionId, now, cfg.Players)
	case "reshare":
		log.Printf("reshare: %s\ntime: %s\nold keyshare holders: %v\nnew keyshare recipients: %v\n",
			cfg.SessionId, now, cfg.Players, cfg.PlayersReshared)
	}
}
