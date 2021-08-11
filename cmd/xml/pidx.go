/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package xml

import (
	"encoding/xml"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

//
//  This file contains all available packages from
//  all vendors.
//
type PidxXML struct {
	XMLName   xml.Name `xml:"index"`
	Timestamp string   `xml:"timestamp"`

	Pindex struct {
		XMLName xml.Name  `xml:"pindex"`
		Pdscs   []PdscTag `xml:"pdsc"`
	} `xml:"pindex"`

	pdscList map[string]bool
	fileName string
}

type PdscTag struct {
	XMLName xml.Name `xml:"pdsc"`
	Vendor  string   `xml:"vendor,attr"`
	URL     string   `xml:"url,attr"`
	Name    string   `xml:"name,attr"`
	Version string   `xml:"version,attr"`
}

func NewPidx(fileName string) *PidxXML {
	log.Debugf("Initializing PidxXML object for \"%s\"", fileName)
	p := new(PidxXML)
	p.fileName = fileName
	return p
}

// AddPdsc takes in a PdscTag and add it to the <pindex> tag
func (p *PidxXML) AddPdsc(pdsc PdscTag) error {
	log.Debugf("Adding pdsc tag \"%s\" pidx file to \"%s\"", pdsc, p.fileName)
	if p.HasPdsc(pdsc) {
		return errs.PdscEntryExists
	}

	p.Pindex.Pdscs = append(p.Pindex.Pdscs, pdsc)
	p.pdscList[pdsc.Key()] = true
	return nil
}

// HasPdsc tells whether of not pdsc is already present in this pidx file
func (p *PidxXML) HasPdsc(pdsc PdscTag) bool {
	log.Debugf("Checking if pidx \"%s\" contains \"%s\"", p.fileName, pdsc.Key())
	return p.pdscList[pdsc.Key()]
}

// Key returns this pdscTag unique key
func (p *PdscTag) Key() string {
	return p.URL + p.Vendor + "." + p.Name + "." + p.Version
}

// Read reads FileName into this PidxXML struct
func (p *PidxXML) Read() error {
	log.Debugf("Reading pidx from file \"%s\"", p.fileName)
	if !utils.FileExists(p.fileName) {
		if err := p.Write(); err != nil {
			return err
		}
	}

	if err := utils.ReadXML(p.fileName, p); err != nil {
		return err
	}

	p.pdscList = make(map[string]bool)
	for _, pdsc := range p.Pindex.Pdscs {
		log.Debugf("Registring \"%s\"", pdsc.Key())
		p.pdscList[pdsc.Key()] = true
	}

	return nil
}

// Save saves this PidxXML struct into its FileName
func (p *PidxXML) Write() error {
	log.Debugf("Writing pidx file to \"%s\"", p.fileName)
	return utils.WriteXML(p.fileName, p)
}

// String returns the string representation of this pdscTag
func (p *PdscTag) String() string {
	tagString := fmt.Sprintf("<pdsc url=\"%s\" vendor=\"%s\" name=\"%s\" version=\"%s\" />", p.URL, p.Vendor, p.Name, p.Version)
	return tagString
}
