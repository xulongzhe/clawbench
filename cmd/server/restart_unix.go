//go:build !windows

package main

import "syscall"

func selfSignalInterrupt() {
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}
