//
// garbler.go
//
// Copyright (c) 2019-2025 Markku Rossi
//
// All rights reserved.
//

package circuit

import (
	"fmt"
	"math/big"

	"github.com/cockroachdb/errors"
	"github.com/markkurossi/mpc/env"
	"github.com/markkurossi/mpc/ot"
	"github.com/markkurossi/mpc/p2p"
)

// FileSize specifies a file (or data transfer) size in bytes.
type FileSize uint64

func (s FileSize) String() string {
	if s > 1000*1000*1000*1000 {
		return fmt.Sprintf("%dTB", s/(1000*1000*1000*1000))
	} else if s > 1000*1000*1000 {
		return fmt.Sprintf("%dGB", s/(1000*1000*1000))
	} else if s > 1000*1000 {
		return fmt.Sprintf("%dMB", s/(1000*1000))
	} else if s > 1000 {
		return fmt.Sprintf("%dkB", s/1000)
	} else {
		return fmt.Sprintf("%dB", s)
	}
}

// Garbler runs the garbler on the P2P network.
func Garbler(
	cfg *env.Config,
	conn *p2p.Conn,
	oti ot.OT,
	circ *Circuit,
	inputs *big.Int,
	verbose bool,
) (
	[]*big.Int, error,
) {
	rand := cfg.GetRandom()
	timing := NewTiming()
	if verbose {
		fmt.Printf(" - Garbling...\n")
	}

	var key [32]byte
	_, err := rand.Read(key[:])
	if err != nil {
		return nil, err
	}

	garbled, err := circ.Garble(rand, key[:])
	if err != nil {
		return nil, err
	}

	timing.Sample("Garble", nil)

	// Send program info.
	if verbose {
		fmt.Printf(" - Sending garbled circuit...\n")
	}
	if err := conn.DirectSend(key, "ephemeral key"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Garbler(...), when sending ephemeral key.")
		return nil, err
	}

	// Send garbled circuits.
	if err := conn.DirectSend(garbled.Gates, "garbled gates"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Garbler(...), when sending garbled gates.")
		return nil, err
	}

	// Select and send our inputs.
	var n1 []ot.Label
	for i := 0; i < int(circ.Inputs[0].Type.Bits); i++ {
		wire := garbled.Wires[i]
		n := LabelForBit(wire, inputs.Bit(i) == 1)
		n1 = append(n1, n)
	}
	if err := conn.DirectSend(n1, "inputs"); err != nil {
		err = errors.Wrap(err, "in mpc_hd::Garbler(...), when sending inputs.")
	}

	ioStats := conn.Stats.Sum()
	timing.Sample("Xfer", []string{FileSize(ioStats).String()})
	if verbose {
		fmt.Printf(" - Processing messages...\n")
	}

	// Init oblivious transfer.
	if err := oti.InitSender(conn); err != nil {
		return nil, err
	}
	xfer := conn.Stats.Sum() - ioStats
	ioStats = conn.Stats.Sum()
	timing.Sample("OT Init", []string{FileSize(xfer).String()})

	// Peer OTs its inputs.
	type OtQuery struct {
		Offset int
		Count  int
	}
	var query OtQuery
	if err := conn.DirectRecv(&query, "ot query"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Garbler(...), when receiving ot query")
		return nil, err
	}
	if query.Offset != int(circ.Inputs[0].Type.Bits) ||
		query.Count != int(circ.Inputs[1].Type.Bits) {
		return nil, fmt.Errorf("peer can't OT wires [%d..%d]",
			query.Offset, query.Offset+query.Count)
	}
	err = oti.Send(garbled.Wires[query.Offset : query.Offset+query.Count])
	if err != nil {
		return nil, err
	}
	xfer = conn.Stats.Sum() - ioStats
	ioStats = conn.Stats.Sum()
	timing.Sample("OT", []string{FileSize(xfer).String()})

	// Resolve result values.
	var labels []ot.Label
	if err := conn.DirectRecv(&labels, "ot labels"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Garbler(...), when receiving ot labels")
		return nil, err
	}
	timing.Sample("Eval", nil)

	result := big.NewInt(0)
	for i := 0; i < circ.Outputs.Size(); i++ {
		label := labels[i]
		wire := garbled.Wires[circ.NumWires-circ.Outputs.Size()+i]
		boolBit, err := BitFromLabel(wire, label)
		if err != nil {
			err = errors.Wrap(err,
				"in mpc_hd::Garbler(...), when extracting a bit from each label")
			return nil, err
		}
		if boolBit {
			result = big.NewInt(0).SetBit(result, i, 1)
		}
	}
	if err := conn.DirectSend(result, "result"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Garbler(...), when sending result")
		return nil, err
	}

	xfer = conn.Stats.Sum() - ioStats
	timing.Sample("Result", []string{FileSize(xfer).String()})
	if verbose {
		timing.Print(conn.Stats)
	}

	return circ.Outputs.Split(result), nil
}
