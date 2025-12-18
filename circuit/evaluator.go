//
// evaluator.go
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
	"github.com/markkurossi/mpc/ot"
	"github.com/markkurossi/mpc/p2p"
)

// Evaluator runs the evaluator on the P2P network.
func Evaluator(
	conn *p2p.Conn,
	oti *ot.CO,
	circ *Circuit,
	inputs *big.Int,
	verbose bool,
) (
	[]*big.Int, error,
) {
	// E1. 接收临时密钥.
	if verbose {
		fmt.Printf(" - Waiting for circuit info...\n")
	}
	var key [32]byte
	if err := conn.DirectRecv(&key, "ephemeral key"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when receiving ephemeral key.")
		return nil, err
	}

	// E2. 接收 gates
	if verbose {
		fmt.Printf(" - Receiving garbled circuit...\n")
	}
	garbled := make([][]ot.Label, circ.NumGates)
	if err := conn.DirectRecv(&garbled, "garbled gates"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when receiving garbled gates.")
		return nil, err
	}

	// E3. 接收 inputs
	var wires []ot.Label
	if err := conn.DirectRecv(&wires, "inputs"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when receiving inputs.")
		return nil, err
	}
	padlen := circ.NumWires - len(wires)
	wires = append(wires, make([]ot.Label, padlen)...)

	// E4. 发送 offset 和 count
	if verbose {
		fmt.Printf(" - Querying our inputs...\n")
	}
	type OtQuery struct {
		Offset int
		Count  int
	}
	query := OtQuery{
		Offset: int(circ.Inputs[0].Type.Bits),
		Count:  int(circ.Inputs[1].Type.Bits),
	}
	if err := conn.DirectSend(&query, "ot query"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when sending ot query.")
		return nil, err
	}
	flags := make([]bool, int(circ.Inputs[1].Type.Bits))
	for i := 0; i < int(circ.Inputs[1].Type.Bits); i++ {
		if inputs.Bit(i) == 1 {
			flags[i] = true
		}
	}
	start := int(circ.Inputs[0].Type.Bits)
	end := start + int(circ.Inputs[1].Type.Bits)

	// E5. 执行 ot 接收. 位于 ot/co.go: func Receive
	if err := oti.Receive(flags, wires[start:end], conn); err != nil {
		return nil, err
	}

	// Evaluate gates.
	if verbose {
		fmt.Printf(" - Evaluating circuit...\n")
	}
	if err := circ.Eval(key[:], wires, garbled); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when evaluating gates.")
		return nil, err
	}

	// E6. 发送结果 labels
	var labels []ot.Label
	for i := 0; i < circ.Outputs.Size(); i++ {
		r := wires[Wire(circ.NumWires-circ.Outputs.Size()+i)]
		labels = append(labels, r)
	}
	if err := conn.DirectSend(&labels, "result labels"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when sending ot labels.")
		return nil, err
	}

	// E7. 接收结果.
	var result big.Int
	if err := conn.DirectRecv(&result, "result"); err != nil {
		err = errors.Wrap(err,
			"in mpc_hd::Evaluator(...), when receiving result.")
		return nil, err
	}
	return circ.Outputs.Split(&result), nil
}
