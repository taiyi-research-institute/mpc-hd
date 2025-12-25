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

	"github.com/markkurossi/mpc/ot"
)

var (
	_ ot.IO = &Conn{}
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

// SendByte sends a byte value.
// 多处使用.
func (c *Conn) SendByte(val byte) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendByte")
	}
	return nil
}

// SendUint16 sends an uint16 value.
func (c *Conn) SendUint16(val int) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendUint16")
	}
	return nil
}

// SendUint32 sends an uint32 value.
func (c *Conn) SendUint32(val int) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendUint32")
	}
	return nil
}

// SendData sends binary data.
func (c *Conn) SendData(val []byte) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendData")
	}
	return nil
}

// SendLabel sends an OT label.
func (c *Conn) SendLabel(val ot.Label, data *ot.LabelData) error {
	val.GetData(data)

	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendData")
	}
	return nil
}

// SendString sends a string value.
func (c *Conn) SendString(val string) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendString")
	}
	return nil
}

// SendInputSizes sends the input sizes.
func (c *Conn) SendInputSizes(val []int) error {
	conn := c.conn
	c.nsend += 1
	err := conn.DirectSend(val, conn.SessionId, "", c.je, c.tu, c.nsend)
	if err != nil {
		return errors.Wrapf(err, "SendString")
	}
	return nil
}

// ReceiveByte receives a byte value.
func (c *Conn) ReceiveByte() (byte, error) {
	conn := c.conn
	c.nrecv += 1
	var recv byte
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return recv, errors.Wrapf(err, "ReceiveByte")
	}
	return recv, nil
}

// ReceiveUint16 receives an uint16 value.
func (c *Conn) ReceiveUint16() (int, error) {
	conn := c.conn
	c.nrecv += 1
	var recv int
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return recv, errors.Wrapf(err, "ReceiveUint16")
	}
	return recv, nil
}

// ReceiveUint32 receives an uint32 value.
func (c *Conn) ReceiveUint32() (int, error) {
	conn := c.conn
	c.nrecv += 1
	var recv int
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return recv, errors.Wrapf(err, "ReceiveUint32")
	}
	return recv, nil
}

// ReceiveData receives binary data.
func (c *Conn) ReceiveData() ([]byte, error) {
	conn := c.conn
	c.nrecv += 1
	var recv []byte
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return recv, errors.Wrapf(err, "ReceiveData")
	}
	return recv, nil
}

// ReceiveLabel receives an OT label.
func (c *Conn) ReceiveLabel(recv *ot.Label, data *ot.LabelData) error {
	conn := c.conn
	c.nrecv += 1
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return errors.Wrapf(err, "ReceiveLabel")
	}
	recv.GetData(data)
	return nil
}

// ReceiveString receives a string value.
func (c *Conn) ReceiveString() (string, error) {
	conn := c.conn
	c.nrecv += 1
	var recv string
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return recv, errors.Wrapf(err, "ReceiveString")
	}
	return recv, nil
}

// ReceiveInputSizes receives input sizes.
func (c *Conn) ReceiveInputSizes() ([]int, error) {
	conn := c.conn
	c.nrecv += 1
	var recv []int
	err := conn.DirectRecv(&recv, conn.SessionId, "", c.tu, c.je, c.nrecv)
	if err != nil {
		return recv, errors.Wrapf(err, "ReceiveInputSizes")
	}
	return recv, nil
}

// Receive implements OT receive for the bit value of a wire.
func (c *Conn) Receive(receiver *ot.Receiver, wire, bit uint) ([]byte, error) {
	if err := c.SendUint32(int(wire)); err != nil {
		return nil, err
	}
	if err := c.Flush(); err != nil {
		return nil, err
	}

	xfer, err := receiver.NewTransfer(bit)
	if err != nil {
		return nil, err
	}

	x0, err := c.ReceiveData()
	if err != nil {
		return nil, err
	}
	x1, err := c.ReceiveData()
	if err != nil {
		return nil, err
	}
	err = xfer.ReceiveRandomMessages(x0, x1)
	if err != nil {
		return nil, err
	}

	v := xfer.V()
	if err := c.SendData(v); err != nil {
		return nil, err
	}
	if err := c.Flush(); err != nil {
		return nil, err
	}

	m0p, err := c.ReceiveData()
	if err != nil {
		return nil, err
	}
	m1p, err := c.ReceiveData()
	if err != nil {
		return nil, err
	}

	err = xfer.ReceiveMessages(m0p, m1p, nil)
	if err != nil {
		return nil, err
	}

	m, _ := xfer.Message()
	return m, nil
}
