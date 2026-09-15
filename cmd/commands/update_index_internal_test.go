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

func TestUpdateIndexDailyRejectsConflictingVerbosity(t *testing.T) {
	originalViper := viper
	originalDaily := updateIndexCmdFlags.daily
	dailyFlag := UpdateIndexCmd.Flags().Lookup("daily")
	originalDailyValue := dailyFlag.Value.String()
	originalDailyChanged := dailyFlag.Changed
	t.Cleanup(func() {
		viper = originalViper
		updateIndexCmdFlags.daily = originalDaily
		_ = dailyFlag.Value.Set(originalDailyValue)
		dailyFlag.Changed = originalDailyChanged
	})

	viper = viperType.New()
	viper.Set("quiet", true)
	viper.Set("verbose", true)
	updateIndexCmdFlags.daily = false
	dailyFlag.Changed = true

	err := UpdateIndexCmd.RunE(UpdateIndexCmd, nil)
	if err == nil || err.Error() != "both \"-q\" and \"-v\" were specified, please pick only one verboseness option" {
		t.Fatalf("expected conflicting verbosity error, got %v", err)
	}
}

func TestUpdateIndexRejectsMissingPackRoot(t *testing.T) {
	originalViper := viper
	originalCreatePackRoot := createPackRoot
	dailyFlag := UpdateIndexCmd.Flags().Lookup("daily")
	originalDailyChanged := dailyFlag.Changed
	t.Cleanup(func() {
		viper = originalViper
		createPackRoot = originalCreatePackRoot
		dailyFlag.Changed = originalDailyChanged
	})

	viper = viperType.New()
	viper.Set("pack-root", filepath.Join(t.TempDir(), "missing"))
	createPackRoot = false
	dailyFlag.Changed = false

	err := UpdateIndexCmd.RunE(UpdateIndexCmd, nil)
	if !errors.Is(err, errs.ErrPackRootDoesNotExist) {
		t.Fatalf("expected ErrPackRootDoesNotExist, got %v", err)
	}
}
