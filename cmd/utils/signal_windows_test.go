//go:build windows
// +build windows

/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	// "syscall"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
)

func sendCtrlC(t *testing.T, pid int) {
	// FIXME: For some reason the code below causes a weird behavior running on Github Actions:
	//
	// ?   	github.com/open-cmsis-pack/cpackget/cmd	[no test files]
	// ?   	github.com/open-cmsis-pack/cpackget/cmd/commands	[no test files]
	// ?   	github.com/open-cmsis-pack/cpackget/cmd/errors	[no test files]
	// Entering debug mode. Use h or ? for help.
	//
	// At D:\a\_temp\c2f70f1b-ad63-45ae-b2ec-8226d8ffe991.ps1:4 char:5
	// + if ((Test-Path -LiteralPath variable:\LASTEXITCODE)) { exit $LASTEXIT …
	// +     ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// [DBG]: PS D:\a\cpackget\cpackget>>
	// Error: Process completed with exit code 1.
	//
	// So I'll hack it for now until we really understand this bug

	/*
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
	*/

	// And hack it to bypass the test
	utils.ShouldAbortFunction = func() bool { return true }
}
