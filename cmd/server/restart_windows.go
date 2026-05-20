//go:build windows

package main

import "os"

func selfSignalInterrupt() {
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(os.Interrupt)
}
