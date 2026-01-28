//
// Copyright (c) 2019-2025 Markku Rossi
//
// All rights reserved.
//

// Package p2p implements point-to-point protocols for multi-party
// computation peers.
package ot

import (
	"github.com/cockroachdb/errors"
)

// Conn implements a protocol connection.
type Conn struct {
	conn *MessengerClient
	je   int
	tu   int

	nsend int
	nrecv int
}

func (c *Conn) SessionId() string {
	return c.conn.SessionId
}

// NewConn creates a new connection around the argument connection.
func NewConn(isGarbler bool, hostport, sid string) (*Conn, error) {
	conn := new(MessengerClient)
	conn, err := conn.Connect(hostport)
	if err != nil {
		err = errors.Wrapf(err, "mpc-hd/NewConn : failed to connect to grpc server %s:%d", hostport)
		return nil, err
	}
	if sid == "" {
		sid, err = conn.GrpcNewSessionEasy()
	} else {
		conn.SessionId = sid
	}
	if err != nil {
		err = errors.Wrapf(err, "mpc-hd/NewConn : failed to set session_id %s w.r.t. grpc server %s", sid, hostport)
		return nil, err
	}

	c := &Conn{
		conn:  conn,
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
