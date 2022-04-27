/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
)

var (
	ListCached = true
	ListPublic = true
)

// Listing on empty
func ExampleListInstalledPacks() {
	localTestingDir := "test-list-empty-pack-root"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer os.RemoveAll(localTestingDir)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)

	_ = installer.ListInstalledPacks(!ListCached, !ListPublic)
	// Output:
	// I: Listing installed packs
	// I: (no packs installed)
}

func ExampleListInstalledPacks_emptyCache() {
	localTestingDir := "test-list-empty-cache"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer os.RemoveAll(localTestingDir)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)

	_ = installer.ListInstalledPacks(ListCached, !ListPublic)
	// Output:
	// I: Listing cached packs
	// I: (no packs cached)
}

func ExampleListInstalledPacks_emptyPublicIndex() {
	localTestingDir := "test-list-empty-index"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer os.RemoveAll(localTestingDir)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)

	_ = installer.ListInstalledPacks(ListCached, ListPublic)
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
	defer os.RemoveAll(localTestingDir)

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
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, ListPublic)
	// Output:
	// I: Listing packs from the public index
	// I: TheVendor.PublicLocalPack.1.2.3 (cached)
	// I: TheVendor.PublicLocalPack.1.2.4 (installed)
	// I: TheVendor.PublicLocalPack.1.2.5
}

func ExampleListInstalledPacks_listCached() {
	localTestingDir := "test-list-cached-packs"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer os.RemoveAll(localTestingDir)

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
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(ListCached, !ListPublic)
	// Output:
	// I: Listing cached packs
	// I: TheVendor.PublicLocalPack.1.2.3
	// I: TheVendor.PublicLocalPack.1.2.4 (installed)
}

func ExampleListInstalledPacks_listInstalled() {
	localTestingDir := "test-list-installed-packs"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer os.RemoveAll(localTestingDir)

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
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula)
	_ = installer.AddPack(publicLocalPack124, !CheckEula, !ExtractEula)
	_ = installer.RemovePack("TheVendor.PublicLocalPack.1.2.3", false /*no purge*/)

	// Install a pack via PDSC file
	_ = installer.AddPdsc(pdscPack123)

	log.SetOutput(os.Stdout)
	defer log.SetOutput(ioutil.Discard)
	_ = installer.ListInstalledPacks(!ListCached, !ListPublic)
	// Output:
	// I: Listing installed packs
	// I: TheVendor.PackName.1.2.3
	// I: TheVendor.PublicLocalPack.1.2.4
}

func ExampleListInstalledPacks_listMalformedInstalledPacks() {
	localTestingDir := "test-list-malformed-installed-packs"
	_ = installer.SetPackRoot(localTestingDir, CreatePackRoot)
	defer os.RemoveAll(localTestingDir)

	pdscFilePath := strings.Replace(publicLocalPack123, ".1.2.3.pack", ".pdsc", -1)
	_ = utils.CopyFile(pdscFilePath, filepath.Join(installer.Installation.WebDir, "TheVendor.PublicLocalPack.pdsc"))
	_ = installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
		Vendor:  "TheVendor",
		Name:    "PublicLocalPack",
		Version: "1.2.3",
	})
	_ = installer.AddPack(publicLocalPack123, !CheckEula, !ExtractEula)

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
	_ = installer.ListInstalledPacks(!ListCached, !ListPublic)
	// Output:
	// I: Listing installed packs
	// E: _TheVendor._PublicLocalPack.1.2.3.4 - error: vendor, pack name, pack version incorrect format
	// W: 1 error(s) detected
}
