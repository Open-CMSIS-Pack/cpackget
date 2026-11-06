/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package xml

import "encoding/xml"

type TidxXML struct {
	XMLName       xml.Name          `xml:"index"`
	SchemaVersion string            `xml:"schemaVersion,attr"`
	Vendor        string            `xml:"vendor"`
	URL           string            `xml:"url"`
	Devices       []TargetDeviceTag `xml:"devices>device"`
	Boards        []TargetBoardTag  `xml:"boards>board"`
}

type TargetDeviceTag struct {
	DeviceIdentity
	Pdsc PdscTag `xml:"pdsc"`
}

type TargetMountedDeviceTag struct {
	DeviceIdentity
	Index string `xml:"index,attr,omitempty"`
}

type TargetBoardTag struct {
	Vendor         string                   `xml:"vendor,attr"`
	Name           string                   `xml:"name,attr"`
	Revision       string                   `xml:"revision,attr,omitempty"`
	Pdsc           PdscTag                  `xml:"pdsc"`
	MountedDevices []TargetMountedDeviceTag `xml:"mountedDevice"`
}
