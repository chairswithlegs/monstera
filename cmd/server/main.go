package main

import (
	"os"

	"github.com/chairswithlegs/monstera/cmd/server/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
