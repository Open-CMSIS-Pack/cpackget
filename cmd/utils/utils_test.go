/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package utils_test

import (
	//"os"
	"reflect"
	"testing"
)

func AssertEqual(t *testing.T, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Wanted \"%s\", got \"%s\" instead", want, got)
	}
}

func TestPackPackToPdscTag(t *testing.T) {
	/*
		t.Run("test bad pack name", func (t *testing.T) {
			_, err := PackPathToPdscTag("invalid-pack-name")
			AssertEqual(t, err, BadPackName)
		})

		t.Run("test invalid file extension", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendor.Pack.0.0.1.txt")
			AssertEqual(t, err, BadPackNameInvalidExtension)
		})

		t.Run("test invalid vendor", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendo2?r.Pack.0.0.1.pack")
			AssertEqual(t, err, BadPackNameInvalidVendor)
		})

		t.Run("test invalid pack name", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendor.Pack*.0.0.1.pack")
			AssertEqual(t, err, BadPackNameInvalidName)
		})

		t.Run("test invalid version", func (t *testing.T) {
			_, err := PackPathToPdscTag("Vendor.Pack.0.0.1.2.pack")
			AssertEqual(t, err, BadPackNameInvalidVersion)
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
		}) */
}
