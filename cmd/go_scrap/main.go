package main

import (
	"fmt"
	"os"

	"go_scrap/internal/entrypoint"
)

func main() {
	code, err := entrypoint.Execute(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	if err != nil || code != 0 {
		os.Exit(code)
	}
}
