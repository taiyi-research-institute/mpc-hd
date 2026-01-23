//
// co.go
//
// Copyright (c) 2019-2025 Markku Rossi
//
// All rights reserved.
//
// Chou Orlandi OT - The Simplest Protocol for Oblivious Transfer.
//  - https://eprint.iacr.org/2015/267.pdf

/*

This implementation is derived from the EMP Toolkit's co.h
(https://github.com/emp-toolkit/emp-ot/blob/master/emp-ot/co.h)
with original license as follows:

MIT License

Copyright (c) 2018 Xiao Wang (wangxiao1254@gmail.com)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

Enquiries about further applications and development opportunities are welcome.

*/

package ot

import (
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"math/big"

	"github.com/cockroachdb/errors"
)

// COSender implements CO OT sender.
type COSender struct {
	rand  io.Reader
	curve elliptic.Curve
}

// NewCOSender creates a new CO OT sender.
func NewCOSender(rand io.Reader) *COSender {
	return &COSender{
		rand:  rand,
		curve: elliptic.P256(),
	}
}

// Curve returns sender's elliptic curve.
func (s *COSender) Curve() elliptic.Curve {
	return s.curve
}

// NewTransfer creates a new OT transfer for the values.
func (s *COSender) NewTransfer(m0, m1 []byte) (*COSenderXfer, error) {
	curveParams := s.curve.Params()

	// a <- Zp
	a, err := rand.Int(s.rand, curveParams.N)
	if err != nil {
		return nil, err
	}

	// A = G^a
	Ax, Ay := s.curve.ScalarBaseMult(a.Bytes())

	// Aa = A^a
	Aax, Aay := s.curve.ScalarMult(Ax, Ay, a.Bytes())

	// a:    {x,y}
	// a^-1: {x,-y}
	// AaInv = {Aax, -Aay}
	AaInvx := big.NewInt(0).Set(Aax)
	AaInvy := big.NewInt(0).Sub(curveParams.P, Aay)

	return &COSenderXfer{
		curve:  s.curve,
		m0:     m0,
		m1:     m1,
		a:      a,
		Ax:     Ax,
		Ay:     Ay,
		AaInvx: AaInvx,
		AaInvy: AaInvy,
	}, nil
}

// COSenderXfer implements sender OT transfer.
type COSenderXfer struct {
	curve  elliptic.Curve
	m0     []byte
	m1     []byte
	a      *big.Int
	Ax     *big.Int
	Ay     *big.Int
	AaInvx *big.Int
	AaInvy *big.Int
	e0     []byte
	e1     []byte
}

// A returns sender's random value.
func (s *COSenderXfer) A() (x, y []byte) {
	return s.Ax.Bytes(), s.Ay.Bytes()
}

// ReceiveB receives receiver's selection.
func (s *COSenderXfer) ReceiveB(x, y []byte) {
	bx := big.NewInt(0).SetBytes(x)
	by := big.NewInt(0).SetBytes(y)

	bx, by = s.curve.ScalarMult(bx, by, s.a.Bytes())
	bax, bay := s.curve.Add(bx, by, s.AaInvx, s.AaInvy)

	mask0 := deriveMask(bx, by, 0)
	mask1 := deriveMask(bax, bay, 0)
	s.e0 = append([]byte(nil), xor(mask0[:], s.m0)...)
	s.e1 = append([]byte(nil), xor(mask1[:], s.m1)...)
}

// E returns sender's encrypted messages.
func (s *COSenderXfer) E() (e0, e1 []byte) {
	return s.e0, s.e1
}

// COReceiver implements CO OT receiver.
type COReceiver struct {
	rand  io.Reader
	curve elliptic.Curve
}

// NewCOReceiver creates a new OT receiver.
func NewCOReceiver(rand io.Reader, curve elliptic.Curve) *COReceiver {
	return &COReceiver{
		rand:  rand,
		curve: curve,
	}
}

// NewTransfer creates a new OT transfer for the selection bit.
func (r *COReceiver) NewTransfer(bit uint) (*COReceiverXfer, error) {
	curveParams := r.curve.Params()

	// b <= Zp
	b, err := rand.Int(r.rand, curveParams.N)
	if err != nil {
		return nil, err
	}

	return &COReceiverXfer{
		curve: r.curve,
		bit:   bit,
		b:     b,
	}, nil
}

// COReceiverXfer implements receiver OT transfer.
type COReceiverXfer struct {
	curve elliptic.Curve
	bit   uint
	b     *big.Int
	Bx    *big.Int
	By    *big.Int
	Asx   *big.Int
	Asy   *big.Int
}

// ReceiveA receives sender's random value.
func (r *COReceiverXfer) ReceiveA(x, y []byte) {
	Ax := big.NewInt(0).SetBytes(x)
	Ay := big.NewInt(0).SetBytes(y)

	Bx, By := r.curve.ScalarBaseMult(r.b.Bytes())
	if r.bit != 0 {
		Bx, By = r.curve.Add(Bx, By, Ax, Ay)
	}
	r.Bx = Bx
	r.By = By

	Asx, Asy := r.curve.ScalarMult(Ax, Ay, r.b.Bytes())
	r.Asx = Asx
	r.Asy = Asy
}

// B returns receiver's selection.
func (r *COReceiverXfer) B() (x, y []byte) {
	return r.Bx.Bytes(), r.By.Bytes()
}

// ReceiveE receives encrypted messages from the sender and returns
// the result value.
func (r *COReceiverXfer) ReceiveE(e0, e1 []byte) []byte {
	mask := deriveMask(r.Asx, r.Asy, 0)

	if r.bit != 0 {
		return append([]byte(nil), xor(mask[:], e1)...)
	}
	return append([]byte(nil), xor(mask[:], e0)...)
}

// CO implements CO OT as the OT interface.
type CO struct {
	rand  io.Reader
	curve elliptic.Curve
	io    IO
}

// NewCO creates a new CO OT implementing the OT interface.
func NewCO(rand io.Reader) *CO {
	return &CO{
		rand:  rand,
		curve: elliptic.P256(),
	}
}

// Send sends the wire labels with OT.
func (co *CO) Send(wires []Wire, conn *Conn) error {
	setup, err := GenerateCOSenderSetup(co.rand, co.curve)
	if err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Send(...), when generating sender setup.")
		return err
	}

	snd := &ECPoint{X: setup.Ax, Y: setup.Ay}
	if err := conn.DirectSend(snd, "setup.Ax,Ay"); err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Send(...), when sending setup.Ax,Ay")
		return err
	}

	var points []ECPoint
	if err := conn.DirectRecv(&points, "co choices"); err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Send(...), when receiving co choices")
		return err
	}

	ct, err := EncryptCOCiphertexts(co.curve, setup, points, wires)
	if err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Send(...), when making ot ciphertexts")
		return err
	}
	if err := conn.DirectSend(&ct, "ot ciphertexts"); err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Send(...), when receiving ot ciphertext")
		return err
	}

	return nil
}

// Receive receives the wire labels with OT based on the flag values.
func (co *CO) Receive(flags []bool, result []Label, conn *Conn) error {
	rcv := &ECPoint{}
	if err := conn.DirectRecv(rcv, "setup.Ax,Ay"); err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Receive(...), when receiving setup.Ax,Ay")
		return err
	}

	bundle, points, err := BuildCOChoices(co.rand, co.curve, rcv.X, rcv.Y, flags)
	if err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Receive(...), when building co choices")
		return err
	}
	if err := conn.DirectSend(points, "co choices"); err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Receive(...), when sending co choices")
		return err
	}

	var ciphertexts []LabelCiphertext
	if err := conn.DirectRecv(&ciphertexts, "ot ciphertexts"); err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Receive(...), when receiving ot ciphertexts")
		return err
	}

	labels, err := DecryptCOCiphertexts(co.curve, bundle, ciphertexts)
	if err != nil {
		err = errors.Wrap(err,
			"in func (co *CO) Receive(...), when decrypting ot ciphertexts")
		return err
	}
	if len(labels) != len(result) {
		err := errors.Newf("label count mismatch: got %d want %d", len(labels), len(result))
		return err
	}
	copy(result, labels)
	return nil
}
