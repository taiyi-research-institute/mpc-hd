//
// Copyright (c) 2025 Markku Rossi
//
// All rights reserved.
//

package p2p

import (
	"io"

	"github.com/cockroachdb/errors"
)

// Pipe implements the Conn interface as a bidirectional communication
// pipe. Anything send to the first endpoint can be received from the
// second and vice versa.
func Pipe() (*Conn, *Conn, error) {
	host, port := "127.0.0.1", uint16(65534)
	c1, err := NewConn(host, port, "")
	if err != nil {
		err = errors.Wrap(err, "Pipe().c1")
		return nil, nil, err
	}
	c2, err := NewConn(host, port, c1.SessionId())
	if err != nil {
		err = errors.Wrap(err, "Pipe().c2")
		return nil, nil, err
	}
	return c1, c2, nil
}

type pipe struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (p *pipe) Close() error {
	if err := p.r.Close(); err != nil {
		return err
	}
	return p.w.Close()
}

func (p *pipe) Read(data []byte) (n int, err error) {
	return p.r.Read(data)
}

func (p *pipe) Write(data []byte) (n int, err error) {
	return p.w.Write(data)
}
