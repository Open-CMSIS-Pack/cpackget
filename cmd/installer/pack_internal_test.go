/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
)

func TestResolveExactVersionPreservesLeadingZeros(t *testing.T) {
	pdsc := xml.NewPdscXML("")
	pdsc.ReleasesTag.Releases = []xml.ReleaseTag{{Version: "26.09.00"}}
	for _, test := range []struct {
		requested string
		expected  string
	}{
		{"26.9.0", "26.09.00"},
		{"26.09.00", "26.09.00"},
		{"26.8.0", "26.8.0"},
	} {
		t.Run(test.requested, func(t *testing.T) {
			pack := &PackType{
				PdscTag:         xml.PdscTag{Version: test.requested},
				versionModifier: utils.ExactVersion,
			}
			pack.resolveVersionModifier(pdsc)
			if pack.targetVersion != test.expected {
				t.Fatalf("resolved version = %q, want %q", pack.targetVersion, test.expected)
			}
		})
	}
}
