//
// Copyright (c) 2019-2025 Markku Rossi
//
// All rights reserved.
//

// Package p2p implements point-to-point protocols for multi-party
// computation peers.
package p2p

import (
	"sync/atomic"

	"github.com/cockroachdb/errors"
	mgr "github.com/taiyi-research-institute/svarog-messenger/messenger"
)

// Conn implements a protocol connection.
type Conn struct {
	conn  *mgr.MessengerClient
	Stats IOStats
	je    int
	tu    int

	nsend int
	nrecv int
}

func (c *Conn) SessionId() string {
	return c.conn.SessionId
}

// IOStats implements I/O statistics.
type IOStats struct {
	Sent    *atomic.Uint64
	Recvd   *atomic.Uint64
	Flushed *atomic.Uint64
}

// NewIOStats creates a new I/O statistics object.
func NewIOStats() IOStats {
	return IOStats{
		Sent:    new(atomic.Uint64),
		Recvd:   new(atomic.Uint64),
		Flushed: new(atomic.Uint64),
	}
}

// Clear clears the I/O statistics.
func (stats IOStats) Clear() {
	stats.Sent.Store(0)
	stats.Recvd.Store(0)
	stats.Flushed.Store(0)
}

// Add adds the argument stats to this IOStats and returns the sum.
func (stats IOStats) Add(o IOStats) IOStats {
	sent := new(atomic.Uint64)
	sent.Store(stats.Sent.Load() + o.Sent.Load())

	recvd := new(atomic.Uint64)
	recvd.Store(stats.Recvd.Load() + o.Recvd.Load())

	flushed := new(atomic.Uint64)
	flushed.Store(stats.Flushed.Load() + o.Flushed.Load())

	return IOStats{
		Sent:    sent,
		Recvd:   recvd,
		Flushed: flushed,
	}
}

// Sum returns sum of sent and received bytes.
func (stats IOStats) Sum() uint64 {
	return stats.Sent.Load() + stats.Recvd.Load()
}

// NewConn creates a new connection around the argument connection.
func NewConn(isGarbler bool, host string, port uint16, sid string) (*Conn, error) {
	conn := new(mgr.MessengerClient)
	conn, err := conn.Connect(host, port)
	if err != nil {
		err = errors.Wrapf(err, "mpc-hd/NewConn : failed to connect to grpc server %s:%d", host, port)
		return nil, err
	}
	if sid == "" {
		sid, err = conn.GrpcNewSessionEasy()
	} else {
		conn.SessionId = sid
	}
	if err != nil {
		err = errors.Wrapf(err, "mpc-hd/NewConn : failed to set session_id %s w.r.t. grpc server %s:%d", sid, host, port)
		return nil, err
	}

	c := &Conn{
		conn:  conn,
		Stats: NewIOStats(),
		nsend: 0,
		nrecv: 0,
	}
	if isGarbler {
		c.je, c.tu = 1, 2
	} else {
		c.je, c.tu = 2, 1
	}

	return c, nil
}

// NeedSpace ensures the write buffer has space for count bytes. The
// function flushes the output if needed.
// 一处使用: stream_garble.go
func (c *Conn) NeedSpace(count int) error {
	return nil
}

// Flush flushed any pending data in the connection.
func (c *Conn) Flush() error {
	return nil
}

// Fill fills the input buffer from the connection. Any unused data in
// the buffer is moved to the beginning of the buffer.
func (c *Conn) Fill(n int) error {
	return nil
}

// Close flushes any pending data and closes the connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) RegisterSend(snd any, topic string) {
	conn := c.conn
	c.nsend += 1
	conn.RegisterSend(snd, conn.SessionId, topic, c.je, c.tu, c.nsend)
}

func (c *Conn) RegisterRecv(rcv any, topic string) {
	conn := c.conn
	c.nrecv += 1
	conn.RegisterRecv(rcv, conn.SessionId, topic, c.tu, c.je, c.nrecv)
}

func (c *Conn) Exchange() error {
	conn := c.conn
	if err := conn.Exchange(120, 240); err != nil {
		err = errors.Wrap(err, "in mpc_hd::Conn::Exchange(&self)")
	}
	return nil
}

func (c *Conn) DirectSend(snd any, topic string) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(snd, conn.SessionId, topic, c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "in mpc_hd::Conn::DirectSend(&self, any)")
	}
	return nil
}

func (c *Conn) DirectRecv(rcv any, topic string) error {
	conn := c.conn
	c.nrecv += 1
	err := conn.DirectRecv(rcv, conn.SessionId, topic, c.tu, c.je, c.nrecv)
	if err != nil {
		return errors.Wrapf(err, "in mpc_hd::Conn::DirectRecv(&self, any)")
	}
	return nil
}
