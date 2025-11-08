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

	t.Run("test GetFileName returns correct file name", func(t *testing.T) {
		fileName := "testfile.pidx"
		pidx := xml.NewPidxXML(fileName)
		assert.Equal(fileName, pidx.GetFileName(), "GetFileName should return the file name set at initialization")

		// Change file name and check again
		newFileName := "newfile.pidx"
		pidx.SetFileName(newFileName)
		assert.Equal(newFileName, pidx.GetFileName(), "GetFileName should return the updated file name after SetFileName")
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

		// pdscTag2DiffURL := xml.PdscTag{
		// 	Vendor:  "TheVendor",
		// 	URL:     "http://different-url.com/",
		// 	Name:    "ThePack",
		// 	Version: "0.0.2",
		// }

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		// Adding first time is OK
		assert.Nil(pidx.AddPdsc(pdscTag1))

		// Adding an existing one is not OK
		assert.Equal(pidx.AddPdsc(pdscTag1), errs.ErrPdscEntryExists)

		// Adding a second PDSC tag is OK
		assert.Nil(pidx.AddPdsc(pdscTag2))

		// Adding a PDSC of a Pack with different URL is also OK
		//		assert.Nil(pidx.AddPdsc(pdscTag2DiffURL))
	})

	t.Run("test ReplacePdscVersion replaces version successfully", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		originalTag := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "1.0.0",
		}
		newVersionTag := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "2.0.0",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		// Add the original tag
		assert.Nil(pidx.AddPdsc(originalTag))

		// Replace version
		assert.Nil(pidx.ReplacePdscVersion(newVersionTag))

		// The old version should not be found
		assert.Equal(pidx.HasPdsc(originalTag), xml.PdscIndexNotFound)

		// The new version should be found
		assert.GreaterOrEqual(pidx.HasPdsc(newVersionTag), 0)

		// The tag in the list should have the new version
		foundTags := pidx.FindPdscTags(newVersionTag)
		assert.Equal(1, len(foundTags))
		assert.Equal("2.0.0", foundTags[0].Version)
	})

	t.Run("test ReplacePdscVersion returns error if tag not found", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		nonExistentTag := xml.PdscTag{
			Vendor:  "NoVendor",
			URL:     "http://novendor.com/",
			Name:    "NoPack",
			Version: "1.0.0",
		}

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		// Attempt to replace version for a tag that doesn't exist
		err := pidx.ReplacePdscVersion(nonExistentTag)
		assert.Equal(err, errs.ErrPdscEntryNotFound)
	})

	t.Run("test Empty returns true for new PidxXML", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pidx := xml.NewPidxXML(fileName)
		assert.True(pidx.Empty(), "Empty should return true for a new PidxXML with no pdsc tags")
	})

	t.Run("test Empty returns false after adding PdscTag", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		pdscTag := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}
		assert.Nil(pidx.AddPdsc(pdscTag))
		assert.False(pidx.Empty(), "Empty should return false after adding a PdscTag")
	})

	t.Run("test Empty returns true after removing all PdscTags", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pidx := xml.NewPidxXML(fileName)
		assert.Nil(pidx.Read())

		pdscTag := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}
		assert.Nil(pidx.AddPdsc(pdscTag))
		assert.False(pidx.Empty(), "Empty should return false after adding a PdscTag")
		assert.Nil(pidx.RemovePdsc(pdscTag))
		assert.True(pidx.Empty(), "Empty should return true after removing all PdscTags")
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

	// TODO: this test does not work because multiple versions of the same pack are not supported in index.pidx
	t.Run("test removing a PDSC tag without version from a PIDX file", func(t *testing.T) {
		fileName := utils.RandStringBytes(10) + ".pidx"
		defer os.Remove(fileName)

		pdscTag1 := xml.PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://vendor.com/",
			Name:    "ThePack",
			Version: "0.0.1",
		}

		// pdscTag2 := xml.PdscTag{
		// 	Vendor:  "TheVendor",
		// 	URL:     "http://vendor.com/",
		// 	Name:    "ThePack",
		// 	Version: "0.0.2",
		// }

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
		// assert.Nil(pidx.AddPdsc(pdscTag2))
		assert.Nil(pidx.AddPdsc(pdscTagThatShouldRemain))

		// Removing a PDSC tag without version should remove all PDSC tags that match TheVendor.ThePack
		assert.Nil(pidx.RemovePdsc(pdscTag2WithoutVersion))

		// Make sure it really got removed
		assert.Equal(pidx.RemovePdsc(pdscTag1), errs.ErrPdscEntryNotFound)
		// assert.Equal(pidx.RemovePdsc(pdscTag2), errs.ErrPdscEntryNotFound)
	})

	t.Run("test writing changes to a PIDX file", func(t *testing.T) {
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
