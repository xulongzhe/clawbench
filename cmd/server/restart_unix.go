//go:build !windows

package main

import "syscall"

// signalSelf is the function called by selfSignalInterrupt to deliver the signal.
// Overridden in tests to avoid killing the test process.
var signalSelf = func(sig syscall.Signal) error {
	return syscall.Kill(syscall.Getpid(), sig)
}

func selfSignalInterrupt() {
	_ = signalSelf(syscall.SIGINT)
}
