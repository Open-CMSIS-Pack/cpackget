/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	log "github.com/sirupsen/logrus"
)

func (manager *PacksManagerType) Install(packPath string) error {
	log.Infof("Installing %s", packPath)

	pack, err := manager.NewPackInstallation(packPath)
	if err != nil {
		return err
	}

	var pidx *PidxXML
	if pack.IsLocal {
		pidx = manager.LocalPidx
	} else {
		pidx = manager.Pidx
	}

	pdsc := pack.ToPdscTag()
	if pidx.HasPdsc(pdsc) {
		log.Infof("Pack %s is already installed", pdsc.Key())
		return ErrPdscEntryExists
	}

	err = pack.Fetch()
	if err != nil {
		return err
	}

	err = pack.Install()
	if err != nil {
		return err
	}

	err = pidx.AddPdsc(pdsc)
	if err != nil {
		log.Errorf("Can't register pack %s: %s", pdsc.Key(), err)
		return ErrUnknownBehavior
	}

	return nil
}
