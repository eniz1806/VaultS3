//go:build windows

package main

import (
	"fmt"
	"os"
)

func runMount(args []string) {
	fmt.Fprintln(os.Stderr, "FUSE mount is not supported on Windows.")
	os.Exit(1)
}

func runUmount(args []string) {
	fmt.Fprintln(os.Stderr, "FUSE unmount is not supported on Windows.")
	os.Exit(1)
}
