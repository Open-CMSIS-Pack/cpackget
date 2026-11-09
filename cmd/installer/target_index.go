/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"cmp"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	packxml "github.com/open-cmsis-pack/cpackget/cmd/xml"
)

func targetDirectoryURL(directory string) (string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	path := filepath.ToSlash(absolute)
	host := "localhost"
	if strings.HasPrefix(path, "//") {
		parts := strings.SplitN(strings.TrimPrefix(path, "//"), "/", 2)
		host, path = parts[0], "/"+parts[1]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Host: host, Path: strings.TrimSuffix(path, "/") + "/"}).String(), nil
}

func collectTargetIndex() (*packxml.TidxXML, error) {
	directoryURL, err := targetDirectoryURL(filepath.Join(Installation.PackRoot, ".Targets"))
	if err != nil {
		return nil, err
	}
	index := &packxml.TidxXML{
		SchemaVersion: "1.0.0",
		Vendor:        "target_index",
		URL:           directoryURL + "index.tidx",
	}
	packs, err := findInstalledPacks(false, false)
	if err != nil {
		return nil, err
	}
	latest := make(map[string]installedPack)
	for _, pack := range packs {
		key := pack.Vendor + "::" + pack.Name
		previous, exists := latest[key]
		if !exists || utils.SemverCompare(pack.Version, previous.Version) > 0 {
			latest[key] = pack
		}
	}
	paths := make(map[string]string)
	for _, pack := range latest {
		paths[pack.pdscPath] = pack.Version
	}
	if Installation.LocalPidx != nil && utils.FileExists(Installation.LocalPidx.GetFileName()) {
		localIndex := packxml.NewPidxXML(Installation.LocalPidx.GetFileName(), false)
		if err := localIndex.Read(); err != nil {
			return nil, err
		}
		for _, tag := range localIndex.ListPdscTags() {
			parsed, err := url.Parse(tag.URL)
			if err != nil || parsed.Scheme != "file" {
				return nil, fmt.Errorf("invalid local PDSC URL %q", tag.URL)
			}
			path, err := utils.FileURLToPath(strings.TrimRight(tag.URL, "/") + "/" + tag.VName() + utils.PdscExtension)
			if err != nil {
				return nil, err
			}
			if _, exists := paths[path]; !exists {
				paths[path] = ""
			}
		}
	}
	seen := make(map[string]bool)
	for path, version := range paths {
		pdsc := packxml.NewPdscXML(path)
		if err := pdsc.Read(); err != nil {
			return nil, err
		}
		if version == "" {
			version = pdsc.LatestVersion()
		}
		if pdsc.Vendor == "" || pdsc.Name == "" || utils.SemverMajor(version) == "" {
			return nil, fmt.Errorf("invalid pack identity or version in %q", path)
		}
		directoryURL, err := targetDirectoryURL(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		tag := packxml.PdscTag{Vendor: pdsc.Vendor, Name: pdsc.Name, Version: version, URL: directoryURL}
		key := tag.VName() + "@" + version + ":" + directoryURL
		if seen[key] {
			continue
		}
		seen[key] = true
		for _, device := range pdsc.TargetDevices() {
			index.Devices = append(index.Devices, packxml.TargetDeviceTag{DeviceIdentity: device, Pdsc: tag})
		}
		for _, board := range pdsc.Boards {
			entry := packxml.TargetBoardTag{Vendor: board.Vendor, Name: board.Name, Revision: board.Revision, Pdsc: tag}
			for _, device := range board.MountedDevices {
				entry.MountedDevices = append(entry.MountedDevices, packxml.TargetMountedDeviceTag{DeviceIdentity: packxml.DeviceIdentity{Vendor: strings.SplitN(device.Vendor, ":", 2)[0], Name: device.Name}, Index: device.Index})
			}
			sort.Slice(entry.MountedDevices, func(left, right int) bool {
				return compareMountedDevices(entry.MountedDevices[left], entry.MountedDevices[right]) < 0
			})
			index.Boards = append(index.Boards, entry)
		}
	}
	slices.SortFunc(index.Devices, compareTargetDevices)
	slices.SortFunc(index.Boards, compareTargetBoards)
	return index, nil
}

func compareTargetProviders(left, right packxml.PdscTag) int {
	return cmp.Or(strings.Compare(left.Vendor, right.Vendor), strings.Compare(left.Name, right.Name),
		strings.Compare(left.Version, right.Version), strings.Compare(left.URL, right.URL),
		strings.Compare(left.Deprecated, right.Deprecated), strings.Compare(left.Replacement, right.Replacement))
}

func compareMountedDevices(left, right packxml.TargetMountedDeviceTag) int {
	return cmp.Or(strings.Compare(left.Vendor, right.Vendor), strings.Compare(left.Name, right.Name), strings.Compare(left.Index, right.Index))
}

func compareTargetDevices(left, right packxml.TargetDeviceTag) int {
	return cmp.Or(strings.Compare(left.Vendor, right.Vendor), strings.Compare(left.Name, right.Name), compareTargetProviders(left.Pdsc, right.Pdsc))
}

func compareTargetBoards(left, right packxml.TargetBoardTag) int {
	return cmp.Or(strings.Compare(left.Vendor, right.Vendor), strings.Compare(left.Name, right.Name),
		strings.Compare(left.Revision, right.Revision), compareTargetProviders(left.Pdsc, right.Pdsc),
		slices.CompareFunc(left.MountedDevices, right.MountedDevices, compareMountedDevices))
}

func UpdateTargetIndex() error {
	index, err := collectTargetIndex()
	if err != nil {
		return err
	}
	directory := filepath.Join(Installation.PackRoot, ".Targets")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	contents, err := xml.MarshalIndent(index, "", " ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".index-*.tidx")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(append([]byte(xml.Header), contents...)); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(utils.FileModeRW); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceTargetIndex(temporary.Name(), filepath.Join(directory, "index.tidx"))
}

type TargetOperation struct {
	changed bool
}

func (operation *TargetOperation) Finish(result error) error {
	if !operation.changed {
		return result
	}
	operation.changed = false
	if err := UpdateTargetIndex(); err != nil {
		err = fmt.Errorf("repository changes were applied, but target index refresh failed: %w", err)
		if result == nil {
			return err
		}
		return errors.Join(result, err)
	}
	return result
}
