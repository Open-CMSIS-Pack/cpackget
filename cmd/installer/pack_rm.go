/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	//"os"
	//"path"

	log "github.com/sirupsen/logrus"
)

func (manager *PacksManagerType) Uninstall(packName string) error {
	log.Infof("Uninstalling %s", packName)

/*
	pdsc, err := PackPathToPdscTag(packName)
	if err != nil {
		return err
	}
	var pidx *PidxXML
	if manager.Pidx.HasPdsc(pdsc) {
		pidx = manager.Pidx
	} else if manager.LocalPidx.HasPdsc(pdsc) {
		pidx = manager.LocalPidx
	}

	if pidx == nil {
		return ErrPdscNotFound
	}

	packPath := path.Join(manager.PackRoot, pdsc.Vendor, pdsc.Name, pdsc.Version)
	if err := os.RemoveAll(packPath); err != nil {
		return err
	}

	// TODO: If there are left over empty directories

	/*err = pidx.RemovePdsc(pdsc)
	if err != nil {
		log.Errorf("Can't deregister pack %s: %s", pdsc.Key(), err)
		return ErrUnknownBehavior
	}
*/
	return nil
}
