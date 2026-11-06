/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetDevices(t *testing.T) {
	var pdsc PdscXML
	err := xml.Unmarshal([]byte(`<package><devices><family Dfamily="Family" Dvendor="Vendor:123"><subFamily DsubFamily="Sub"><device Dname="Base"><variant Dvariant="ChipA"/><variant Dvariant="ChipB" Dvendor="Other:456"/></device><device Dname="ChipC"/></subFamily></family></devices><boards><board vendor="BoardVendor" name="Board" revision="A"><mountedDevice Dvendor="Vendor:123" Dname="ChipA" deviceIndex="0"/></board></boards></package>`), &pdsc)
	require.NoError(t, err)
	require.Equal(t, []DeviceIdentity{{Vendor: "Vendor", Name: "ChipA"}, {Vendor: "Other", Name: "ChipB"}, {Vendor: "Vendor", Name: "ChipC"}}, pdsc.TargetDevices())
	require.Len(t, pdsc.Boards, 1)
	require.Equal(t, "0", pdsc.Boards[0].MountedDevices[0].Index)
}
