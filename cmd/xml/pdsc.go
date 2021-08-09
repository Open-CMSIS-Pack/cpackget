/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package main

import (
	"encoding/xml"

	log "github.com/sirupsen/logrus"
)

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

type ReleaseTag struct {
	XMLName xml.Name `xml:"release"`
	Version string   `xml:"version,attr"`
	Date    string   `xml:"Date,attr"`
}

func NewPdsc(fileName string) *PdscXML {
	log.Debugf("Initializing PdscXML object for \"%s\"", fileName)
	p := new(PdscXML)
	p.fileName = fileName
	return p
}

func (p *PdscXML) LatestVersion() string {
	releases := p.ReleasesTag.Releases
	if len(releases) > 0 {
		return releases[0].Version
	}
	return ""
}

func (p *PdscXML) Tag() PdscTag {
	return PdscTag{
		Vendor:  p.Vendor,
		URL:     p.URL,
		Name:    p.Name,
		Version: p.LatestVersion(),
	}
}

func (p *PdscXML) Read() error {
	log.Debugf("Reading pdsc from file \"%s\"", p.fileName)
	return ReadXML(p.fileName, p)
}
