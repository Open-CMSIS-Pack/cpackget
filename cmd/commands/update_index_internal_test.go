/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"errors"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	viperType "github.com/spf13/viper"
)

func TestUpdateIndexRejectsMissingPackRoot(t *testing.T) {
	originalViper := viper
	originalCreatePackRoot := createPackRoot
	t.Cleanup(func() {
		viper = originalViper
		createPackRoot = originalCreatePackRoot
	})

	viper = viperType.New()
	viper.Set("pack-root", filepath.Join(t.TempDir(), "missing"))
	createPackRoot = false

	err := UpdateIndexCmd.RunE(UpdateIndexCmd, nil)
	if !errors.Is(err, errs.ErrPackRootDoesNotExist) {
		t.Fatalf("expected ErrPackRootDoesNotExist, got %v", err)
	}
}
