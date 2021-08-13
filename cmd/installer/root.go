/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"os"
	"path"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
)

// AddPack adds a pack to the pack installation directory structure
func AddPack(packPath string) error {
	log.Debugf("Adding pack \"%v\"", packPath)

	pack, err := preparePack(packPath, false)
	if err != nil {
		return err
	}

	if pack.isInstalled {
		return errs.PackAlreadyInstalled
	}

	if err = pack.fetch(); err != nil {
		return err
	}

	if err = pack.install(installation); err != nil {
		return err
	}

	return installation.touchPackIdx()
}

// RemovePack removes a pack given a pack path
func RemovePack(packPath string, purge bool) error {
	log.Debugf("Removing pack \"%v\"", packPath)

	pack, err := preparePack(packPath, true)
	if err != nil {
		return err
	}

	if pack.isInstalled {
		if err = pack.uninstall(installation); err != nil {
			return err
		}

		if purge {
			if err = pack.purge(); err != nil {
				return err
			}
		}

		return installation.touchPackIdx()
	} else if purge {
		return pack.purge()
	} else {
		log.Errorf("Pack \"%v\" is not installed", packPath)
		return errs.PackNotInstalled
	}

	return nil
}

// AddPdsc adds a pack via PDSC file
func AddPdsc(pdscPath string) error {
	log.Debugf("Adding pdsc \"%v\"", pdscPath)

	pdsc, err := preparePdsc(pdscPath, false)
	if err != nil {
		return err
	}

	if pdsc.isInstalled {
		return errs.PdscEntryExists
	}

	if err := pdsc.install(installation); err != nil {
		return err
	}

	if err := installation.localPidx.Write(); err != nil {
		return err
	}

	return installation.touchPackIdx()
}

// RemovePdsc removes a pack given a pdsc path
func RemovePdsc(pdscPath string) error {
	log.Debugf("Removing pdsc \"%v\"", pdscPath)

	pdsc, err := preparePdsc(pdscPath, true)
	if err != nil {
		return err
	}

	if err = pdsc.uninstall(installation); err != nil {
		return err
	}

	if err := installation.localPidx.Write(); err != nil {
		return err
	}

	return installation.touchPackIdx()
}

// installation is a singleton variable that keeps the only reference
// to PacksInstallationType
var installation *PacksInstallationType

// SetPackRoot sets the working directory of the packs installation
func SetPackRoot(packRoot string) error {
	log.Debugf("Setting pack installation working directory to \"%v\"", packRoot)
	installation = &PacksInstallationType{
		packRoot:    packRoot,
		downloadDir: path.Join(packRoot, ".Download"),
		localDir:    path.Join(packRoot, ".Local"),
		webDir:      path.Join(packRoot, ".Web"),
	}
	installation.localPidx = xml.NewPidx(path.Join(installation.localDir, "local_repository.pidx"))
	installation.packIdx = path.Join(packRoot, "pack.idx")

	var err error
	for _, dir := range []string{packRoot, installation.downloadDir, installation.localDir, installation.webDir} {
		if err = utils.EnsureDir(dir); err != nil {
			return err
		}
	}

	return nil
}

// PacksInstallationType is the scruct tha manages Open-CMSIS-Pack installation/deletion.
type PacksInstallationType struct {
	// packRoot is the working directory if the packs installation
	packRoot string

	// packs installed
	packs map[string]bool

	// downloadDir stores copies of all packs that were installed via pack files
	// from external servers.
	downloadDir string

	// localDir stores "local_repository.pidx" containing a list of all packs
	// installed via PDSC files.
	localDir string

	// webDir stores "index.pidx" containing a list of PDSC tags with all
	// publicly available packs.
	webDir string

	// localPidx is a reference to "local_repository.pidx" that contains a flat
	// list of PDSC tags representing all packs installed via PDSC files.
	localPidx *xml.PidxXML

	// localIsLoaded is a flag that tells whether the local_repository.pidx has been loaded or not
	localIsLoaded bool

	// packIdx is the "pack.idx" file used by other tools to be notified that
	// the pack installation had changed.
	packIdx string
}

// touchPackIdx changes the timestamp of pack.idx.
func (p *PacksInstallationType) touchPackIdx() error {
	return utils.TouchFile(p.packIdx)
}

// packIsInstalled checks whether a given pack is already installed or not
func (p *PacksInstallationType) packIsInstalled(pack *PackType) bool {
	installationDir := path.Join(p.packRoot, pack.Vendor, pack.Name, pack.Version)
	if _, err := os.Stat(installationDir); !os.IsNotExist(err) {
		return true
	}
	return false
}

// pdscIsInstalled checks whether a given pack PDSC is already installed or not
func (p *PacksInstallationType) pdscIsInstalled(pdsc *PdscType) (bool, error) {
	// lazyly lists all packs installed via PDSC
	if !p.localIsLoaded {
		if err := p.localPidx.Read(); err != nil {
			return false, err
		}
		p.localIsLoaded = true
	}

	tag, err := pdsc.toPdscTag()
	if err != nil {
		return false, err
	}

	return p.localPidx.HasPdsc(tag), nil
}

// packIsPublic checks whether the pack is public or not.
// Being public means a PDSC file is present in ".Web/" folder
func (p *PacksInstallationType) packIsPublic(pack *PackType) bool {
	// lazyly lists all pdsc files in the ".Web/" folder only once
	if p.packs == nil {
		p.packs = make(map[string]bool)
		files, _ := utils.ListDir(p.webDir, `^.*\\.pdsc$`)
		for _, file := range files {
			p.packs[file] = true
		}
	}

	key := pack.Vendor + "." + pack.Name + ".pdsc"
	return p.packs[key]
}
