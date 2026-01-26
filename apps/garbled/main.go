//
// main.go
//
// Copyright (c) 2019-2025 Markku Rossi
//
// All rights reserved.
//

package main

/*
#include <stdlib.h>
*/
import "C"

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
	"unsafe"

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

// C-exported evaluator function for Rust FFI
// Parameters:
//   - circ_dir: directory containing circuit files (null-terminated C string)
//   - sid: session ID (null-terminated C string)
//   - ui: secret integer as hex string with "0x" prefix (null-terminated C string)
//   - result_ptr: pointer to store the result byte array
//   - result_len: pointer to store the length of result
//
// Returns: 0 on success, -1 on error
//
//export c_evaluator_fn
func c_evaluator_fn(circ_dir *C.char, sid *C.char, ui *C.char, result_ptr **C.uchar, result_len *C.int) C.int {
	// Convert C strings to Go strings
	goCircDir := C.GoString(circ_dir)
	goSid := C.GoString(sid)
	goUi := C.GoString(ui)

	// Call the Go evaluator function
	buf, err := evaluator_fn(goCircDir, goSid, goUi)
	if err != nil {
		log.Printf("c_evaluator_fn error: %v", err)
		return -1
	}

	// Allocate C memory for result and copy data
	*result_len = C.int(len(buf))
	*result_ptr = (*C.uchar)(C.malloc(C.size_t(len(buf))))
	if *result_ptr == nil {
		log.Printf("c_evaluator_fn: failed to allocate memory")
		return -1
	}

	// Copy Go slice to C memory
	for i, b := range buf {
		*(*C.uchar)(unsafe.Pointer(uintptr(unsafe.Pointer(*result_ptr)) + uintptr(i))) = C.uchar(b)
	}

	return 0
}

// C-exported garbler function for Rust FFI
// Parameters:
//   - circ_dir: directory containing circuit files (null-terminated C string)
//   - session_id: session ID (null-terminated C string)
//   - ui: secret integer as hex string with "0x" prefix (null-terminated C string)
//   - cc: chain code as hex string with "0x" prefix (null-terminated C string)
//   - cnum: chain number as decimal string (null-terminated C string)
//   - result_ptr: pointer to store the result byte array
//   - result_len: pointer to store the length of result
//
// Returns: 0 on success, -1 on error
//
//export c_garbler_fn
func c_garbler_fn(circ_dir *C.char, session_id *C.char, ui *C.char, cc *C.char, cnum *C.char, result_ptr **C.uchar, result_len *C.int) C.int {
	// Convert C strings to Go strings
	goCircDir := C.GoString(circ_dir)
	goSessionId := C.GoString(session_id)
	goUi := C.GoString(ui)
	goCc := C.GoString(cc)
	goCnum := C.GoString(cnum)

	// Call the Go garbler function
	buf, err := garbler_fn(goCircDir, goSessionId, goUi, goCc, goCnum)
	if err != nil {
		log.Printf("c_garbler_fn error: %v", err)
		return -1
	}

	// Allocate C memory for result and copy data
	*result_len = C.int(len(buf))
	*result_ptr = (*C.uchar)(C.malloc(C.size_t(len(buf))))
	if *result_ptr == nil {
		log.Printf("c_garbler_fn: failed to allocate memory")
		return -1
	}

	// Copy Go slice to C memory
	for i, b := range buf {
		*(*C.uchar)(unsafe.Pointer(uintptr(unsafe.Pointer(*result_ptr)) + uintptr(i))) = C.uchar(b)
	}

	return 0
}

// Free memory allocated by C functions
//
//export c_free_result
func c_free_result(ptr *C.uchar) {
	C.free(unsafe.Pointer(ptr))
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return
	}

	var args InputArguments
	flag.Var(&args, "i", "comma-separated list of circuit inputs")
	evaluator := flag.Bool("e", false, "evaluator / garbler mode")
	flag.Parse()

	var buf []byte
	if *evaluator {
		buf, err = evaluator_fn(cwd+"/circuits",
			"dummy_session_id", args[0])
		log.Println("evaluator result:", hex.EncodeToString(buf))
	} else {
		buf, err = garbler_fn(cwd+"/circuits",
			"dummy_session_id", args[0], args[1], args[2])
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
	circ_dir string,
	sid string,
	ui string, // "0x"-leading hex string, within 32 bytes.
) ([]byte, error) {
	params := utils.NewParams()
	params.Verbose = false
	params.OptPruneGates = true
	params.PkgPath = []string{circ_dir}
	defer params.Close()
	args := []string{ui}

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
		circPath := circ_dir + "/bip32_derive_tweak_ec.mpcl"
		circ, err = loadCircuit(circPath, params, inputSizes)
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
		return nil, errors.Wrapf(err, "in evaluator_fn(), filepath=%s", circ)
	}
	result, err := circuit.Evaluator(conn, oti, circ, input, false)
	conn.Close()
	if err != nil && err != io.EOF {
		return nil, errors.Wrapf(err, "in evaluator_fn(), filepath=%s", circ)
	}
	val := getResult(result, circ.Outputs)
	return val, nil
}

func garbler_fn(
	circ_dir string,
	session_id string,
	ui string, // "0x"-leading hex string, within 32 bytes.
	cc string, // "0x"-leading hex string, within 32 bytes.
	cnum string, // Integer as decimal string
) ([]byte, error) {
	params := utils.NewParams()
	params.Verbose = false
	params.PkgPath = []string{circ_dir}
	params.OptPruneGates = true
	defer params.Close()
	args := []string{ui, cc, cnum}

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

	circPath := circ_dir + "/bip32_derive_tweak_ec.mpcl"
	circ, err := loadCircuit(circPath, params, inputSizes)
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
		return nil, errors.Wrapf(err, "in garbler_fn(), filepath=%s", circ)
	}
	result, err := circuit.Garbler(params.Config, conn, oti, circ, input, false)
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
