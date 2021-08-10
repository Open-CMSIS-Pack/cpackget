/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
)

func AddPack(packPath string) error {
	log.Debugf("Adding pack \"%v\"", packPath)

	// Sanity check
	info, err := utils.ExtractPackInfo(packPath)
	if err != nil {
		return err
	}

	pack := &PackType{
		PdscTag: xml.PdscTag{
			URL: info.Location,
			Vendor: info.Vendor,
			Name: info.Pack,
			Version: info.Version,
		},
		path: packPath,
	}

	// Check if it's already installed
	if installation.packIsInstalled(pack) {
		return errs.PackAlreadyInstalled
	}

	// Check if it's public
	pack.isPublic = installation.packIsPublic(pack)

	// Fetch if, if needed
	if err = pack.fetch(); err != nil {
		return err
	}

	// Really installs the pack
	if err = pack.install(installation); err != nil {
		return err
	}

	// Touches pack.idx
	return installation.touchPackIdx()
}


/*
func AddPdsc(pdscPath string) error {
	return p.Manager.LocalPidx.HasPdsc(p.ToPdscTag())
	if p.IsLocal {
		pdsc := NewPdsc(p.Path)
		pdsc.Read()
		return p.Manager.LocalPidx.AddPdsc(pdsc.Tag())
	}
}
*/

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
	// PackRoot is the working directory if the packs installation
	packRoot string

	// Packs installed
	packs map[string]bool

	// DownloadDir stores copies of all packs that were installed via pack files
	// from external servers.
	downloadDir string

	// LocalDir stores "local_repository.pidx" containing a list of all packs
	// installed via PDSC files.
	localDir string

	// WebDir stores "index.pidx" containing a list of PDSC tags with all
	// publicly available packs.
	webDir string

	// LocalPidx is a reference to "local_repository.pidx" that contains a flat
	// list of PDSC tags representing all packs installed via PDSC files.
	localPidx *xml.PidxXML

	// PackIdx is the "pack.idx" file used by other tools to be notified that
	// the pack installation had changed.
	packIdx string
}

// touchPackIdx changes the timestamp of pack.idx.
func (p *PacksInstallationType) touchPackIdx() error {
	return utils.TouchFile(p.packIdx)
}

// Save saves the file "local_repository.pidx" to disk
func (p *PacksInstallationType) saveLocalRepository() error {
	return p.localPidx.Write()
}

// packIsInstalled checks whether a given pack is already installed or not
func (p *PacksInstallationType) packIsInstalled(pack *PackType) bool {
	installationDir := path.Join(p.packRoot, pack.Vendor, pack.Name, pack.Version)
	if _, err := os.Stat(installationDir); !os.IsNotExist(err) {
		return true
	}
	return false
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
