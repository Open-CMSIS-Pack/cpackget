//go:build windows
// +build windows

/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"syscall"
)

var expectedFileNotFoundError = syscall.ERROR_PATH_NOT_FOUND
