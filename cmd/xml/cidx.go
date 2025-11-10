/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml

import (
	"encoding/xml"
	"strings"
	"sync"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

var (
	CIndexNotFound = -1
)

// CidxXML maps the CIDX file format.
type CidxXML struct {
	XMLName       xml.Name `xml:"index"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	TimeStamp     string   `xml:"timestamp,omitempty"`

	Cindex struct {
		XMLName xml.Name   `xml:"cindex"`
		Pdscs   []CacheTag `xml:"pdsc"`
	} `xml:"cindex"`

	mu           sync.Mutex            // protects concurrent access to maps and slices
	pdscList     map[string][]CacheTag // map of CacheTag.Key() to CacheTag
	pdscListName map[string]string     // map of lowercase Vendor.Pack to CacheTag.Key()
	fileName     string
}

// CacheTag maps a <pdsc> tag that goes in CIDX files.
type CacheTag struct {
	XMLName xml.Name `xml:"pdsc"`
	URL     string   `xml:"url,attr"`
	Vendor  string   `xml:"vendor,attr"`
	Name    string   `xml:"name,attr"`
	Version string   `xml:"version,attr"`
}

// NewCidxXML initializes a new CidxXML object with the given file name.
// It logs the initialization process and returns a pointer to the created CidxXML object.
//
// Parameters:
//   - fileName: The name of the file to be associated with the CidxXML object.
//
// Returns:
//   - *CidxXML: A pointer to the newly created CidxXML object.
func NewCidxXML(fileName string) *CidxXML {
	log.Debugf("Initializing CidxXML object for %q", fileName)
	c := new(CidxXML)
	c.fileName = fileName
	return c
}

// GetFileName returns the file name associated with the CidxXML instance.
func (c *CidxXML) GetFileName() string {
	return c.fileName
}

func (c *CidxXML) SetFileName(fileName string) {
	c.fileName = fileName
}

func (c *CidxXML) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pdscList = make(map[string][]CacheTag)
	c.pdscListName = make(map[string]string)
	// truncate Cindex.Pdscs
	c.Cindex.Pdscs = c.Cindex.Pdscs[:0]
}

// AddPdsc adds or updates a pdsc tag in the CidxXML cache index.
// If a pdsc with the same vendor and pack name (case-insensitive) already exists,
// it updates the URL and version of the existing entry and re-indexes it with the new key.
// Otherwise, it creates a new pdsc entry.
// The method maintains both a key-based index (pdscList) and a name-based lookup map (pdscListName).
//
// Parameters:
//   - cTag: The CacheTag containing pdsc information to add or update
//
// Returns:
//   - error: Always returns nil in the current implementation
func (c *CidxXML) AddPdsc(cTag CacheTag) error {
	log.Debugf("Adding pdsc tag %v to %q", cTag, c.fileName)
	c.mu.Lock()
	defer c.mu.Unlock()

	name := strings.ToLower(cTag.VName())
	key, ok := c.pdscListName[name]
	if ok {
		oldPdsc := c.pdscList[key]
		if len(oldPdsc) > 0 {
			oldPdsc[0].URL = cTag.URL
			oldPdsc[0].Version = cTag.Version
			delete(c.pdscList, key) // remove the old key
			key = cTag.Key()        // and replace by new one
			c.pdscList[key] = oldPdsc
		} else {
			// If the slice is empty, treat it as a new entry
			key = cTag.Key()
			c.pdscList[key] = append(c.pdscList[key], cTag)
		}
	} else {
		key = cTag.Key() // insert new key
		c.pdscList[key] = append(c.pdscList[key], cTag)
	}
	c.pdscListName[name] = key
	return nil
}

// ReplacePdscVersion replaces the version of an existing PDSC tag in the CidxXML structure.
// It updates the version of the PDSC tag identified by the given cTag.
// If the PDSC tag is not found, it returns an error.
//
// Parameters:
//
//	cTag - The cTag containing the new version information.
//
// Returns:
//
//	error - An error if the CacheTag is not found, otherwise nil.
func (c *CidxXML) ReplacePdscVersion(cTag CacheTag) error {
	log.Debugf("Replacing version of pdsc tag %v", cTag)
	c.mu.Lock()
	defer c.mu.Unlock()

	name := strings.ToLower(cTag.VName())
	key, ok := c.pdscListName[name]
	if !ok {
		return errs.ErrPdscEntryNotFound
	}

	oldPdsc := c.pdscList[key]
	if len(oldPdsc) == 0 {
		return errs.ErrPdscEntryNotFound
	}
	oldPdsc[0].URL = cTag.URL
	oldPdsc[0].Version = cTag.Version
	delete(c.pdscList, key) // remove the old key
	key = cTag.Key()        // and replace by new one
	c.pdscList[key] = oldPdsc
	c.pdscListName[name] = key
	return nil
}

// Empty checks if the CidxXML instance has an uninitialized or nil pdscList.
// It returns true if pdscList is nil, indicating that the CidxXML is empty.
func (c *CidxXML) Empty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pdscList) == 0
}

// RemovePdsc removes a CacheTag from the CidxXML structure.
// It first identifies which pdsc tags need to be removed based on the provided CacheTag.
// If the CacheTag includes a version, it checks if the tag exists and marks it for removal.
// If the version is omitted, it searches all versions and marks the matching tags for removal.
// If no matching tags are found, it returns an error indicating that the Cache entry was not found.
// Otherwise, it removes the identified tags from the pdscList and updates the pdscListName accordingly.
//
// Parameters:
//   - pdsc: The CacheTag to be removed.
//
// Returns:
//   - error: An error if the Cache entry was not found, otherwise nil.
func (c *CidxXML) RemovePdsc(pdsc CacheTag) error {
	log.Debugf("Removing pdsc tag \"%v\" from %q", pdsc, c.fileName)

	c.mu.Lock()
	defer c.mu.Unlock()

	// removeInfo serves as a helper to identify which pdsc tags need removal
	// key is mandatory pdscTag.Key() formatted as Vendor.Pack[.x.y.z]
	// index is the index of the pdsc tags available for Vendor.Pack.x.y.z,
	type removeInfo struct {
		key   string
		index int
	}

	toRemove := []removeInfo{}

	name := strings.ToLower(pdsc.VName())
	if pdsc.Version != "" {
		if index := c.hasPdscUnsafe(pdsc); index != PdscIndexNotFound {
			toRemove = append(toRemove, removeInfo{
				key:   pdsc.Key(),
				index: index,
			})
		}
	} else {
		// Version is omitted, search all versions
		key, ok := c.pdscListName[name]
		if ok {
			tags := c.pdscList[key]
			index := PdscIndexNotFound
			for i, tag := range tags {
				if tag.VName() == pdsc.VName() {
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

	if len(toRemove) == 0 {
		return errs.ErrPdscEntryNotFound
	}

	for _, info := range toRemove {
		log.Debugf("Removing \"%v:%d\"", info.key, info.index)
		c.pdscList[info.key] = append(c.pdscList[info.key][:info.index], c.pdscList[info.key][info.index+1:]...)
		if len(c.pdscList[info.key]) == 0 {
			delete(c.pdscList, info.key)
			delete(c.pdscListName, name)
		}
	}

	return nil
}

// HasPdsc checks if the CidxXML contains the specified PdscTag.
// It returns the index of the PdscTag if found, otherwise it returns PdscIndexNotFound.
//
// Parameters:
//
//	pdsc (cTag): The cTag to search for in the CidxXML.
//
// Returns:
//
//	int: The index of the CacheTag if found, otherwise PdscIndexNotFound.
//
// hasPdscUnsafe is an internal method that checks for PDSC without locking.
// Must be called with c.mu already locked.
func (c *CidxXML) hasPdscUnsafe(pdsc CacheTag) int {
	index := PdscIndexNotFound
	if tags, found := c.pdscList[pdsc.Key()]; found {
		for i, tag := range tags {
			if tag.Key() == pdsc.Key() {
				index = i
				break
			}
		}
	}
	return index
}

func (c *CidxXML) HasPdsc(pdsc CacheTag) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := c.hasPdscUnsafe(pdsc)
	log.Debugf("Checking if pidx %q contains \"%s (%s)\": %d", c.fileName, pdsc.Key(), pdsc.URL, index)
	return index
}

// ListPdscTags returns a slice of PdscTag containing all the PDSC tags
// from the pdscList field of the CidxXML struct. It iterates over each
// element in the pdscList and appends the tags to the resulting slice.
func (c *CidxXML) ListPdscTags() []CacheTag {
	c.mu.Lock()
	defer c.mu.Unlock()
	tags := []CacheTag{}
	for _, pdscTags := range c.pdscList {
		tags = append(tags, pdscTags...)
	}
	return tags
}

// FindPdscTags searches for PDSC tags in the CidxXML structure.
// If the provided PdscTag has a version, it returns the tags that match the exact key.
// If the version is empty, it searches for tags that start with the "Vendor.Pack" key.
//
// Parameters:
//   - pdsc: The PdscTag to search for.
//
// Returns:
//   - A slice of CacheTag containing the found tags.
func (c *CidxXML) FindPdscTags(pdsc CacheTag) []CacheTag {
	log.Debugf("Searching for pdsc %q", pdsc.Key())
	c.mu.Lock()
	defer c.mu.Unlock()
	if pdsc.Version != "" {
		foundTags := c.pdscList[pdsc.Key()]
		log.Debugf("%q contains %d pdsc tag(s) for %q", c.fileName, len(foundTags), pdsc.Key())
		return foundTags
	}

	// No version, means "Vendor.Pack"
	name := strings.ToLower(pdsc.VName())
	foundKey, ok := c.pdscListName[name]
	foundTags := []CacheTag{}
	if ok {
		foundTags = c.pdscList[foundKey]
	}
	log.Debugf("%q contains %d pdsc tag(s) for %q", c.fileName, len(foundTags), foundKey)
	return foundTags
}

func (c *CidxXML) FindPdscNameTags(pdsc CacheTag) []CacheTag {
	log.Debugf("Searching for pdsc %q", pdsc.VName())
	c.mu.Lock()
	defer c.mu.Unlock()
	name := strings.ToLower(pdsc.VName())
	foundKey, ok := c.pdscListName[name]
	foundTags := []CacheTag{}
	if ok {
		foundTags = c.pdscList[foundKey]
	}
	log.Debugf("%q contains %d pdsc tag(s) for %q", c.fileName, len(foundTags), foundKey)
	return foundTags
}

// CheckTime verifies the timestamp of the CidxXML file.
// It performs the following steps:
// 1. Logs the action of checking the timestamp.
// 2. Initializes the pdscList map.
// 3. Checks if the file exists; if not, it returns nil.
// 4. Reads the XML content of the file into the CidxXML struct.
// 5. Checks if the timestamp is present; if not, returns an error indicating the index is too old.
// 6. Parses the timestamp and checks if it is older than 24 hours; if so, returns an error indicating the index is too old.
// Returns an error if any of the steps fail, otherwise returns nil.
func (c *CidxXML) CheckTime() error {
	log.Debugf("Checking timestamp of cidx %q", c.fileName)

	c.pdscList = make(map[string][]CacheTag)

	if !utils.FileExists(c.fileName) {
		return nil
	}
	if err := utils.ReadXML(c.fileName, c); err != nil {
		return err
	}
	if len(c.TimeStamp) == 0 {
		return errs.ErrIndexTooOld // if there is no timestamp it always is too old
	}
	if t, err := time.Parse(time.RFC3339Nano, c.TimeStamp); err != nil {
		return err
	} else {
		if time.Since(t).Hours() > 24 { // index.pidx older than 1 day
			return errs.ErrIndexTooOld
		}
	}
	return nil
}

// Read reads the CidxXML from the file specified by c.fileName.
// If the file does not exist, it creates a new CidxXML with default values and writes it to the file.
// It initializes the pdscList map and populates it with cTag entries from the Cindex.Pdscs slice.
// After processing, it truncates the Cindex.Pdscs slice to release memory.
//
// Returns an error if reading the XML file fails or if writing a new file fails.
func (c *CidxXML) Read() error {
	log.Debugf("Reading cidx from file %q", c.fileName)

	c.pdscList = make(map[string][]CacheTag)
	c.pdscListName = make(map[string]string)

	// Create a new empty Cidx file if it does not exist
	if !utils.FileExists(c.fileName) {
		c.SchemaVersion = "1.0.0"
		return errs.ErrFileNotFound
	}
	if err := utils.ReadXML(c.fileName, c); err != nil {
		return err
	}

	for _, pdsc := range c.Cindex.Pdscs {
		key := pdsc.Key()
		name := strings.ToLower(pdsc.VName())
		c.pdscList[key] = append(c.pdscList[key], pdsc)
		c.pdscListName[name] = key
	}

	// truncate Cindex.Pdscs
	c.Cindex.Pdscs = c.Cindex.Pdscs[:0]

	return nil
}

// Write writes the CidxXML data to the file specified by c.fileName.
// It appends the pdsc tags from c.pdscList to c.Pindex.Pdscs before writing,
// and truncates c.Pindex.Pdscs after writing to prepare for potential future writes.
// Returns an error if the writing process fails.
func (c *CidxXML) Write() error {
	log.Debugf("Writing cidx file to %q", c.fileName)

	c.mu.Lock()
	// Use c.pdscList as the main source of pdsc tags
	for _, pdscs := range c.pdscList {
		c.Cindex.Pdscs = append(c.Cindex.Pdscs, pdscs...)
	}
	c.mu.Unlock()

	t := time.Now()
	c.TimeStamp = t.Format(time.RFC3339Nano)
	err := utils.WriteXML(c.fileName, c)

	// Truncate Cindex.Pdscs in case a new Write is requested
	c.mu.Lock()
	c.Cindex.Pdscs = c.Cindex.Pdscs[:0]
	c.mu.Unlock()

	return err
}

// Key generates a unique key for the CacheTag by concatenating the Vendor, Name, and Version fields
// with periods ('.') as separators. The resulting key is in the format "Vendor.Name.Version".
func (c *CacheTag) Key() string {
	return c.Vendor + "." + c.Name + "." + c.Version
}

// VName returns a string that concatenates the Vendor and Name fields
// of the CacheTag struct, separated by a dot.
func (c *CacheTag) VName() string {
	return c.Vendor + "." + c.Name
}

// YamlPackID generates a string that uniquely identifies a pack in the format "Vendor::Name@Version".
// It concatenates the Vendor, Name, and Version fields of the CacheTag struct with "::" and "@" as separators
// as it is specified in
// https://github.com/Open-CMSIS-Pack/devtools/blob/tools/toolbox/0.10.0/tools/projmgr/docs/Manual/YML-Format.md#pack-name-conventions
func (c *CacheTag) YamlPackID() string {
	return c.Vendor + "::" + c.Name + "@" + c.Version
}

// PackURL constructs and returns the full URL of the pack file
// by concatenating the base URL with the key and the ".pack" extension.
func (c *CacheTag) PackURL() string {
	return c.URL + c.VName() + utils.SemverStripMeta(c.Version) + utils.PackExtension
}
