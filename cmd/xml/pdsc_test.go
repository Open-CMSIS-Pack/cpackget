/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml_test

import (
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

func TestPdscXML(t *testing.T) {

	assert := assert.New(t)

	t.Run("test NewPdscXML", func(t *testing.T) {
		var fileName = "somefile.pdsc"
		pdscXML := xml.NewPdscXML(fileName)
		assert.NotNil(pdscXML, "NewPdscXML should not fail on a simple instance creation")
	})

	t.Run("test latest version", func(t *testing.T) {
		var latest string
		pdscXML := xml.PdscXML{
			Vendor: "TheVendor",
			URL:    "http://the.url/",
			Name:   "TheName",
		}

		// It is OK to have an empty LatestVersion() (or is it?)
		latest = pdscXML.LatestVersion()
		assert.Equal(latest, "")

		release1 := xml.ReleaseTag{
			Version: "0.0.1",
		}
		release2 := xml.ReleaseTag{
			Version: "0.0.2",
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release2)
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release1)

		latest = pdscXML.LatestVersion()
		assert.Equal(latest, "0.0.2")
	})

	t.Run("test pdscXML to pdscTag generation", func(t *testing.T) {
		var url = "http://the.url/"
		var name = "TheName"
		var vendor = "TheVendor"
		var version = "0.0.1"
		pdscXML := xml.PdscXML{
			Vendor: vendor,
			URL:    url,
			Name:   name,
		}
		release := xml.ReleaseTag{
			Version: version,
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release)

		pdscTag := pdscXML.Tag()
		assert.Equal(pdscTag.Vendor, vendor)
		assert.Equal(pdscTag.URL, url)
		assert.Equal(pdscTag.Name, name)
		assert.Equal(pdscTag.Version, version)
	})

	t.Run("test reading a PDSC file", func(t *testing.T) {
		pdsc := xml.NewPdscXML("../../testdata/devpack/1.2.3/TheVendor.DevPack.pdsc")
		assert.Nil(pdsc.Read())
		assert.Equal(pdsc.Vendor, "TheVendor")
		assert.Equal(pdsc.URL, "file:///testdata/devpack/1.2.3/")
		assert.Equal(pdsc.Name, "DevPack")
		assert.Equal(pdsc.LatestVersion(), "1.2.3")
	})

	t.Run("test finding release tag", func(t *testing.T) {
		pdsc := xml.NewPdscXML("../../testdata/devpack/1.2.3/TheVendor.DevPack.pdsc")
		assert.Nil(pdsc.Read())
		releaseTag := pdsc.FindReleaseTagByVersion("1.2.3")
		assert.NotNil(releaseTag)
		assert.Equal(releaseTag.Version, "1.2.3")

		releaseTag = pdsc.FindReleaseTagByVersion("")
		assert.NotNil(releaseTag)
		assert.Equal(releaseTag.Version, "1.2.3")
	})
}
