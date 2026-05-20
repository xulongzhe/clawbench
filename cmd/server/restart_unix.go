//go:build !windows

package main

import "syscall"

func selfSignalInterrupt() {
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}
