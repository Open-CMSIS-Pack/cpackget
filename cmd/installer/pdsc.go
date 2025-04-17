/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"path/filepath"
	"runtime"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
)

// PdscType is the struct that represents the installation of a
// pack via PDSC file
type PdscType struct {
	xml.PdscTag

	// file points to the actual PDSC file
	file *xml.PdscXML

	// path points to a file in the local system, whether or not it's local
	path string
}

// preparePdsc does some sanity validation regarding pdsc name
// and check if it's already installed or not
func preparePdsc(pdscPath string) (*PdscType, error) {
	var err error
	pdsc := &PdscType{
		path: pdscPath,
	}

	info, err := utils.ExtractPackInfo(pdscPath)
	if err != nil {
		return pdsc, err
	}
	pdsc.URL = info.Location
	pdsc.Name = info.Pack
	pdsc.Vendor = info.Vendor
	pdsc.Version = info.Version

	if !Installation.localIsLoaded {
		if err := Installation.LocalPidx.Read(); err != nil {
			return pdsc, err
		}
		Installation.localIsLoaded = true
	}

	return pdsc, err
}

// toPdscTag returns a <pdsc> tag representation of this PDSC file
func (p *PdscType) toPdscTag() (xml.PdscTag, error) {
	tag := p.PdscTag

	if p.file == nil {
		p.file = xml.NewPdscXML(p.path)
		if err := p.file.Read(); err != nil {
			return tag, err
		}
	}

	// uses the Version from the actual file
	tag.Version = p.file.Tag().Version

	return tag, nil
}

// install installs a pack via PDSC file.
// It:
//   - Adds it to the "CMSIS_PACK_ROOT/.Local/local_repository.pidx"
//     using version from the PDSC file
func (p *PdscType) install(installation *PacksInstallationType) error {
	log.Debugf("Installing \"%s\"", p.path)
	tag, err := p.toPdscTag()
	if err != nil {
		return err
	}

	searchTag := xml.PdscTag{
		Vendor: tag.Vendor,
		Name:   tag.Name,
	}
	tagURL := tag.URL
	if strings.HasPrefix(tagURL, "file://") && runtime.GOOS == "windows" {
		tagURL = strings.ToLower(tagURL) // case insensitive if windows
	}
	foundTags := installation.LocalPidx.FindPdscTags(searchTag)
	if len(foundTags) > 0 {
		for _, foundTag := range foundTags {
			foundURL := foundTag.URL
			if strings.HasPrefix(foundURL, "file://") && runtime.GOOS == "windows" {
				foundURL = strings.ToLower(foundURL) // case insensitive if windows
			}
			if foundURL == tagURL {
				if strings.Contains(foundTag.URL, "\\") {
					log.Warn("Found PDSC file with an equal but malformed path (using '\\'). Correcting entry and reinstall.")
					if err := installation.LocalPidx.RemovePdsc(foundTag); err != nil {
						return err
					}
				}
				return errs.ErrPdscEntryExists
			}
		}
	}

	return installation.LocalPidx.AddPdsc(tag)
}

// uninstall uninstalls a pack via PDSC
// It:
//   - Removes it to the "CMSIS_PACK_ROOT/.Local/local_repository.pidx"
//     If version is ommited, remove all pdsc tags belonging to this pack
func (p *PdscType) uninstall(installation *PacksInstallationType) error {
	log.Debugf("Unistalling \"%s\"", p.path)

	tags := installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: p.Vendor,
		Name:   p.Name,
	})

	if len(tags) == 0 {
		return errs.ErrPdscEntryNotFound
	}

	toRemove := []xml.PdscTag{}
	dirName := filepath.Dir(p.path)
	if dirName == "" || dirName == "." {
		toRemove = tags
	} else {
		targetPath := strings.ReplaceAll(dirName, "\\", "/")
		for _, tag := range tags {
			tagURL := strings.ReplaceAll(tag.URL, "\\", "/")
			if strings.Contains(tagURL, targetPath) {
				toRemove = append(toRemove, tag)
			}
		}
	}

	if len(toRemove) == 0 {
		return errs.ErrPdscEntryNotFound
	}

	for _, tag := range toRemove {
		if err := installation.LocalPidx.RemovePdsc(tag); err != nil {
			return err
		}
	}
	return nil
}
