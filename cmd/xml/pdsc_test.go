/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package main

import (
	"fmt"
	"testing"
)

func TestPdscXML(t *testing.T) {
	t.Run("test match pdsc tag with equal file content", func(t *testing.T) {
		pdscTag := PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://the.url/",
			Name:    "TheName",
			Version: "0.0.1",
		}

		pdscXML := PdscXML{
			Vendor: "TheVendor",
			URL:    "http://the.url/",
			Name:   "TheName",
		}
		release := ReleaseTag{
			Version: "0.0.1",
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release)

		err := pdscXML.MatchTag(pdscTag)
		if err != nil {
			t.Errorf("MatchTag should not return error on matching tag: %s", err)
		}
	})

	t.Run("test match pdsc tag with different file content", func(t *testing.T) {
		pdscTag := PdscTag{
			Vendor:  "TheVendor",
			URL:     "http://the.url/",
			Name:    "TheName",
			Version: "0.0.1",
		}

		pdscXML := PdscXML{
			Vendor: "TheVendor2",
			URL:    "http://the.url2/",
			Name:   "TheName2",
		}
		release := ReleaseTag{
			Version: "0.0.2",
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release)

		err := pdscXML.MatchTag(pdscTag)
		expected := fmt.Sprintf("Pdsc tag '%s%s' does not match the actual file:", pdscTag.URL, pdscTag.Name)
		expected += fmt.Sprintf(" Vendor('%s' != '%s')", pdscXML.Vendor, pdscTag.Vendor)
		expected += fmt.Sprintf(" URL('%s' != '%s')", pdscXML.URL, pdscTag.URL)
		expected += fmt.Sprintf(" Name('%s' != '%s')", pdscXML.Name, pdscTag.Name)
		expected += fmt.Sprintf(" Version('%s' != '%s')", pdscXML.LatestVersion(), pdscTag.Version)

		AssertEqual(t, err.Error(), expected)
	})

	t.Run("test latest version", func(t *testing.T) {
		var latest string
		pdscXML := PdscXML{
			Vendor: "TheVendor",
			URL:    "http://the.url/",
			Name:   "TheName",
		}

		latest = pdscXML.LatestVersion()
		AssertEqual(t, latest, "")

		release1 := ReleaseTag{
			Version: "0.0.1",
		}
		release2 := ReleaseTag{
			Version: "0.0.2",
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release2)
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release1)

		latest = pdscXML.LatestVersion()
		AssertEqual(t, latest, "0.0.2")
	})

	t.Run("test pdscXML to pdscTag generation", func(t *testing.T) {
		pdscXML := PdscXML{
			Vendor: "TheVendor",
			URL:    "http://the.url/",
			Name:   "TheName",
		}
		release := ReleaseTag{
			Version: "0.0.1",
		}
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, release)

		pdscTag := pdscXML.Tag()
		err := pdscXML.MatchTag(pdscTag)
		if err != nil {
			t.Errorf("MatchTag should not return error on matching tag: %s", err)
		}
	})
}
