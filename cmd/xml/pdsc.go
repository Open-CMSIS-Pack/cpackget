/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml

import (
	"encoding/xml"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

// PdscXML maps few tags of a PDSC file.
// Ref: https://github.com/ARM-software/CMSIS_5/blob/develop/CMSIS/Utilities/PACK.xsd
type PdscXML struct {
	XMLName xml.Name `xml:"package"`
	Vendor  string   `xml:"vendor"`
	URL     string   `xml:"url"`
	Name    string   `xml:"name"`
	License string   `xml:"license"`

	ReleasesTag struct {
		XMLName  xml.Name     `xml:"releases"`
		Releases []ReleaseTag `xml:"release"`
	} `xml:"releases"`

	FileName string
}

// ReleaseTag maps the <release> tag of a PDSC file.
type ReleaseTag struct {
	XMLName xml.Name `xml:"release"`
	Version string   `xml:"version,attr"`
	Date    string   `xml:"Date,attr"`
	URL     string   `xml:"url,attr"`
}

// NewPdscXML receives a PDSC file name to be later read into the PdscXML struct
func NewPdscXML(fileName string) *PdscXML {
	log.Debugf("Initializing PdscXML object for \"%s\"", fileName)
	p := new(PdscXML)
	p.FileName = fileName
	return p
}

// LatestVersion returns a string containing version of the first tag within
// the <releases> tag.
func (p *PdscXML) LatestVersion() string {
	releases := p.ReleasesTag.Releases
	if len(releases) > 0 {
		return releases[0].Version
	}
	return ""
}

// AllReleases returns a slice of strings containing all available releases in this Pdsc file
func (p *PdscXML) AllReleases() []string {
	allReleases := []string{}
	if len(p.ReleasesTag.Releases) > 0 {
		for _, releaseTag := range p.ReleasesTag.Releases {
			allReleases = append(allReleases, releaseTag.Version)
		}
	}

	return allReleases
}

// FindReleaseTagByVersion iterates over the PDSC file's releases tag and returns
// the release that matching version.
func (p *PdscXML) FindReleaseTagByVersion(version string) *ReleaseTag {
	releases := p.ReleasesTag.Releases
	if len(releases) > 0 {
		if version == "" {
			return &releases[0]
		}

		for _, releaseTag := range releases {
			if releaseTag.Version == version {
				return &releaseTag
			}
		}
	}
	return nil
}

// Tag returns a PdscTag representation of a PDSC file.
func (p *PdscXML) Tag() PdscTag {
	return PdscTag{
		Vendor:  p.Vendor,
		URL:     p.URL,
		Name:    p.Name,
		Version: p.LatestVersion(),
	}
}

// Read reads the PDSC file specified in p.FileName into the PdscXML struct
func (p *PdscXML) Read() error {
	log.Debugf("Reading pdsc from file \"%s\"", p.FileName)
	return utils.ReadXML(p.FileName, p)
}

// PackURL returns a url for the Pack described in this PDSC file
func (p *PdscXML) PackURL(version string) string {
	baseURL := p.URL
	lenBaseURL := len(baseURL)
	if lenBaseURL > 0 && baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}

	if version == "" {
		version = p.LatestVersion()
	}

	return baseURL + p.Vendor + "." + p.Name + "." + version + ".pack"
}
