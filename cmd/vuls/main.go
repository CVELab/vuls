package main

import (
	"fmt"
	"os"

	"github.com/cvelab/vuls/pkg/cmd/root"
)

func main() {
	if err := root.NewCmdRoot().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to exec vuls: %s\n", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}
}
