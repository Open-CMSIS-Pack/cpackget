//go:build windows
// +build windows

/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"syscall"
	"testing"
)

func sendCtrlC(t *testing.T, pid int) {
	// https://go.dev/src/os/signal/signal_windows_test.go
	d, e := syscall.LoadDLL("kernel32.dll")
	if e != nil {
		t.Fatalf("LoadDLL: %v\n", e)
	}
	p, e := d.FindProc("GenerateConsoleCtrlEvent")
	if e != nil {
		t.Fatalf("FindProc: %v\n", e)
	}
	r, _, e := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if r == 0 {
		t.Fatalf("GenerateConsoleCtrlEvent: %v\n", e)
	}
}
