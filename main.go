package main

import (
	"os"

	"github.com/dropalltables/cdp/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
