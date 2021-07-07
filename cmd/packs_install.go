/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	log "github.com/sirupsen/logrus"
)

// Install receives a pack path (*.pack/*.zip or *pdsc) and installs packs properly
//
// - if packPath is a pdsc file:
//   1. Add a pdsc tag to ".Local/local_repository.pidx"
//     1.1. Vendor, Name and Version should be extracted from reading the pdsc file
//     1.2. URL should be the `dirname` of the given pdsc file
//
// - if packPath is a pack file (either *.pdsc or *.zip extension)
//   1. Save a copy of the file in ".Download/"
//   2. Save a versioned pdsc file in ".Download/"
//   3. Extract all files to "Vendor/Name/Version"
//   4. If pack does not exist in ".Web/index.pidx"
//     4.1. Save an unversioned copy of the pdsc file in ".Local/"
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
