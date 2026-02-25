package main

import (
	"os"

	"github.com/chairswithlegs/monstera-fed/cmd/monstera-fed/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
