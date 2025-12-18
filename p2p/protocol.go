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

	mgr "github.com/taiyi-research-institute/svarog-messenger/messenger"

	"github.com/markkurossi/mpc/ot"
)

var (
	_ ot.IO = &Conn{}
)

const (
	numBuffers   = 3
	writeBufSize = 64 * 1024
	readBufSize  = 1024 * 1024
)

// Conn implements a protocol connection.
type Conn struct {
	conn  *mgr.MessengerClient
	Stats IOStats

	nread  uint64
	nwrite uint64
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
func NewConn(conn *mgr.MessengerClient) *Conn {
	c := &Conn{
		conn:   conn,
		Stats:  NewIOStats(),
		nread:  0,
		nwrite: 0,
	}

	return c
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
// 多处使用.
func (c *Conn) Close() error {
	return nil
}

// SendByte sends a byte value.
// 多处使用.
func (c *Conn) SendByte(val byte) error {
	if c.WritePos+1 > len(c.WriteBuf) {
		if err := c.Flush(); err != nil {
			return err
		}
	}
	c.WriteBuf[c.WritePos] = val
	c.WritePos++
	return nil
}

// SendUint16 sends an uint16 value.
func (c *Conn) SendUint16(val int) error {
	if c.WritePos+2 > len(c.WriteBuf) {
		if err := c.Flush(); err != nil {
			return err
		}
	}
	c.WriteBuf[c.WritePos+0] = byte((uint32(val) >> 8) & 0xff)
	c.WriteBuf[c.WritePos+1] = byte(uint32(val) & 0xff)
	c.WritePos += 2
	return nil
}

// SendUint32 sends an uint32 value.
func (c *Conn) SendUint32(val int) error {
	if c.WritePos+4 > len(c.WriteBuf) {
		if err := c.Flush(); err != nil {
			return err
		}
	}
	c.WriteBuf[c.WritePos+0] = byte((uint32(val) >> 24) & 0xff)
	c.WriteBuf[c.WritePos+1] = byte((uint32(val) >> 16) & 0xff)
	c.WriteBuf[c.WritePos+2] = byte((uint32(val) >> 8) & 0xff)
	c.WriteBuf[c.WritePos+3] = byte(uint32(val) & 0xff)
	c.WritePos += 4
	return nil
}

// SendData sends binary data.
func (c *Conn) SendData(val []byte) error {
	err := c.SendUint32(len(val))
	if err != nil {
		return err
	}
	for len(val) > 0 {
		if c.WritePos >= len(c.WriteBuf) {
			if err := c.Flush(); err != nil {
				return err
			}
		}
		n := copy(c.WriteBuf[c.WritePos:], val)
		c.WritePos += n
		val = val[n:]
	}
	return nil
}

// SendLabel sends an OT label.
func (c *Conn) SendLabel(val ot.Label, data *ot.LabelData) error {
	bytes := val.Bytes(data)
	if c.WritePos+len(bytes) > len(c.WriteBuf) {
		if err := c.Flush(); err != nil {
			return err
		}
	}
	copy(c.WriteBuf[c.WritePos:], bytes)
	c.WritePos += len(bytes)

	return nil
}

// SendString sends a string value.
func (c *Conn) SendString(val string) error {
	return c.SendData([]byte(val))
}

// SendInputSizes sends the input sizes.
func (c *Conn) SendInputSizes(sizes []int) error {
	if err := c.SendUint32(len(sizes)); err != nil {
		return err
	}
	for i := 0; i < len(sizes); i++ {
		if err := c.SendUint32(sizes[i]); err != nil {
			return err
		}
	}
	return nil
}

// ReceiveByte receives a byte value.
func (c *Conn) ReceiveByte() (byte, error) {
	if c.ReadStart+1 > c.ReadEnd {
		if err := c.Fill(1); err != nil {
			return 0, err
		}
	}
	val := c.ReadBuf[c.ReadStart]
	c.ReadStart++
	return val, nil
}

// ReceiveUint16 receives an uint16 value.
func (c *Conn) ReceiveUint16() (int, error) {
	if c.ReadStart+2 > c.ReadEnd {
		if err := c.Fill(2); err != nil {
			return 0, err
		}
	}
	val := uint32(c.ReadBuf[c.ReadStart+0])
	val <<= 8
	val |= uint32(c.ReadBuf[c.ReadStart+1])
	c.ReadStart += 2

	return int(val), nil
}

// ReceiveUint32 receives an uint32 value.
func (c *Conn) ReceiveUint32() (int, error) {
	if c.ReadStart+4 > c.ReadEnd {
		if err := c.Fill(4); err != nil {
			return 0, err
		}
	}
	val := uint32(c.ReadBuf[c.ReadStart+0])
	val <<= 8
	val |= uint32(c.ReadBuf[c.ReadStart+1])
	val <<= 8
	val |= uint32(c.ReadBuf[c.ReadStart+2])
	val <<= 8
	val |= uint32(c.ReadBuf[c.ReadStart+3])
	c.ReadStart += 4

	return int(val), nil
}

// ReceiveData receives binary data.
func (c *Conn) ReceiveData() ([]byte, error) {
	len, err := c.ReceiveUint32()
	if err != nil {
		return nil, err
	}
	result := make([]byte, len)

	var read int

	for read < len {
		if c.ReadStart >= c.ReadEnd {
			need := len - read
			if need > readBufSize {
				need = readBufSize
			}
			if err := c.Fill(need); err != nil {
				return nil, err
			}
		}
		need := len - read
		avail := c.ReadEnd - c.ReadStart

		if avail > need {
			avail = need
		}
		n := copy(result[read:], c.ReadBuf[c.ReadStart:c.ReadStart+avail])
		c.ReadStart += n
		read += n
	}

	return result, nil
}

// ReceiveLabel receives an OT label.
func (c *Conn) ReceiveLabel(val *ot.Label, data *ot.LabelData) error {
	if c.ReadStart+len(data) > c.ReadEnd {
		if err := c.Fill(len(data)); err != nil {
			return err
		}
	}
	copy(data[:], c.ReadBuf[c.ReadStart:c.ReadStart+len(data)])
	c.ReadStart += len(data)

	val.SetData(data)
	return nil
}

// ReceiveString receives a string value.
func (c *Conn) ReceiveString() (string, error) {
	data, err := c.ReceiveData()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReceiveInputSizes receives input sizes.
func (c *Conn) ReceiveInputSizes() ([]int, error) {
	count, err := c.ReceiveUint32()
	if err != nil {
		return nil, err
	}
	result := make([]int, count)
	for i := 0; i < count; i++ {
		size, err := c.ReceiveUint32()
		if err != nil {
			return nil, err
		}
		result[i] = size
	}
	return result, nil
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
