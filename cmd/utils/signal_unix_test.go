//go:build !windows

/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"syscall"
	"testing"
)

func sendCtrlC(_ *testing.T, pid int) {
	_ = syscall.Kill(pid, syscall.SIGINT)
}
