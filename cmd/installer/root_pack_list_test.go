/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	ListCached = true
	ListPublic = true
	ListFilter = ""
)

// Listing on empty
func ExampleListInstalledPacks() {
	localTestingDir := "test-list-empty-pack-root"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer removePackRoot(localTestingDir)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)

	_ = installer.ListInstalledPacks(!ListCached, !ListPublic, ListFilter)
	// Output:
	// I: Listing installed packs
	// I: (no packs installed)
}

func ExampleListInstalledPacks_emptyCache() {
	localTestingDir := "test-list-empty-cache"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer removePackRoot(localTestingDir)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)

	_ = installer.ListInstalledPacks(ListCached, !ListPublic, ListFilter)
	// Output:
	// I: Listing cached packs
	// I: (no packs cached)
}

func ExampleListInstalledPacks_emptyPublicIndex() {
	localTestingDir := "test-list-empty-index"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer removePackRoot(localTestingDir)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)

	_ = installer.ListInstalledPacks(ListCached, ListPublic, ListFilter)
	// Output:
	// I: Listing packs from the public index
	// I: (no packs in public index)
}

// Now list 3 packs from the public index, where
// * 1 is cached only
// * 1 is installed
// * 1 is neither installer or cached, it's just available in the public index
func ExampleListInstalledPacks_list() {
	localTestingDir := "test-list-packs"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.4",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.5",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/, Timeout)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, ListPublic, ListFilter)
	// Output:
	// I: Listing packs from the public index
	// I: TheVendor::PublicLocalPack@1.2.3 (cached)
	// I: TheVendor::PublicLocalPack@1.2.4 (installed)
	// I: TheVendor::PublicLocalPack@1.2.5
}

func ExampleListInstalledPacks_listCached() {
	localTestingDir := "test-list-cached-packs"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.4",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.5",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/, Timeout)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, !ListPublic, ListFilter)
	// Output:
	// I: Listing cached packs
	// I: TheVendor::PublicLocalPack@1.2.3
	// I: TheVendor::PublicLocalPack@1.2.4 (installed)
}

func TestListInstalledPacks(t *testing.T) {
	assert := assert.New(t)

	t.Run("test listing all installed packs", func(t *testing.T) {
		localTestingDir := "test-list-installed-packs"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
		assert.Nil(utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc")))
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
			Vendor:  "TheVendor",
			Name:    "PublicLocalPack",
			Version: "1.2.3",
		}))
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
			Vendor:  "TheVendor",
			Name:    "PublicLocalPack",
			Version: "1.2.4",
		}))
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
			Vendor:  "TheVendor",
			Name:    "PublicLocalPack",
			Version: "1.2.5",
		}))
		assert.Nil(installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout))
		assert.Nil(installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula, !ForceReinstall, Timeout))
		assert.Nil(installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/, Timeout))

		// Install a pack via PDSC file
		assert.Nil(installer.AddPdsc(pdscPack123))

		expectedPdscAbsPath, err := os.Getwd()
		assert.Nil(err)
		expectedPdscAbsPath = utils.CleanPath(filepath.Join(expectedPdscAbsPath, pdscPack123))

		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(ioutil.Discard)
		assert.Nil(installer.ListInstalledPacks(!ListCached, !ListPublic, ListFilter))
		stdout := buf.String()
		assert.Contains(stdout, "I: Listing installed packs")
		assert.Contains(stdout, fmt.Sprintf("I: TheVendor::PackName@1.2.3 (installed via %s)", expectedPdscAbsPath))
		assert.Contains(stdout, "I: TheVendor::PublicLocalPack@1.2.4")
	})

	t.Run("test listing local packs with updated version", func(t *testing.T) {
		localTestingDir := "test-list-installed-local-packs-with-updated-version"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// This test checks that cpackget lists whatever the latest version is in the actual PDSC file
		// for a local pack installed via PDSC
		pdscPath := filepath.Base(pdscPack123)
		defer os.Remove(pdscPath)

		assert.Nil(utils.CopyFile(pdscPack123, pdscPath))

		// Install a pack via PDSC file
		assert.Nil(installer.AddPdsc(pdscPath))

		expectedPdscAbsPath, err := os.Getwd()
		assert.Nil(err)
		expectedPdscAbsPath = utils.CleanPath(filepath.Join(expectedPdscAbsPath, pdscPath))

		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(ioutil.Discard)
		assert.Nil(installer.ListInstalledPacks(!ListCached, !ListPublic, ListFilter))
		stdout := buf.String()
		assert.Contains(stdout, "I: Listing installed packs")
		assert.Contains(stdout, fmt.Sprintf("I: TheVendor::PackName@1.2.3 (installed via %s)", expectedPdscAbsPath))

		// Now update the version inside the PDSC file and expect it to be listed
		pdscXML := xml.NewPdscXML(pdscPath)
		assert.Nil(pdscXML.Read())
		pdscXML.ReleasesTag.Releases[0].Version = "1.2.4"
		assert.Nil(utils.WriteXML(pdscPath, pdscXML))
		assert.Nil(installer.ListInstalledPacks(!ListCached, !ListPublic, ListFilter))
		stdout = buf.String()
		assert.Contains(stdout, "I: Listing installed packs")
		assert.Contains(stdout, fmt.Sprintf("I: TheVendor::PackName@1.2.4 (installed via %s)", expectedPdscAbsPath))
	})
}

func ExampleListInstalledPacks_listMalformedInstalledPacks() {
	localTestingDir := "test-list-malformed-installed-packs"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

	// Temper with the installation folder
	currVendorFolder := filepath.Join(localTestingDir, "TheVendor")
	currPackNameFolder := filepath.Join(localTestingDir, "TheVendor", "PublicLocalPack")
	currVersionFolder := filepath.Join(localTestingDir, "TheVendor", "PublicLocalPack", "1.2.3")

	temperedVendorFolder := filepath.Join(localTestingDir, "_TheVendor")
	temperedPackNameFolder := filepath.Join(localTestingDir, "TheVendor", "_PublicLocalPack")
	temperedVersionFolder := filepath.Join(localTestingDir, "TheVendor", "PublicLocalPack", "1.2.3.4")

	// Order matters
	_ = utils.MoveFile(currVersionFolder, temperedVersionFolder)
	_ = utils.MoveFile(currPackNameFolder, temperedPackNameFolder)
	_ = utils.MoveFile(currVendorFolder, temperedVendorFolder)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(!ListCached, !ListPublic, ListFilter)
	// Output:
	// I: Listing installed packs
	// E: _TheVendor::_PublicLocalPack@1.2.3.4 - error: vendor, pack name, pack version incorrect format
	// W: 1 error(s) detected
}

func ExampleListInstalledPacks_filter() {
	localTestingDir := "test-list-packs-filter"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.4",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.5",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/, Timeout)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, ListPublic, "1.2.4")
	// Output:
	// I: Listing packs from the public index, filtering by "1.2.4"
	// I: TheVendor::PublicLocalPack@1.2.4 (installed)
}

func ExampleListInstalledPacks_filterErrorPackages() {
	localTestingDir := "test-list-filter-error-message"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

	// Temper with the installation folder
	currVendorFolder := filepath.Join(localTestingDir, "TheVendor")
	currPackNameFolder := filepath.Join(localTestingDir, "TheVendor", "PublicLocalPack")
	currVersionFolder := filepath.Join(localTestingDir, "TheVendor", "PublicLocalPack", "1.2.3")

	temperedVendorFolder := filepath.Join(localTestingDir, "_TheVendor")
	temperedPackNameFolder := filepath.Join(localTestingDir, "TheVendor", "_PublicLocalPack")
	temperedVersionFolder := filepath.Join(localTestingDir, "TheVendor", "PublicLocalPack", "1.2.3.4")

	// Order matters
	_ = utils.MoveFile(currVersionFolder, temperedVersionFolder)
	_ = utils.MoveFile(currPackNameFolder, temperedPackNameFolder)
	_ = utils.MoveFile(currVendorFolder, temperedVendorFolder)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(!ListCached, !ListPublic, "TheVendor")
	// Output:
	// I: Listing installed packs, filtering by "TheVendor"
	// E: _TheVendor::_PublicLocalPack@1.2.3.4 - error: vendor, pack name, pack version incorrect format
}

func ExampleListInstalledPacks_filterInvalidChars() {
	localTestingDir := "test-list-filter-invalid-chars"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.4",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.5",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/, Timeout)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, ListPublic, "@ :")
	// Output:
	// I: Listing packs from the public index, filtering by "@ :"
}

func ExampleListInstalledPacks_filteradditionalMessages() {
	localTestingDir := "test-list-filter-additional-messages"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	installer.UnlockPackRoot()
	defer removePackRoot(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.4",
	})
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.5",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/, Timeout)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, !ListPublic, "(installed)")
	// Output:
	// I: Listing cached packs, filtering by "(installed)"
}
