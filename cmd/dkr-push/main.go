package main

import (
	"fmt"
	"os"

	"github.com/fd/dkr-util/pkg/push"
)

func main() {
	err := dkrpush.Push(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
