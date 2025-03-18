package main

import (
	"os"

	"github.com/mashiike/actionspin"
)

func main() {
	var c actionspin.CLI
	if code := c.Run(); code != 0 {
		os.Exit(code)
	}
}
