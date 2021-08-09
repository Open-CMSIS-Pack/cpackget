/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

// Install receives a pack path (*.pack/*.zip or *pdsc) and installs packs properly
func (manager *PacksManagerType) Install(packPath string) error {
	log.Infof("Installing %s", packPath)

	pack, err := manager.NewPackInstallation(packPath)
	if err != nil {
		return err
	}

	if pack.IsInstalled() {
		return ErrPackAlreadyInstalled
	}

	if strings.HasSuffix(pack.Path, ".pdsc") {
		pack.IsLocal = true
	}

	if manager.WebPidx.HasPdsc(pack.ToPdscTag()) {
		pack.IsPublic = true
	}

	err = pack.Fetch()
	if err != nil {
		return err
	}

	return pack.Install()
}
