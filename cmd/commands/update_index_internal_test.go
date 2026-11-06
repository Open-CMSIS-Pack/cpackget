/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	viperType "github.com/spf13/viper"
	"github.com/stretchr/testify/require"
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

func TestUpdateTargetIndexCommand(t *testing.T) {
	originalViper, originalInstallation := viper, installer.Installation
	originalCacheDir := utils.CacheDir
	originalOutput, originalLevel := log.StandardLogger().Out, log.GetLevel()
	t.Cleanup(func() {
		viper, installer.Installation = originalViper, originalInstallation
		utils.CacheDir = originalCacheDir
		log.SetOutput(originalOutput)
		log.SetLevel(originalLevel)
	})
	for _, name := range []string{"conflicting verbosity", "missing pack root", "blocked target directory", "successful rebuild"} {
		t.Run(name, func(t *testing.T) {
			viper = viperType.New()
			root := t.TempDir()
			for _, directory := range []string{".Web", ".Local", ".Download"} {
				require.NoError(t, os.Mkdir(filepath.Join(root, directory), 0755))
			}
			viper.Set("pack-root", root)
			command := *UpdateTargetIndexCmd
			command.SetOut(io.Discard)
			switch name {
			case "conflicting verbosity":
				viper.Set("quiet", true)
				viper.Set("verbose", true)
			case "missing pack root":
				viper.Set("pack-root", filepath.Join(root, "missing"))
			case "blocked target directory":
				require.NoError(t, os.WriteFile(filepath.Join(root, ".Targets"), []byte("blocked"), 0600))
			}
			err := command.RunE(&command, nil)
			switch name {
			case "conflicting verbosity":
				require.ErrorContains(t, err, "please pick only one")
			case "missing pack root":
				require.ErrorIs(t, err, errs.ErrPackRootDoesNotExist)
			case "blocked target directory":
				require.Error(t, err)
			default:
				require.NoError(t, err)
				require.FileExists(t, filepath.Join(root, ".Targets", "index.tidx"))
			}
		})
	}
}
