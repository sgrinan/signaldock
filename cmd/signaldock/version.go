package main

import "fmt"

var (
	version = "dev"
	commit  = "unknown"
)

func printVersion() {
	fmt.Printf("SignalDock %s\ncommit: %s\n", version, commit)
}
