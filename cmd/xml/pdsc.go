/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

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

	ReleasesTag struct {
		XMLName  xml.Name     `xml:"releases"`
		Releases []ReleaseTag `xml:"release"`
	} `xml:"releases"`

	fileName string
}

// ReleaseTag maps the <release> tag of a PDSC file.
type ReleaseTag struct {
	XMLName xml.Name `xml:"release"`
	Version string   `xml:"version,attr"`
	Date    string   `xml:"Date,attr"`
}

// NewPdscXML receives a PDSC file name to be later read into the PdscXML struct
func NewPdscXML(fileName string) *PdscXML {
	log.Debugf("Initializing PdscXML object for \"%s\"", fileName)
	p := new(PdscXML)
	p.fileName = fileName
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

// Tag returns a PdscTag representation of a PDSC file.
func (p *PdscXML) Tag() PdscTag {
	return PdscTag{
		Vendor:  p.Vendor,
		URL:     p.URL,
		Name:    p.Name,
		Version: p.LatestVersion(),
	}
}

// Read reads the PDSC file specified in p.fileName into the PdscXML struct
func (p *PdscXML) Read() error {
	log.Debugf("Reading pdsc from file \"%s\"", p.fileName)
	return utils.ReadXML(p.fileName, p)
}
