package main

import (
	"fmt"

	mr "github.com/taiyi-research-institute/svarog-messenger/messenger"
)

func main() {
	fmt.Println("Messenger server will listen at 127.0.0.1:65534 ...")
	mr.SpawnServer("127.0.0.1", 65534)
}
