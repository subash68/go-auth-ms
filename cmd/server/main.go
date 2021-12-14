package main

import (
	"fmt"
	"os"

	cmd "github.com/subash68/authenticator/src/cmd"
)

func main() {
	if err := cmd.RunServer(); err != nil {
		fmt.Fprint(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
