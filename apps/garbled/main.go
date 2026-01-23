//
// main.go
//
// Copyright (c) 2019-2025 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/markkurossi/mpc/circuit"
	"github.com/markkurossi/mpc/compiler"
	"github.com/markkurossi/mpc/compiler/utils"
	"github.com/markkurossi/mpc/ot"
)

var (
	host = "127.0.0.1"
	port = uint16(65534)
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return
	}
	err = os.Setenv("MPCLDIR", cwd)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return
	}
	log.Printf("$MPCLDIR has been set to %s.\r\n", cwd)

	var args InputArguments
	flag.Var(&args, "i", "comma-separated list of circuit inputs")
	evaluator := flag.Bool("e", false, "evaluator / garbler mode")
	flag.Parse()

	file := "pkg/bip32_derive_tweak_ec.mpcl"

	var buf []byte
	if *evaluator {
		buf, err = evaluator_fn("dummy_session_id", args, file, nil, true)
		log.Println("evaluator result:", hex.EncodeToString(buf))
	} else {
		buf, err = garbler_fn("dummy_session_id", args, file, nil, true)
		log.Println("garbler result:", hex.EncodeToString(buf))
	}
	if err != nil {
		log.Fatal(err)
	}
	buf_gt, _ := hex.DecodeString("5c3de1895a724508483c65e3c08ad623db8e319b59294f5a170e521c0cb62980cb6729d2d51cbb17247997ca59584c20356f9cb39ac6ae7c82a5a0671b3f3934")
	if slices.Compare(buf, buf_gt) == 0 {
		log.Println("\033[1;34m Congratulations !!! \033[0m")
	} else {
		log.Fatal("\033[1;34m Wrong implementation !!! \033[0m")
	}
}

func evaluator_fn(
	sid string,
	args []string,
	file string,
	deps []string,
	verbose bool,
) ([]byte, error) {
	params := utils.NewParams()
	params.Verbose = false
	params.PkgPath = deps
	params.OptPruneGates = true
	defer params.Close()

	conn, err := ot.NewConn(false, host, port, sid)
	if err != nil {
		return nil, errors.Wrap(err, "in evaluator_fn()")
	}
	defer conn.Close()

	oti := ot.NewCO(params.Config.GetRandom())

	inputSizes := make([][]int, 2)
	myInputSizes, err := circuit.InputSizes(args)
	if err != nil {
		return nil, errors.Wrap(err, "in evaluator_fn()")
	}
	inputSizes[1] = myInputSizes
	err = conn.DirectSend(myInputSizes, "input sizes")
	if err != nil {
		return nil, errors.Wrap(err, "in evaluator_fn()")
	}

	var peerInputSizes []int
	err = conn.DirectRecv(&peerInputSizes, "input sizes")
	if err != nil {
		return nil, errors.Wrap(err, "in evaluator_fn()")
	}
	log.Println("evaluator exchanged input sizes")
	inputSizes[0] = peerInputSizes

	var circ *circuit.Circuit
	var oPeerInputSizes []int
	if slices.Compare(peerInputSizes, oPeerInputSizes) != 0 {
		circ, err = loadCircuit(file, params, inputSizes)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "in evaluator_fn()")
		}
		oPeerInputSizes = peerInputSizes
	}
	circ.PrintInputs(circuit.IDEvaluator, args)
	if len(circ.Inputs) != 2 {
		return nil, errors.Newf(
			"invalid circuit for 2-party MPC: %d parties",
			len(circ.Inputs))
	}

	input, err := circ.Inputs[1].Parse(args)
	if err != nil {
		conn.Close()
		return nil, errors.Wrapf(err, "in evaluator_fn(), filepath=%s", file)
	}
	result, err := circuit.Evaluator(conn, oti, circ, input, verbose)
	conn.Close()
	if err != nil && err != io.EOF {
		return nil, errors.Wrapf(err, "in evaluator_fn(), filepath=%s", file)
	}
	val := getResult(result, circ.Outputs)
	return val, nil
}

func garbler_fn(
	session_id string,
	args []string,
	file string,
	deps []string,
	verbose bool,
) ([]byte, error) {
	params := utils.NewParams()
	params.Verbose = false
	params.PkgPath = deps
	params.OptPruneGates = true
	defer params.Close()

	conn, err := ot.NewConn(true, host, port, session_id)
	if err != nil {
		return nil, errors.Wrap(err, "in garbler_fn()")
	}
	defer conn.Close()

	oti := ot.NewCO(params.Config.GetRandom())

	inputSizes := make([][]int, 2)
	myInputSizes, err := circuit.InputSizes(args)
	if err != nil {
		return nil, errors.Wrap(err, "in garbler_fn()")
	}
	inputSizes[0] = myInputSizes
	err = conn.DirectSend(myInputSizes, "input sizes")
	if err != nil {
		return nil, errors.Wrap(err, "in evaluator_fn()")
	}

	var peerInputSizes []int
	err = conn.DirectRecv(&peerInputSizes, "input sizes")
	if err != nil {
		return nil, errors.Wrap(err, "in evaluator_fn()")
	}
	log.Println("evaluator exchanged input sizes")
	inputSizes[0] = peerInputSizes

	circ, err := loadCircuit(file, params, inputSizes)
	if err != nil {
		return nil, errors.Wrap(err, "in garbler_fn()")
	}
	circ.PrintInputs(circuit.IDGarbler, args)
	if len(circ.Inputs) != 2 {
		return nil, errors.Newf(
			"invalid circuit for 2-party MPC: %d parties",
			len(circ.Inputs))
	}

	input, err := circ.Inputs[0].Parse(args)
	if err != nil {
		return nil, errors.Wrapf(err, "in garbler_fn(), filepath=%s", file)
	}
	result, err := circuit.Garbler(params.Config, conn, oti, circ, input,
		verbose)
	if err != nil {
		return nil, errors.Wrap(err, "in garbler_fn()")
	}
	val := getResult(result, circ.Outputs)
	return val, nil
}

func loadCircuit(
	file string,
	params *utils.Params,
	inputSizes [][]int,
) (*circuit.Circuit, error) {
	var circ *circuit.Circuit
	var err error

	if circuit.IsFilename(file) {
		circ, err = circuit.Parse(file)
		if err != nil {
			return nil, err
		}
	} else if compiler.IsFilename(file) {
		circ, _, err = compiler.New(params).CompileFile(file, inputSizes)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unknown file type '%s'", file)
	}

	if circ != nil {
		circ.AssignLevels()
	}
	return circ, err
}

func getResult(results []*big.Int, outputs circuit.IO) []byte {
	val := Results(results, outputs)[0].([]byte)
	return val
}

type InputArguments []string

func (i *InputArguments) String() string {
	return fmt.Sprint(*i)
}

func (i *InputArguments) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		*i = append(*i, v)
	}
	return nil
}

type DependencyDirectories []string

func (pkg *DependencyDirectories) String() string {
	return fmt.Sprint(*pkg)
}

func (pkg *DependencyDirectories) Set(value string) error {
	for _, v := range strings.Split(value, ":") {
		*pkg = append(*pkg, v)
	}
	return nil
}
