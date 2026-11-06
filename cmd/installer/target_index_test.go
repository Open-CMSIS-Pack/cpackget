/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"syscall"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/require"
)

func TestUpdateTargetIndex(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	root := t.TempDir()
	Installation = &PacksInstallationType{PackRoot: root, LocalPidx: xml.NewPidxXML(filepath.Join(root, ".Local", "local_repository.pidx"), false)}
	writePdsc := func(directory, name, contents string) string {
		require.NoError(t, os.MkdirAll(directory, 0755))
		path := filepath.Join(directory, name)
		require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
		return path
	}
	writePdsc(filepath.Join(root, "Vendor", "DFP", "1.0.0"), "Vendor.DFP.pdsc", `<package><vendor>Vendor</vendor><name>DFP</name><devices><family Dvendor="ChipVendor:1"><device Dname="Old"/></family></devices></package>`)
	latestPath := writePdsc(filepath.Join(root, "Vendor", "DFP", "2.0.0"), "Vendor.DFP.pdsc", `<package><vendor>Vendor</vendor><name>DFP</name><devices><family Dvendor="ChipVendor:1"><device Dname="New"/></family></devices></package>`)
	writePdsc(filepath.Join(root, ".Web"), "Vendor.Cached.pdsc", `<package><vendor>Vendor</vendor><name>Cached</name></package>`)
	local := t.TempDir()
	writePdsc(local, "Vendor.BSP.pdsc", `<package><vendor>Vendor</vendor><name>BSP</name><releases><release version="1.0.0"/></releases><boards><board vendor="Vendor" name="Board"><mountedDevice Dvendor="ChipVendor:1" Dname="New" deviceIndex="1"/><mountedDevice Dvendor="ChipVendor:1" Dname="New" deviceIndex="0"/></board></boards></package>`)
	localURL, err := targetDirectoryURL(local)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".Local"), 0755))
	require.NoError(t, Installation.LocalPidx.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "BSP", Version: "1.0.0", URL: localURL}))
	require.NoError(t, Installation.LocalPidx.Write())
	require.NoError(t, UpdateTargetIndex())
	path := filepath.Join(root, ".Targets", "index.tidx")
	var index xml.TidxXML
	require.NoError(t, utils.ReadXML(path, &index))
	require.Len(t, index.Devices, 1)
	require.Equal(t, "New", index.Devices[0].Name)
	require.Equal(t, "2.0.0", index.Devices[0].Pdsc.Version)
	require.Len(t, index.Boards, 1)
	require.Len(t, index.Boards[0].MountedDevices, 2)
	require.Equal(t, "0", index.Boards[0].MountedDevices[0].Index)
	require.Equal(t, "1", index.Boards[0].MountedDevices[1].Index)
	require.Equal(t, "ChipVendor", index.Boards[0].MountedDevices[0].Vendor)
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(before), "<vendor>target_index</vendor>")
	require.Contains(t, string(before), "<url></url>")
	require.Equal(t, "1.0.0", index.SchemaVersion)
	require.Equal(t, "Vendor", index.Devices[0].Pdsc.Vendor)
	require.Equal(t, "DFP", index.Devices[0].Pdsc.Name)
	require.NotEmpty(t, index.Devices[0].Pdsc.URL)
	require.NoError(t, UpdateTargetIndex())
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoError(t, os.WriteFile(latestPath, []byte(`<package>`), 0600))
	require.Error(t, UpdateTargetIndex())
	after, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestTargetIndexInvalidLocalRepository(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	for _, test := range []struct {
		name     string
		url      string
		contents string
	}{
		{name: "malformed local index"},
		{name: "remote URL", url: "https://example.com/"},
		{name: "missing PDSC", url: "local"},
		{name: "missing identity", url: "local", contents: `<package><releases><release version="1.0.0"/></releases></package>`},
		{name: "invalid version", url: "local", contents: `<package><vendor>Vendor</vendor><name>DFP</name><releases><release version="invalid"/></releases></package>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			Installation = &PacksInstallationType{PackRoot: root, LocalPidx: xml.NewPidxXML(filepath.Join(root, ".Local", "local_repository.pidx"), false)}
			require.NoError(t, UpdateTargetIndex())
			indexPath := filepath.Join(root, ".Targets", "index.tidx")
			before, err := os.ReadFile(indexPath)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Join(root, ".Local"), 0755))
			if test.url == "" {
				require.NoError(t, os.WriteFile(Installation.LocalPidx.GetFileName(), []byte(`<index>`), 0600))
			} else {
				address := test.url
				if address == "local" {
					directory := t.TempDir()
					address, err = targetDirectoryURL(directory)
					require.NoError(t, err)
					if test.contents != "" {
						require.NoError(t, os.WriteFile(filepath.Join(directory, "Vendor.DFP.pdsc"), []byte(test.contents), 0600))
					}
				}
				require.NoError(t, Installation.LocalPidx.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "DFP", URL: address}))
				require.NoError(t, Installation.LocalPidx.Write())
			}
			require.Error(t, UpdateTargetIndex())
			after, err := os.ReadFile(indexPath)
			require.NoError(t, err)
			require.Equal(t, before, after)
			files, err := filepath.Glob(filepath.Join(root, ".Targets", ".index-*.tidx"))
			require.NoError(t, err)
			require.Empty(t, files)
		})
	}
}

func TestEmptyTargetIndex(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	root := t.TempDir()
	Installation = &PacksInstallationType{PackRoot: root}
	require.NoError(t, UpdateTargetIndex())
	contents, err := os.ReadFile(filepath.Join(root, ".Targets", "index.tidx"))
	require.NoError(t, err)
	require.Contains(t, string(contents), "<devices></devices>")
	require.Contains(t, string(contents), "<boards></boards>")
	require.Contains(t, string(contents), "<vendor>target_index</vendor>")
	require.Contains(t, string(contents), "<url></url>")
}

func TestTargetDirectoryURLRoundTrip(t *testing.T) {
	directories := []string{filepath.Join(t.TempDir(), "local packs #100%")}
	if runtime.GOOS == "windows" {
		directories = append(directories, `\\server\share\local packs #100%`)
	}
	for _, directory := range directories {
		address, err := targetDirectoryURL(directory)
		require.NoError(t, err)
		path, err := utils.FileURLToPath(address)
		require.NoError(t, err)
		require.Equal(t, filepath.Clean(directory), filepath.Clean(path))
	}
}

func TestTargetOperation(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	root := t.TempDir()
	Installation = &PacksInstallationType{PackRoot: root}
	operation := &TargetOperation{}
	original := errors.New("mutation failed")
	require.ErrorIs(t, operation.Finish(original), original)
	require.NoDirExists(t, filepath.Join(root, ".Targets"))
	operation.changed = true
	require.ErrorIs(t, operation.Finish(original), original)
	path := filepath.Join(root, ".Targets", "index.tidx")
	require.FileExists(t, path)
	require.NoError(t, os.Remove(path))
	require.NoError(t, operation.Finish(nil))
	require.NoFileExists(t, path)
	require.NoError(t, os.Mkdir(path, 0755))
	operation.changed = true
	err := operation.Finish(original)
	require.ErrorIs(t, err, original)
	require.ErrorContains(t, err, "repository changes were applied")
	files, err := filepath.Glob(filepath.Join(root, ".Targets", ".index-*.tidx"))
	require.NoError(t, err)
	require.Empty(t, files)
	operation.changed = true
	require.ErrorContains(t, operation.Finish(nil), "repository changes were applied")
}

func TestTargetOperationPartialRemoval(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	for _, cleanupFailure := range []bool{false, true} {
		for _, refreshFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("cleanupFailure=%t/refreshFailure=%t", cleanupFailure, refreshFailure), func(t *testing.T) {
				root := t.TempDir()
				Installation = &PacksInstallationType{PackRoot: root, LocalDir: filepath.Join(root, ".Local")}
				path := filepath.Join(root, "Vendor", "DFP", "1.0.0", "Vendor.DFP.pdsc")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, []byte(`<package><vendor>Vendor</vendor><name>DFP</name><devices><family Dvendor="Vendor"><device Dname="Chip"/></family></devices></package>`), 0600))
				require.NoError(t, UpdateTargetIndex())
				indexPath := filepath.Join(root, ".Targets", "index.tidx")
				before, err := os.ReadFile(indexPath)
				require.NoError(t, err)
				if refreshFailure {
					bad := filepath.Join(root, "Other", "DFP", "1.0.0", "Other.DFP.pdsc")
					require.NoError(t, os.MkdirAll(filepath.Dir(bad), 0755))
					require.NoError(t, os.WriteFile(bad, []byte(`<package>`), 0600))
				}
				pack := &PackType{PdscTag: xml.PdscTag{Vendor: "Vendor", Name: "DFP", Version: "1.0.0"}, IsPublic: !cleanupFailure}
				operation := &TargetOperation{}
				err = pack.uninstallWithOperation(Installation, operation)
				if cleanupFailure {
					require.ErrorIs(t, err, os.ErrNotExist)
				} else {
					require.NoError(t, err)
				}
				require.True(t, operation.changed)
				original := err
				err = operation.Finish(err)
				if original != nil {
					require.ErrorIs(t, err, original)
				}
				if refreshFailure {
					require.ErrorContains(t, err, "repository changes were applied")
					after, readErr := os.ReadFile(indexPath)
					require.NoError(t, readErr)
					require.Equal(t, before, after)
				} else {
					var index xml.TidxXML
					require.NoError(t, utils.ReadXML(indexPath, &index))
					require.Empty(t, index.Devices)
				}
			})
		}
	}
	operation := &TargetOperation{}
	pack := &PackType{PdscTag: xml.PdscTag{Vendor: "Missing", Name: "DFP", Version: "1.0.0"}, IsPublic: true}
	require.NoError(t, pack.uninstallWithOperation(&PacksInstallationType{PackRoot: t.TempDir()}, operation))
	require.False(t, operation.changed)
	pack.Vendor = "["
	require.Error(t, pack.uninstallWithOperation(&PacksInstallationType{PackRoot: t.TempDir()}, operation))
	require.False(t, operation.changed)
}

func TestTargetIndexOrdering(t *testing.T) {
	provider := xml.PdscTag{Vendor: "Vendor", Name: "DFP", Version: "1.0.0", URL: "file:///a/"}
	other := provider
	other.URL = "file:///b/"
	devices := []xml.TargetDeviceTag{
		{DeviceIdentity: xml.DeviceIdentity{Vendor: "A", Name: "BC"}, Pdsc: other},
		{DeviceIdentity: xml.DeviceIdentity{Vendor: "AB", Name: "C"}, Pdsc: provider},
		{DeviceIdentity: xml.DeviceIdentity{Vendor: "A", Name: "BC"}, Pdsc: provider},
	}
	slices.SortFunc(devices, compareTargetDevices)
	require.Equal(t, provider.URL, devices[0].Pdsc.URL)
	require.Equal(t, other.URL, devices[1].Pdsc.URL)
	require.Equal(t, "AB", devices[2].Vendor)
	expected := slices.Clone(devices)
	slices.Reverse(devices)
	slices.SortFunc(devices, compareTargetDevices)
	require.Equal(t, expected, devices)
	first := xml.TargetBoardTag{Vendor: "Vendor", Name: "Board", Pdsc: provider,
		MountedDevices: []xml.TargetMountedDeviceTag{{DeviceIdentity: xml.DeviceIdentity{Name: "Chip"}, Index: "0"}}}
	second := first
	second.MountedDevices = []xml.TargetMountedDeviceTag{{DeviceIdentity: xml.DeviceIdentity{Name: "Chip"}, Index: "1"}}
	require.Less(t, compareTargetBoards(first, second), 0)
}

func TestTargetOperationRegistrationBatch(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	root := t.TempDir()
	require.NoError(t, SetPackRoot(root, true))
	UnlockPackRoot()
	operation := &TargetOperation{}
	var paths []string
	for _, name := range []string{"First", "Second"} {
		path := filepath.Join(t.TempDir(), "Vendor."+name+".pdsc")
		contents := fmt.Sprintf(`<package><vendor>Vendor</vendor><name>%s</name><releases><release version="1.0.0"/></releases><boards><board vendor="Vendor" name="%s"/></boards></package>`, name, name)
		require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
		require.NoError(t, operation.AddPdsc(path))
		paths = append(paths, path)
		require.NoFileExists(t, filepath.Join(root, ".Targets", "index.tidx"))
	}
	original := operation.AddPdsc("invalid")
	require.Error(t, original)
	require.ErrorIs(t, operation.Finish(original), original)
	indexPath := filepath.Join(root, ".Targets", "index.tidx")
	var index xml.TidxXML
	require.NoError(t, utils.ReadXML(indexPath, &index))
	require.Len(t, index.Boards, 2)
	require.NoError(t, os.Remove(indexPath))
	require.NoError(t, operation.AddPdsc(paths[0]))
	require.NoError(t, operation.Finish(nil))
	require.NoFileExists(t, indexPath)
	for _, path := range paths {
		require.NoError(t, operation.RemovePdsc(path))
		require.NoFileExists(t, indexPath)
	}
	require.NoError(t, operation.Finish(nil))
	index = xml.TidxXML{}
	require.NoError(t, utils.ReadXML(indexPath, &index))
	require.Empty(t, index.Boards)
	require.NoError(t, AddPdsc(paths[0]))
	index = xml.TidxXML{}
	require.NoError(t, utils.ReadXML(indexPath, &index))
	require.Len(t, index.Boards, 1)
}

func TestTargetIndexConcurrentReader(t *testing.T) {
	previous := Installation
	t.Cleanup(func() { Installation = previous })
	root := t.TempDir()
	Installation = &PacksInstallationType{PackRoot: root}
	require.NoError(t, UpdateTargetIndex())
	done := make(chan struct{})
	reader := make(chan error, 1)
	go func() {
		for {
			select {
			case <-done:
				reader <- nil
				return
			default:
				var index xml.TidxXML
				if err := utils.ReadXML(filepath.Join(root, ".Targets", "index.tidx"), &index); err != nil {
					if runtime.GOOS == "windows" && (errors.Is(err, syscall.Errno(32)) || os.IsPermission(err)) {
						continue
					}
					reader <- err
					return
				}
			}
		}
	}()
	var publicationError error
	for range 30 {
		if publicationError = UpdateTargetIndex(); publicationError != nil {
			break
		}
	}
	close(done)
	require.NoError(t, <-reader)
	if runtime.GOOS != "windows" || (!errors.Is(publicationError, syscall.Errno(32)) && !os.IsPermission(publicationError)) {
		require.NoError(t, publicationError)
	}
	var index xml.TidxXML
	require.NoError(t, utils.ReadXML(filepath.Join(root, ".Targets", "index.tidx"), &index))
}

func BenchmarkTargetIndexSort(b *testing.B) {
	devices := make([]xml.TargetDeviceTag, 1000)
	for index := range devices {
		devices[index] = xml.TargetDeviceTag{DeviceIdentity: xml.DeviceIdentity{Vendor: "Vendor", Name: fmt.Sprintf("Chip%04d", len(devices)-index)},
			Pdsc: xml.PdscTag{Vendor: "Vendor", Name: "DFP", Version: "1.0.0", URL: "file:///packs/"}}
	}
	b.Run("formatted", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			values := slices.Clone(devices)
			sort.Slice(values, func(left, right int) bool { return fmt.Sprint(values[left]) < fmt.Sprint(values[right]) })
		}
	})
	b.Run("fields", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			values := slices.Clone(devices)
			slices.SortFunc(values, compareTargetDevices)
		}
	})
}
