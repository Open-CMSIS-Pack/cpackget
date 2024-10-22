/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml_test

import (
	"sort"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
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

		assert.Nil(pdscXML.FindReleaseTagByVersion(""))

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

	t.Run("test list all versions", func(t *testing.T) {
		pdscXML := xml.PdscXML{
			Vendor: "TheVendor",
			URL:    "http://the.url/",
			Name:   "TheName",
		}

		release1 := xml.ReleaseTag{
			Version: "0.0.1",
		}
		release2 := xml.ReleaseTag{
			Version: "0.0.2",
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release2)
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release1)

		allVersions := pdscXML.AllReleases()
		sort.Strings(allVersions)
		expected := []string{"0.0.1", "0.0.2"}
		assert.Equal(expected, allVersions)
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
		assert.Equal(0, utils.SemverCompare(pdsc.LatestVersion(), "1.2.3"))
		assert.Equal("1.2.3+meta3", pdsc.LatestVersion())
	})

	t.Run("test finding release tag", func(t *testing.T) {
		pdsc := xml.NewPdscXML("../../testdata/devpack/1.2.3/TheVendor.DevPack.pdsc")
		assert.Nil(pdsc.Read())
		releaseTag := pdsc.FindReleaseTagByVersion("1.2.3")
		assert.NotNil(releaseTag)
		assert.Equal("1.2.3+meta3", releaseTag.Version)

		releaseTag = pdsc.FindReleaseTagByVersion("1.2.3+meta0")
		assert.NotNil(releaseTag)
		assert.Equal("1.2.3+meta3", releaseTag.Version)

		releaseTag = pdsc.FindReleaseTagByVersion("1.2.2")
		assert.NotNil(releaseTag)
		assert.Equal("1.2.2", releaseTag.Version)

		releaseTag = pdsc.FindReleaseTagByVersion("1.2.2+meta")
		assert.NotNil(releaseTag)
		assert.Equal("1.2.2", releaseTag.Version)

		releaseTag = pdsc.FindReleaseTagByVersion("")
		assert.NotNil(releaseTag)
		assert.Equal("1.2.3+meta3", releaseTag.Version)
	})

	t.Run("test building pack url", func(t *testing.T) {
		var url = "http://the.url"
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

		expectedURL := url + "/" + vendor + "." + name + "." + version + ".pack"
		assert.Equal(expectedURL, pdscXML.PackURL(""))
		assert.Equal(expectedURL, pdscXML.PackURL(version))
	})

	t.Run("test Dependencies1", func(t *testing.T) {
		var name = "TheName"
		var vendor = "TheVendor"
		var version = "0.0.1"
		pdscXML := xml.PdscXML{}
		pack := xml.PackageTag{
			Vendor:  vendor,
			Name:    name,
			Version: version,
		}
		packs := xml.PackagesTag{}
		packs.Packages = append(packs.Packages, pack)
		pdscXML.RequirementsTag.Packages = append(pdscXML.RequirementsTag.Packages, packs)

		expectedDeps := [][]string{}
		expectedDep := []string{name, vendor, version + ":_"}
		expectedDeps = append(expectedDeps, expectedDep)
		assert.Equal(expectedDeps, pdscXML.Dependencies())
	})

	t.Run("test Dependencies2", func(t *testing.T) {
		var name = "TheName"
		var vendor = "TheVendor"
		var version = "0.0.1:0.0.2"
		pdscXML := xml.PdscXML{}
		pack := xml.PackageTag{
			Vendor:  vendor,
			Name:    name,
			Version: version,
		}
		packs := xml.PackagesTag{}
		packs.Packages = append(packs.Packages, pack)
		pdscXML.RequirementsTag.Packages = append(pdscXML.RequirementsTag.Packages, packs)

		expectedDeps := [][]string{}
		expectedDep := []string{name, vendor, version}
		expectedDeps = append(expectedDeps, expectedDep)
		assert.Equal(expectedDeps, pdscXML.Dependencies())
	})

	t.Run("test Dependencies3", func(t *testing.T) {
		var name = "TheName"
		var vendor = "TheVendor"
		var version = ""
		pdscXML := xml.PdscXML{}
		pack := xml.PackageTag{
			Vendor:  vendor,
			Name:    name,
			Version: version,
		}
		packs := xml.PackagesTag{}
		packs.Packages = append(packs.Packages, pack)
		pdscXML.RequirementsTag.Packages = append(pdscXML.RequirementsTag.Packages, packs)

		expectedDeps := [][]string{}
		expectedDep := []string{name, vendor, "latest"}
		expectedDeps = append(expectedDeps, expectedDep)
		assert.Equal(expectedDeps, pdscXML.Dependencies())
	})

	t.Run("test Dependencies4", func(t *testing.T) {
		pdscXML := xml.PdscXML{}

		pdscXML.RequirementsTag.Packages = nil
		assert.Nil(pdscXML.Dependencies())
	})
}
