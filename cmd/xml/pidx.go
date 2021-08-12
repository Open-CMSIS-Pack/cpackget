/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package xml

import (
	"encoding/xml"
	"fmt"
	"strings"

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

	pdscList map[string]PdscTag
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
	log.Debugf("Adding pdsc tag \"%s\" to \"%s\"", pdsc, p.fileName)
	if p.HasPdsc(pdsc) {
		return errs.PdscEntryExists
	}

	p.pdscList[pdsc.Key()] = pdsc
	return nil
}

// RemovePdsc takes in a PdscTag and remove it from the <pindex> tag
func (p *PidxXML) RemovePdsc(pdsc PdscTag) error {
	log.Debugf("Removing pdsc tag \"%s\" from \"%s\"", pdsc, p.fileName)

	var toRemove []string

	if pdsc.Version != "" && p.HasPdsc(pdsc) {
		toRemove = append(toRemove, pdsc.Key())
	} else {
		// Version is omitted, search all versions
		targetKey := pdsc.Key()
		for key := range p.pdscList {
			if strings.Contains(key, targetKey) {
				toRemove = append(toRemove, key)
			}
		}
	}

	if len(toRemove) == 0 {
		return errs.PdscEntryNotFound
	}

	for _, key := range toRemove {
		log.Debugf("Removing \"%v\"", key)
		delete(p.pdscList, key)
	}

	return nil
}

// HasPdsc tells whether of not pdsc is already present in this pidx file
func (p *PidxXML) HasPdsc(pdsc PdscTag) bool {
	_, ok := p.pdscList[pdsc.Key()]
	log.Debugf("Checking if pidx \"%s\" contains \"%s\": %v", p.fileName, pdsc.Key(), ok)
	return ok
}

// Key returns this pdscTag unique key
func (p *PdscTag) Key() string {
	return p.Vendor + "." + p.Name + "." + p.Version
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

	p.pdscList = make(map[string]PdscTag)
	for _, pdsc := range p.Pindex.Pdscs {
		log.Debugf("Registring \"%s\"", pdsc.Key())
		p.pdscList[pdsc.Key()] = pdsc
	}

	// truncate Pindex.Pdscs
	p.Pindex.Pdscs = p.Pindex.Pdscs[:0]

	return nil
}

// Save saves this PidxXML struct into its FileName
func (p *PidxXML) Write() error {
	log.Debugf("Writing pidx file to \"%s\"", p.fileName)

	// Use p.pdscList as the main source of pdsc tags
	for _, pdsc := range p.pdscList {
		p.Pindex.Pdscs = append(p.Pindex.Pdscs, pdsc)
	}

	return utils.WriteXML(p.fileName, p)
}
