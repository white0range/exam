package main

import (
	"fmt"
	"os"

	"exam/internal/cli"
	"exam/internal/discovery"
	"exam/internal/format"
)

func main() {
	opts, err := cli.ParseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report, err := discovery.Scan(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var output []byte
	if opts.JSON {
		output, err = format.JSON(report)
	} else {
		output, err = format.Text(report)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
