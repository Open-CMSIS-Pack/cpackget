/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml

import (
	"encoding/xml"
	"path"
	"path/filepath"
	"strings"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

var (
	PdscIndexNotFound = -1
)

// PidxXML maps the PIDX file format.
// Ref: https://github.com/ARM-software/CMSIS_5/blob/develop/CMSIS/Utilities/PackIndex.xsd
type PidxXML struct {
	XMLName       xml.Name `xml:"index"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	Vendor        string   `xml:"vendor"`
	URL           string   `xml:"url"`
	TimeStamp     string   `xml:"timestamp,omitempty"`

	Pindex struct {
		XMLName xml.Name  `xml:"pindex"`
		Pdscs   []PdscTag `xml:"pdsc"`
	} `xml:"pindex"`

	pdscList map[string][]PdscTag
	fileName string
}

// PdscTag maps a <pdsc> tag that goes in PIDX files.
type PdscTag struct {
	XMLName xml.Name `xml:"pdsc"`
	Vendor  string   `xml:"vendor,attr"`
	URL     string   `xml:"url,attr"`
	Name    string   `xml:"name,attr"`
	Version string   `xml:"version,attr"`
}

// NewPidxXML creates a new instance of the PidxXML struct.
func NewPidxXML(fileName string) *PidxXML {
	log.Debugf("Initializing PidxXML object for \"%s\"", fileName)
	p := new(PidxXML)
	p.fileName = fileName
	return p
}

// AddPdsc takes in a PdscTag and add it to the <pindex> tag.
func (p *PidxXML) AddPdsc(pdsc PdscTag) error {
	log.Debugf("Adding pdsc tag \"%s\" to \"%s\"", pdsc, p.fileName)
	if p.HasPdsc(pdsc) != PdscIndexNotFound {
		return errs.ErrPdscEntryExists
	}

	key := pdsc.Key()
	p.pdscList[key] = append(p.pdscList[key], pdsc)
	return nil
}

// RemovePdsc takes in a PdscTag and remove it from the <pindex> tag.
func (p *PidxXML) RemovePdsc(pdsc PdscTag) error {
	log.Debugf("Removing pdsc tag \"%s\" from \"%s\"", pdsc, p.fileName)

	// removeInfo serves as a helper to identify which pdsc tags need removal
	// key is mandatory pdscTag.Key() formatted as Vendor.Pack[.x.y.z]
	// index is the index of the pdsc tags available for Vendor.Pack.x.y.z,
	type removeInfo struct {
		key   string
		index int
	}

	toRemove := []removeInfo{}

	if pdsc.Version != "" {
		if index := p.HasPdsc(pdsc); index != PdscIndexNotFound {
			toRemove = append(toRemove, removeInfo{
				key:   pdsc.Key(),
				index: index,
			})
		}
	} else {
		// Version is omitted, search all versions
		targetKey := pdsc.Key()
		for key := range p.pdscList {
			if strings.Contains(key, targetKey) {
				tags := p.pdscList[key]
				index := PdscIndexNotFound
				for i, tag := range tags {
					if tag.URL == pdsc.URL {
						index = i
						break
					}
				}
				if index != PdscIndexNotFound {
					toRemove = append(toRemove, removeInfo{
						key:   key,
						index: index,
					})
				}
			}
		}
	}

	if len(toRemove) == 0 {
		return errs.ErrPdscEntryNotFound
	}

	for _, info := range toRemove {
		log.Debugf("Removing \"%v:%d\"", info.key, info.index)
		p.pdscList[info.key] = append(p.pdscList[info.key][:info.index], p.pdscList[info.key][info.index+1:]...)
		if len(p.pdscList[info.key]) == 0 {
			delete(p.pdscList, info.key)
		}
	}

	return nil
}

// HasPdsc tells whether of not pdsc is already present in this pidx file.
// It returns the index of the matching pdsc tag, or -1 if not found
func (p *PidxXML) HasPdsc(pdsc PdscTag) int {
	index := PdscIndexNotFound
	if tags, found := p.pdscList[pdsc.Key()]; found {
		for i, tag := range tags {
			if tag.URL == pdsc.URL {
				index = i
				break
			}
		}
	}

	log.Debugf("Checking if pidx \"%s\" contains \"%s (%s)\": %d", p.fileName, pdsc.Key(), pdsc.URL, index)
	return index
}

// ListPdscTags returns a map of PdscTags in the pidx document
func (p *PidxXML) ListPdscTags() []PdscTag {
	tags := []PdscTag{}
	for _, pdscTags := range p.pdscList {
		tags = append(tags, pdscTags...)
	}
	return tags
}

// FindPdscTags takes in a sample pdscTag and returns the actual PDSC tag inside this PidxXML.
func (p *PidxXML) FindPdscTags(pdsc PdscTag) []PdscTag {
	log.Debugf("Searching for pdsc \"%s\"", pdsc.Key())
	if pdsc.Version != "" {
		foundTags := p.pdscList[pdsc.Key()]
		log.Debugf("\"%s\" contains %d pdsc tag(s) for \"%s\"", p.fileName, len(foundTags), pdsc.Key())
		return foundTags
	}

	// No version, means "Vendor.Pack", so we need
	// to find matching tags that start with "Vendor.Pack",
	// and there might be many
	targetKey := pdsc.Key()
	foundTags := []PdscTag{}
	for key := range p.pdscList {
		if strings.Contains(key, targetKey) {
			foundTags = append(foundTags, p.pdscList[key]...)
		}
	}

	log.Debugf("\"%s\" contains %d pdsc tag(s) for \"%s\"", p.fileName, len(foundTags), pdsc.Key())
	return foundTags
}

// Check timestamp of public index
func (p *PidxXML) CheckTime() error {
	log.Debugf("Checking timestamp of pidx \"%s\"", p.fileName)

	p.pdscList = make(map[string][]PdscTag)

	if !utils.FileExists(p.fileName) {
		return nil
	}
	if err := utils.ReadXML(p.fileName, p); err != nil {
		return err
	}
	if len(p.TimeStamp) == 0 {
		return errs.ErrIndexTooOld // if there is no timestamp it always is too old
	}
	if t, err := time.Parse(time.RFC3339Nano, p.TimeStamp); err != nil {
		return err
	} else {
		if time.Since(t).Hours() > 24 { // index.pidx older than 1 day
			return errs.ErrIndexTooOld
		}
	}
	return nil
}

// Read reads FileName into this PidxXML struct and allocates memory for all PDSC tags.
func (p *PidxXML) Read() error {
	log.Debugf("Reading pidx from file \"%s\"", p.fileName)

	p.pdscList = make(map[string][]PdscTag)

	// Create a new empty l
	if !utils.FileExists(p.fileName) {
		log.Debugf("\"%v\" not found. Creating a new one.", p.fileName)
		p.SchemaVersion = "1.1.0"
		vendorName := ""
		if p.URL == "" {
			vendorName = "local_repository.pidx"
		} else {
			vendorName = path.Base(p.fileName)
		}
		t := time.Now()
		p.TimeStamp = t.Format(time.RFC3339Nano)
		p.Vendor = strings.TrimSuffix(vendorName, filepath.Ext(vendorName))
		return p.Write()
	}
	if err := utils.ReadXML(p.fileName, p); err != nil {
		return err
	}

	for _, pdsc := range p.Pindex.Pdscs {
		key := pdsc.Key()
		log.Debugf("Registring \"%s\"", key)
		p.pdscList[key] = append(p.pdscList[key], pdsc)
	}

	// truncate Pindex.Pdscs
	p.Pindex.Pdscs = p.Pindex.Pdscs[:0]

	return nil
}

// Save saves this PidxXML struct into its fileName.
func (p *PidxXML) Write() error {
	log.Debugf("Writing pidx file to \"%s\"", p.fileName)

	// Use p.pdscList as the main source of pdsc tags
	for _, pdscs := range p.pdscList {
		p.Pindex.Pdscs = append(p.Pindex.Pdscs, pdscs...)
	}

	err := utils.WriteXML(p.fileName, p)

	// Truncate Pindex.Pdscs in case a new Write is requested
	p.Pindex.Pdscs = p.Pindex.Pdscs[:0]

	return err
}

// Key returns this pdscTag unique key.
func (p *PdscTag) Key() string {
	return p.Vendor + "." + p.Name + "." + p.Version
}

// YamlPackId formats the packId as specified in
// https://github.com/Open-CMSIS-Pack/devtools/blob/tools/toolbox/0.10.0/tools/projmgr/docs/Manual/YML-Format.md#pack-name-conventions
func (p *PdscTag) YamlPackID() string {
	return p.Vendor + "::" + p.Name + "@" + p.Version
}

// PackURL constructs a URL of a pack using the tag's attributes.
func (p *PdscTag) PackURL() string {
	return p.URL + p.Key() + ".pack"
}
