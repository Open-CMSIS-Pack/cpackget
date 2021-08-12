/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package installer_test

import (
	//"os"
	//"reflect"
	"testing"
)

// Add tests to the following
// - .pack
//   - locally
//     - is public
//     - is no public
//   - from remote server
//     - is public
//     - is not public
// - .zip
//   - locally
//     - is public
//     - is no public
//   - from remote server
//     - is public
//     - is not public

// Remove packs with the following
// - Vendor.Pack
//   - existing pack installation
//   - non existing pack installation
// - Vendor.Pack.x.y.z
//   - existing pack installation
//   - non existing pack installation

func TestPackPackToPdscTag(t *testing.T) {
	/*
		t.Run("test bad pack name", func (t *testing.T) {
			_, err := PackPathToPdscTag("invalid-pack-name")
			AssertEqual(t, err, ErrBadPackName)
		})

		t.Run("test invalid file extension", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendor.Pack.0.0.1.txt")
			AssertEqual(t, err, ErrBadPackNameInvalidExtension)
		})

		t.Run("test invalid vendor", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendo2?r.Pack.0.0.1.pack")
			AssertEqual(t, err, ErrBadPackNameInvalidVendor)
		})

		t.Run("test invalid pack name", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendor.Pack*.0.0.1.pack")
			AssertEqual(t, err, ErrBadPackNameInvalidName)
		})

		t.Run("test invalid version", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendor.Pack.0.0.1.2.pack")
			AssertEqual(t, err, ErrBadPackNameInvalidVersion)
		})

		t.Run("test pdsc as pack path", func (t *testing.T) {
			pdscTag, err := PackPathToPdscTag("/path/to/Vendor.Pack.pdsc")

			AssertEqual(t, err, nil)
			AssertEqual(t, pdscTag.URL, "/path/to/")
			AssertEqual(t, pdscTag.Vendor, "Vendor")
			AssertEqual(t, pdscTag.Name, "Pack")
			AssertEqual(t, pdscTag.Version, "")
		})

		t.Run("test local pack in current directory as pack path", func (t *testing.T) {
			pdscTag, err := PackPathToPdscTag("Vendor.Pack.0.0.1.pack")

			url, _ := os.Getwd()
			AssertEqual(t, err, nil)
			AssertEqual(t, pdscTag.URL, url)
			AssertEqual(t, pdscTag.Vendor, "Vendor")
			AssertEqual(t, pdscTag.Name, "Pack")
			AssertEqual(t, pdscTag.Version, "0.0.1")
		})

		t.Run("test local pack as pack path", func (t *testing.T) {
			pdscTag, err := PackPathToPdscTag("/path/to/Vendor.Pack.0.0.1.pack")

			AssertEqual(t, err, nil)
			AssertEqual(t, pdscTag.URL, "/path/to/")
			AssertEqual(t, pdscTag.Vendor, "Vendor")
			AssertEqual(t, pdscTag.Name, "Pack")
			AssertEqual(t, pdscTag.Version, "0.0.1")
		})

		t.Run("test url pack as pack path", func (t *testing.T) {
			pdscTag, err := PackPathToPdscTag("http://site.com/Vendor.Pack.0.0.1.pack")

			AssertEqual(t, err, nil)
			AssertEqual(t, pdscTag.URL, "http://site.com/")
			AssertEqual(t, pdscTag.Vendor, "Vendor")
			AssertEqual(t, pdscTag.Name, "Pack")
			AssertEqual(t, pdscTag.Version, "0.0.1")
		})

		t.Run("test local zip as pack path", func (t *testing.T) {
			pdscTag, err := PackPathToPdscTag("/path/to/Vendor.Pack.0.0.1.zip")

			AssertEqual(t, err, nil)
			AssertEqual(t, pdscTag.URL, "/path/to/")
			AssertEqual(t, pdscTag.Vendor, "Vendor")
			AssertEqual(t, pdscTag.Name, "Pack")
			AssertEqual(t, pdscTag.Version, "0.0.1")
		})*/
}
