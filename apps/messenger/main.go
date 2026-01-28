package main

import (
	"fmt"

	"github.com/markkurossi/mpc/ot"
)

func main() {
	fmt.Println("Messenger server will listen at 127.0.0.1:65534 ...")
	ot.SpawnServer("127.0.0.1", 65534)
}
