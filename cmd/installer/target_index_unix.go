//go:build !windows

/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import "os"

func replaceTargetIndex(source, destination string) error {
	return os.Rename(source, destination)
}
