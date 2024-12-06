/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml_test

import (
	"os"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

func TestPdscTag(t *testing.T) {

	assert := assert.New(t)

	t.Run("test PdscTag Key", func(t *testing.T) {
		pdscTag := xml.PdscTag{
			Vendor:  "TheVendor",
			Name:    "ThePack",
			Version: "0.0.1",
			URL:     "http://vendor.com/",
		}

		assert.Equal(pdscTag.Key(), "TheVendor.ThePack.0.0.1")
	})

	t.Run("test PdscTag YamlPackID", func(t *testing.T) {
		pdscTag := xml.PdscTag{
			Vendor:  "TheVendor",
			Name:    "ThePack",
			Version: "0.0.1",
			URL:     "http://vendor.com/",
		}

		assert.Equal(pdscTag.YamlPackID(), "TheVendor::ThePack@0.0.1")
	})
}

func TestPidxXML(t *testing.T) {

	assert := assert.New(t)

	t.Run("test NewPidxXML", func(t *testing.T) {
		var fileName = "somefile.pidx"
		pidx := xml.NewPidxXML(fileName)
		assert.NotNil(pidx, "NewPidxXML should not fail on a simple instance creation")
	})

	t.Run("test adding a PDSC tag to a PIDX file", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pdscTag1 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}

		pdscTag2 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.2",
		}

		pdscTag2DiffURL := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://different-url.com/",
			Name:    "ThePack",
			Version: "0.0.2",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		// Adding first time is OK
		assert.Nil(pidx.AddPdsc(pdscTag1))

		// Adding an existing one is not OK
		assert.Equal(pidx.AddPdsc(pdscTag1), errs.ErrPdscEntryExists)

		// Adding a second PDSC tag is OK
		assert.Nil(pidx.AddPdsc(pdscTag2))

		// Adding a PDSC of a Pack with different URL is also OK
		assert.Nil(pidx.AddPdsc(pdscTag2DiffURL))
	})

	t.Run("test removing a PDSC tag from a PIDX file", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pdscTag1 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(utils.WriteXML(fileName, &pidx))
		assert.Nil(pidx.Read())

		// Removing an non-existing PDSC tag is not OK
		assert.Equal(pidx.RemovePdsc(pdscTag1), errs.ErrPdscEntryNotFound)

		// Adding first time is OK
		assert.Nil(pidx.AddPdsc(pdscTag1))

		// Removing is OK
		assert.Nil(pidx.RemovePdsc(pdscTag1))

		// Make sure it really got removed
		assert.Equal(pidx.RemovePdsc(pdscTag1), errs.ErrPdscEntryNotFound)
	})

	t.Run("test removing a PDSC tag without version from a PIDX file", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pdscTag1 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}

		pdscTag2 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.2",
		}

		pdscTag2WithoutVersion := xml.PdscTag{
			Vendor: "TheVendor",
			URL:    "http://vendor.com/",
			Name:   "ThePack",
		}

		pdscTagThatShouldRemain := xml.PdscTag{
			Vendor:  "TheOtherVendor",
			URL:     "http://theothervendor.com/",
			Name:    "TheOtherPack",
			Version: "0.0.1",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(utils.WriteXML(fileName, &pidx))
		assert.Nil(pidx.Read())

		// Make sure that an attempt to remove a PDSC tag without version also raises an error
		assert.Equal(pidx.RemovePdsc(pdscTag2WithoutVersion), errs.ErrPdscEntryNotFound)

		// Populating tags
		assert.Nil(pidx.AddPdsc(pdscTag1))
		assert.Nil(pidx.AddPdsc(pdscTag2))
		assert.Nil(pidx.AddPdsc(pdscTagThatShouldRemain))

		// Removing a PDSC tag without version should remove all PDSC tags that match TheVendor.ThePack
		assert.Nil(pidx.RemovePdsc(pdscTag2WithoutVersion))

		// Make sure it really got removed
		assert.Equal(pidx.RemovePdsc(pdscTag1), errs.ErrPdscEntryNotFound)
		assert.Equal(pidx.RemovePdsc(pdscTag2), errs.ErrPdscEntryNotFound)
	})

	t.Run("test writting changes to a PIDX file", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pdscTag1 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}

		pdscTag2 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.2",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(utils.WriteXML(fileName, &pidx))
		assert.Nil(pidx.Read())

		// Populates the file
		assert.Nil(pidx.AddPdsc(pdscTag1))
		assert.Nil(pidx.AddPdsc(pdscTag2))

		// Make sure it writes OK
		assert.Nil(pidx.Write())
		newPidx := xml.NewPidxXML(fileName)
		assert.Nil(newPidx.Read())
		assert.Greater(newPidx.HasPdsc(pdscTag1), xml.PdscIndexNotFound)
		assert.Greater(newPidx.HasPdsc(pdscTag2), xml.PdscIndexNotFound)
	})

	t.Run("test reading PIDX file with malformed XML", func(t *testing.T) {
		pidx := xml.NewPidxXML("../../testdata/MalformedPack.pidx")
		err := pidx.Read()
		assert.NotNil(err)
		assert.Equal(err.Error(), "XML syntax error on line 3: unexpected EOF")
	})

	t.Run("test check public index file for old timestamp", func(t *testing.T) {
		pidx := xml.NewPidxXML("../../testdata/OldTimestamp.pidx")
		err := pidx.CheckTime()
		assert.NotNil(err)
		assert.Equal(err, errs.ErrIndexTooOld)
	})

	t.Run("test check public index file for new timestamp", func(t *testing.T) {
		pidx := xml.NewPidxXML("../../testdata/NewTimestamp.pidx")
		err := pidx.CheckTime()
		assert.Nil(err)
	})

	t.Run("test finding pdsc tag", func(t *testing.T) {
		fileName := "test-finding-pdsc-tag.pidx"
		defer os.Remove(fileName)

		pdscTag1 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}

		pdscTag2 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.2",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		// Find with empty version
		assert.Nil(pidx.AddPdsc(pdscTag1))
		pdscTag1.Version = ""
		foundTags := pidx.FindPdscTags(pdscTag1)
		assert.Greater(len(foundTags), 0)
		assert.Equal(foundTags[0].Version, "0.0.1")

		// Find with specified version
		assert.Nil(pidx.AddPdsc(pdscTag2))
		foundTags = pidx.FindPdscTags(pdscTag2)
		assert.Greater(len(foundTags), 0)
		assert.Equal(foundTags[0], pdscTag2)

	})
}
