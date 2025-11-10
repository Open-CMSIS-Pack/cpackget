//go:build !windows

/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func generatePaths(t *testing.T) map[string]string {
	cwd, err := os.Getwd()
	assert.Nil(t, err)

	dirName := "valid-pack-root"
	absPath := filepath.Join(cwd, dirName)

	// Define a few paths to try out
	return map[string]string{
		"regular absolute path":     absPath,
		"absolute path with ..":     filepath.Join(absPath, "..", dirName),
		"multiple leading slashes":  "////" + absPath,
		"multiple trailing slashes": absPath + "////",
	}
}
