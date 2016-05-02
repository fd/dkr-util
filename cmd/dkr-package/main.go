package main

import (
	"fmt"
	"os"

	"github.com/fd/dkr-util/pkg/package"
)

func main() {
	err := dkrpackage.Package(os.Stdout, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
