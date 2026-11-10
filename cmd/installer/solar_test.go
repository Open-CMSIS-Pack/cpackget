/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/require"
)

func TestTargetPackStatus(t *testing.T) {
	root := t.TempDir()
	pack := TargetPack{Vendor: "Vendor", Name: "DFP", Version: "1.0.0"}
	status, err := TargetPackStatus(root, pack)
	require.NoError(t, err)
	require.Empty(t, status)
	path := filepath.Join(root, pack.Vendor, pack.Name, pack.Version)
	require.NoError(t, os.MkdirAll(path, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(path, "Vendor.DFP.pdsc"), nil, 0600))
	status, err = TargetPackStatus(root, pack)
	require.NoError(t, err)
	require.Equal(t, " (installed)", status)
	local := t.TempDir()
	localPath := filepath.Join(local, "Vendor.BSP.pdsc")
	require.NoError(t, os.WriteFile(localPath, []byte(`<package><vendor>Vendor</vendor><name>BSP</name><releases><release version="1.0.0"/></releases></package>`), 0600))
	localURL, err := targetDirectoryURL(local)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".Local"), 0755))
	index := xml.NewPidxXML(filepath.Join(root, ".Local", "local_repository.pidx"), false)
	require.NoError(t, index.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "BSP", Version: "1.0.0", URL: localURL}))
	require.NoError(t, index.Write())
	status, err = TargetPackStatus(root, TargetPack{Vendor: "Vendor", Name: "BSP", Version: "1.0.0"})
	require.NoError(t, err)
	require.Equal(t, " (installed via "+localPath+")", status)
	lookup := NewTargetStatusLookup(root)
	_, err = lookup.Status(pack)
	require.NoError(t, err)
	require.NoError(t, os.Remove(index.GetFileName()))
	status, err = lookup.Status(TargetPack{Vendor: "Vendor", Name: "BSP", Version: "1.0.0"})
	require.NoError(t, err)
	require.Equal(t, " (installed via "+localPath+")", status)
}

func TestSolarBoardRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		var data interface{}
		if strings.Contains(body.Query, "search_boards") {
			require.Equal(t, "Board", body.Variables["q"])
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 3}, "results": []map[string]string{{"id": "Rev.B", "name": "Board"}, {"id": "Rev.C", "name": "Board"}, {"id": "Rev.C.1", "name": "Board"}}}}
		} else {
			data = map[string]interface{}{"target": map[string]interface{}{"name": "Board", "revision": body.Variables["id"], "vendor": map[string]string{"name": "Vendor"}}}
		}
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": data}))
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	for _, pattern := range []string{"Vendor::Board:Rev.C", "Board:rev.c", "Board:c", "Board:ev.C"} {
		results, err := client.discover(context.Background(), "board", pattern, nil)
		require.NoError(t, err)
		require.Len(t, results, 2)
		require.Equal(t, "Rev.C", results[0].Revision)
		require.Equal(t, "Rev.C.1", results[1].Revision)
	}
	results, err := client.discover(context.Background(), "board", "Board:c.1", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Rev.C.1", results[0].Revision)
	results, err = client.discover(context.Background(), "board", "Board:Rev.D", nil)
	require.NoError(t, err)
	require.Empty(t, results)
	_, err = client.discover(context.Background(), "board", "Board:", nil)
	require.Error(t, err)
	boards := []TargetResult{
		{Name: "Board", Revision: "Rev.B", Pack: TargetPack{Vendor: "Vendor", Name: "OldBSP", Version: "1.0.0"}},
		{Name: "Board", Revision: "Rev.C", Pack: TargetPack{Vendor: "Vendor", Name: "BSP", Version: "1.0.0"}},
	}
	for position := range boards {
		boards[position].Vendor.Name = "Vendor"
	}
	packs, err := ResolveTargetPacks("board", boards, "Vendor::Board:rev.c")
	require.NoError(t, err)
	require.Equal(t, []string{"Vendor::BSP@1.0.0"}, packs)
	longer := boards[1]
	longer.Name = "BoardExtended"
	longer.Revision = "Rev.C.1"
	longer.Pack.Name = "ExtendedBSP"
	boards = append(boards, longer)
	for _, pattern := range []string{"Vendor::Board:c", "Vendor::Board:ev.c"} {
		packs, err := ResolveTargetPacks("board", boards, pattern)
		require.NoError(t, err)
		require.Equal(t, []string{"Vendor::BSP@1.0.0"}, packs)
	}
	packs, err = ResolveTargetPacks("board", boards, "Vendor::BoardExtended:rev.c")
	require.NoError(t, err)
	require.Equal(t, []string{"Vendor::ExtendedBSP@1.0.0"}, packs)
	_, err = ResolveTargetPacks("board", boards, "Vendor::Board:Rev.D")
	require.ErrorContains(t, err, "no matching targets")
}

func TestSolarVendorSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			_, err := writer.Write([]byte(`<package><vendor>Vendor</vendor><name>Targets</name><releases><release version="1.0.0"/></releases><devices><family Dvendor="ABOV"><device Dname="Chip"/></family><family Dvendor="OtherVendor"><device Dname="ABOVChip"/></family></devices><boards><board vendor="ABOV" name="Chip"/><board vendor="OtherVendor" name="ABOVChip"/></boards></package>`))
			require.NoError(t, err)
			return
		}
		var body struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		var data interface{}
		if strings.Contains(body.Query, "search_") {
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 3}, "results": []map[string]string{{"id": "vendor", "name": "Chip"}, {"id": "name", "name": "ABOVChip"}, {"id": "unrelated", "name": "Other"}}}}
		} else if strings.Contains(body.Query, "get_pack_family") {
			data = map[string]interface{}{"pack": map[string]interface{}{"cmsis_id": TargetPack{Vendor: "Vendor", Name: "Targets"}, "current_release": map[string]string{"version": "1.0.0", "url": "http://" + request.Host + "/Vendor.Targets.pack"}}}
		} else {
			name, vendor := "Other", "OtherVendor"
			switch body.Variables["id"] {
			case "vendor":
				name, vendor = "Chip", "ABOV"
			case "name":
				name = "ABOVChip"
			}
			data = map[string]interface{}{"target": map[string]interface{}{"name": name, "vendor": map[string]string{"name": vendor}, "source_pack_slug": "targets", "cmsis": map[string]string{"pack_family_id": "targets"}}}
		}
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": data}))
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	for _, kind := range []string{"device", "board"} {
		for _, installation := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/installation=%t", kind, installation), func(t *testing.T) {
				for _, pattern := range []string{"ABOV", "abov", "bo"} {
					updates := []int{}
					client.progress = func(percent int) { updates = append(updates, percent) }
					results, err := client.searchSelected(context.Background(), kind, pattern, false, installation)
					require.NoError(t, err)
					require.Len(t, results, 2)
					require.Equal(t, []int{0, 14, 28, 42, 60, 80, 100}, updates)
					require.NotEmpty(t, updates)
					require.Equal(t, 0, updates[0])
					require.Equal(t, 100, updates[len(updates)-1])
					for position := 1; position < len(updates); position++ {
						require.GreaterOrEqual(t, updates[position], updates[position-1])
						require.Less(t, updates[position-1], 100)
					}
				}
				results, err := client.searchSelected(context.Background(), kind, "abov::chip", false, installation)
				require.NoError(t, err)
				require.Len(t, results, 1)
				require.Equal(t, "ABOV", results[0].Vendor.Name)
				for _, pattern := range []string{"bo::hi", "Bo::CHIP"} {
					results, err := client.searchSelected(context.Background(), kind, pattern, false, installation)
					require.NoError(t, err)
					require.Len(t, results, 1)
					require.Equal(t, "ABOV", results[0].Vendor.Name)
					require.Equal(t, "Chip", results[0].Name)
				}
				results, err = client.searchSelected(context.Background(), kind, "ABOV::ABOV", false, installation)
				require.NoError(t, err)
				require.Empty(t, results)
			})
		}
	}
}

func TestSolarInstallationSelectsBeforeResolution(t *testing.T) {
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			_, err := writer.Write([]byte(`<package><vendor>Vendor</vendor><name>DFP</name><releases><release version="1.0.0"/></releases><devices><family Dvendor="Vendor"><device Dname="Chip"/></family></devices></package>`))
			require.NoError(t, err)
			return
		}
		var body struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.NotContains(t, body.Query, "source_pack_id")
		var data interface{}
		switch {
		case strings.Contains(body.Query, "search_devices"):
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 2}, "results": []map[string]string{{"id": "Chip", "name": "Chip"}, {"id": "ChipLong", "name": "ChipLong"}}}}
		case strings.Contains(body.Query, "get_device"):
			name := body.Variables["id"].(string)
			data = map[string]interface{}{"target": map[string]interface{}{"name": name, "vendor": map[string]string{"name": "Vendor"}, "source_pack_slug": name}}
		case strings.Contains(body.Query, "get_pack_family"):
			id := body.Variables["id"].(string)
			requests[id]++
			if id == "ChipLong" {
				http.Error(writer, "unavailable", http.StatusServiceUnavailable)
				return
			}
			data = map[string]interface{}{"pack": map[string]interface{}{"cmsis_id": TargetPack{Vendor: "Vendor", Name: "DFP"}, "current_release": map[string]string{"version": "1.0.0", "url": "http://" + request.Host + "/Vendor.DFP.pack"}}}
		}
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": data}))
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	results, err := client.searchSelected(context.Background(), "device", "Vendor::Chip", true, true)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, 0, requests["ChipLong"])
	_, err = client.search(context.Background(), "device", "Vendor::Chip")
	require.Error(t, err)
	require.Equal(t, 1, requests["ChipLong"])
}

func BenchmarkSolarDeviceMembership(b *testing.B) {
	pdsc := &xml.PdscXML{Families: []xml.DeviceNode{{Vendor: "Vendor", Devices: make([]xml.DeviceNode, 1000)}}}
	for index := range pdsc.Families[0].Devices {
		pdsc.Families[0].Devices[index].Name = fmt.Sprintf("Chip%04d", index)
	}
	identity := xml.DeviceIdentity{Vendor: "Vendor", Name: "Chip0999"}
	b.Run("flatten-per-match", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			found := false
			for _, device := range pdsc.TargetDevices() {
				if device == identity {
					found = true
					break
				}
			}
			if !found {
				b.Fatal("missing device")
			}
		}
	})
	devices := make(map[xml.DeviceIdentity]bool)
	for _, device := range pdsc.TargetDevices() {
		devices[device] = true
	}
	b.Run("cached-identity-set", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !devices[identity] {
				b.Fatal("missing device")
			}
		}
	})
}

func TestSolarProviderProgress(t *testing.T) {
	for _, kind := range []string{"device", "board"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == http.MethodGet {
					pack := "A"
					if request.URL.Path == "/Vendor.B.pdsc" {
						pack = "B"
					}
					_, err := fmt.Fprintf(writer, `<package><vendor>Vendor</vendor><name>%s</name><releases><release version="1.0.0"/></releases><devices><family Dvendor="Vendor"><device Dname="Chip%s"/></family></devices><boards><board vendor="Vendor" name="Chip%s"/></boards></package>`, pack, pack, pack)
					require.NoError(t, err)
					return
				}
				var body struct {
					Query     string                 `json:"query"`
					Variables map[string]interface{} `json:"variables"`
				}
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				identity, _ := body.Variables["id"].(string)
				var data interface{}
				switch {
				case strings.Contains(body.Query, "search_"):
					data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 2}, "results": []map[string]string{{"id": "A", "name": "ChipA"}, {"id": "B", "name": "ChipB"}}}}
				case strings.Contains(body.Query, "get_pack_family"):
					data = map[string]interface{}{"pack": map[string]interface{}{"cmsis_id": TargetPack{Vendor: "Vendor", Name: identity}, "current_release": map[string]string{"version": "1.0.0", "url": "http://" + request.Host + "/Vendor." + identity + ".pack"}}}
				default:
					data = map[string]interface{}{"target": map[string]interface{}{"name": "Chip" + identity, "vendor": map[string]string{"name": "Vendor"}, "source_pack_slug": identity, "cmsis": map[string]string{"pack_family_id": identity}}}
				}
				require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": data}))
			}))
			defer server.Close()
			var updates []int
			client := solarClient{endpoint: server.URL, http: server.Client(), progress: func(percent int) { updates = append(updates, percent) }}
			results, err := client.searchSelected(context.Background(), kind, "Chip", false, false)
			require.NoError(t, err)
			require.Len(t, results, 2)
			require.Equal(t, []int{0, 20, 40, 40, 60, 80, 100}, updates)
		})
	}
}

func TestSolarDiscoveryConcurrent(t *testing.T) {
	var active, peak atomic.Int32
	started := make(chan struct{}, 16)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if strings.Contains(body.Query, "search_devices") {
			matches := make([]map[string]string, 16)
			for position := range matches {
				name := fmt.Sprintf("Chip%02d", position)
				matches[position] = map[string]string{"id": name, "name": name}
			}
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"data": map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": len(matches)}, "results": matches}}})
			return
		}
		current := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); current > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-request.Context().Done():
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"data": map[string]interface{}{"target": map[string]interface{}{"name": body.Variables["id"], "vendor": map[string]string{"name": "Vendor"}, "source_pack_slug": "dfp"}}})
	}))
	defer server.Close()
	defer close(release)
	client := solarClient{endpoint: server.URL, http: server.Client()}
	done := make(chan struct{})
	var results []solarTarget
	var searchErr error
	var updates []int
	go func() {
		results, searchErr = client.discover(context.Background(), "device", "Vendor::Chip", func(completed, total int) {
			updates = append(updates, 100*completed/(total+1))
		})
		close(done)
	}()
	for range 8 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("detail requests did not run concurrently")
		}
	}
	for range 16 {
		release <- struct{}{}
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("discovery did not finish")
	}
	require.NoError(t, searchErr)
	require.Equal(t, int32(8), peak.Load())
	require.Len(t, results, 16)
	require.Len(t, updates, 17)
	for completed, percent := range updates {
		require.Equal(t, 100*completed/17, percent)
	}
	for position, result := range results {
		require.Equal(t, fmt.Sprintf("Chip%02d", position), result.Name)
	}
}

func TestSolarSearch(t *testing.T) {
	removed := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			if removed {
				_, err := writer.Write([]byte(`<package><vendor>Vendor</vendor><name>DFP</name><releases><release version="1.0.0"/></releases></package>`))
				require.NoError(t, err)
				return
			}
			_, err := writer.Write([]byte(`<package><vendor>Vendor</vendor><name>DFP</name><releases><release version="1.00.00"/></releases><devices><family Dvendor="Vendor"><device Dname="ChipA"/><device Dname="ChipB"/></family></devices></package>`))
			require.NoError(t, err)
			return
		}
		var body struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		writer.Header().Set("Content-Type", "application/json")
		if strings.Contains(body.Query, "get_pack_family") {
			require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": map[string]interface{}{"pack": map[string]interface{}{"cmsis_id": TargetPack{Vendor: "Vendor", Name: "DFP"}, "current_release": map[string]string{"version": "1.0.0", "url": "http://" + request.Host + "/Vendor.DFP.1.0.0.pack"}}}}))
			return
		}
		if strings.Contains(body.Query, "search_devices") {
			require.Equal(t, "chip", body.Variables["q"])
			name := "ChipA"
			if body.Variables["offset"] == float64(1) {
				name = "ChipB"
			}
			require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 2}, "results": []map[string]string{{"id": name, "name": name}}}}}))
			return
		}
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": map[string]interface{}{"target": map[string]interface{}{"name": body.Variables["id"], "vendor": map[string]string{"name": "Vendor"}, "source_pack_slug": "dfp-vendor", "source_pack_id": TargetPack{Vendor: "Vendor", Name: "DFP", Version: "0.9.0"}}}}))
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	var updates []int
	client.progress = func(percent int) { updates = append(updates, percent) }
	results, err := client.search(context.Background(), "device", "chip")
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, []int{0, 20, 40, 50, 75, 100}, updates)
	packs, err := ResolveTargetPacks("device", results)
	require.NoError(t, err)
	require.Equal(t, []string{"Vendor::DFP@1.00.00"}, packs)
	qualified, err := client.search(context.Background(), "device", "vEnDoR::chip")
	require.NoError(t, err)
	require.Equal(t, results, qualified)
	otherVendor, err := client.search(context.Background(), "device", "OtherVendor::chip")
	require.NoError(t, err)
	require.Empty(t, otherVendor)
	results[1].Pack.Name = "Alternative"
	_, err = ResolveTargetPacks("device", results)
	require.EqualError(t, err, "device selection is ambiguous or unresolved; specify a device explicitly:\nVendor::ChipA\nVendor::ChipB")
	removed = true
	results, err = client.search(context.Background(), "device", "chip")
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestResolveBoardPacks(t *testing.T) {
	board := TargetResult{Pack: TargetPack{Vendor: "Vendor", Name: "BSP", Version: "1.0.0"}, Devices: []TargetResult{{Pack: TargetPack{Vendor: "Vendor", Name: "DFP", Version: "2.0.0"}}}}
	board.Vendor.Name = "BoardVendor"
	board.Name = "Board"
	board.Revision = "A"
	packs, err := ResolveTargetPacks("board", []TargetResult{board, board})
	require.NoError(t, err)
	require.Equal(t, []string{"Vendor::BSP@1.0.0", "Vendor::DFP@2.0.0"}, packs)
	alternative := board.Devices[0]
	alternative.Pack.Name = "AlternativeDFP"
	board.Devices = append(board.Devices, alternative)
	_, err = ResolveTargetPacks("board", []TargetResult{board})
	require.EqualError(t, err, "board selection is ambiguous or unresolved; specify a board explicitly:\nBoardVendor::Board:A")
	board.Devices[0].Pack = TargetPack{}
	_, err = ResolveTargetPacks("board", []TargetResult{board})
	require.EqualError(t, err, "board selection is ambiguous or unresolved; specify a board explicitly:\nBoardVendor::Board:A")
	_, err = ResolveTargetPacks("device", nil)
	require.Error(t, err)
}

func TestResolveTargetPacksExactMatch(t *testing.T) {
	board := TargetResult{Name: "LPCXpresso54S018", Pack: TargetPack{Vendor: "NXP", Name: "BSP", Version: "1.0.0"}}
	board.Vendor.Name = "NXP"
	longer := board
	longer.Name += "M"
	longer.Pack.Name = "OtherBSP"
	for _, kind := range []string{"board", "device"} {
		t.Run(kind, func(t *testing.T) {
			for _, pattern := range []string{"LPCXpresso54S018", "nxp::lpcxpresso54s018", "Nx::lpcxpresso54s018"} {
				packs, err := ResolveTargetPacks(kind, []TargetResult{longer, board}, pattern)
				require.NoError(t, err)
				require.Equal(t, []string{"NXP::BSP@1.0.0"}, packs)
			}
			_, err := ResolveTargetPacks(kind, []TargetResult{board, longer}, "NXP::LPCXpresso54")
			require.ErrorContains(t, err, "selection is ambiguous")
			conflict := board
			conflict.Pack.Name = "ConflictingBSP"
			_, err = ResolveTargetPacks(kind, []TargetResult{board, conflict, longer}, "NXP::LPCXpresso54S018")
			require.ErrorContains(t, err, "selection is ambiguous")
			unresolved := board
			unresolved.Pack = TargetPack{}
			_, err = ResolveTargetPacks(kind, []TargetResult{unresolved, longer}, "NXP::LPCXpresso54S018")
			require.Error(t, err)
		})
	}
}

func TestResolveBoardMissingPackProvider(t *testing.T) {
	board := TargetResult{Name: "LPC810", Devices: []TargetResult{{Pack: TargetPack{Vendor: "NXP", Name: "LPC810_DFP", Version: "13.0.0"}}}}
	board.Vendor.Name = "NXP"
	packs, err := ResolveTargetPacks("board", []TargetResult{board})
	require.EqualError(t, err, "cannot install NXP::LPC810: board pack provider is unresolved")
	require.Nil(t, packs)
	board.Revision = "A"
	_, err = ResolveTargetPacks("board", []TargetResult{board})
	require.EqualError(t, err, "cannot install NXP::LPC810:A: board pack provider is unresolved")
}

func TestSolarBoardUnresolvedProviderFiltered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		response := `{"data":{"search":{"metadata":{"total":1},"results":[{"id":"board","name":"Board"}]}}}`
		if strings.Contains(body.Query, "get_board") {
			response = `{"data":{"target":{"name":"Board","vendor":{"name":"Vendor"},"cmsis":null}}}`
		}
		_, err := writer.Write([]byte(response))
		require.NoError(t, err)
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	for _, mountedDevices := range []bool{false, true} {
		results, err := client.search(context.Background(), "board", "Board", mountedDevices)
		require.NoError(t, err)
		require.Empty(t, results)
		_, err = ResolveTargetPacks("board", results, "Board")
		require.EqualError(t, err, "no matching targets found")
	}
}

func TestSolarFailures(t *testing.T) {
	for _, response := range []string{
		`{"errors":[{"message":"unavailable"}]}`,
		`{"data":null}`,
		`{"data":{"search":{"metadata":{"total":2},"results":[]}}}`,
		`{"data":{"search":null}}`,
		`invalid JSON`,
	} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, err := writer.Write([]byte(response))
				require.NoError(t, err)
			}))
			defer server.Close()
			client := solarClient{endpoint: server.URL, http: server.Client()}
			_, err := client.search(context.Background(), "device", "chip")
			require.Error(t, err)
		})
	}
}

func TestSolarBoardSearch(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_, err := writer.Write([]byte(`<package><vendor>Vendor</vendor><name>BSP</name><releases><release version="1.0.0"/></releases><boards><board vendor="Vendor" name="Board" revision="A"><mountedDevice Dvendor="ChipVendor:1" Dname="Chip" deviceIndex="0"/></board></boards></package>`))
			require.NoError(t, err)
			return
		}
		var body struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		response := `{"data":{"search":{"metadata":{"total":0},"results":[]}}}`
		switch {
		case strings.Contains(body.Query, "search_boards"):
			response = `{"data":{"search":{"metadata":{"total":1},"results":[{"id":"board","name":"Board"}]}}}`
		case strings.Contains(body.Query, "get_board"):
			response = `{"data":{"target":{"name":"Board","revision":"A","vendor":{"name":"Vendor"},"cmsis":{"pack_family_id":"bsp"},"devices":[{"name":"CompatibleNotMounted"}]}}}`
		case strings.Contains(body.Query, "get_pack_family"):
			response = `{"data":{"pack":{"cmsis_id":{"vendor":"Vendor","name":"BSP"},"current_release":{"version":"1.0.0","url":"` + server.URL + `/Vendor.BSP.1.0.0.pack"}}}}`
		}
		_, err := writer.Write([]byte(response))
		require.NoError(t, err)
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	results, err := client.search(context.Background(), "board", "board")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Len(t, results[0].Devices, 1)
	require.Equal(t, "Chip", results[0].Devices[0].Name)
	require.Equal(t, "ChipVendor", results[0].Devices[0].Vendor.Name)
	qualified, err := client.search(context.Background(), "board", "vEnDoR::board")
	require.NoError(t, err)
	require.Equal(t, results, qualified)
	otherVendor, err := client.search(context.Background(), "board", "OtherVendor::board")
	require.NoError(t, err)
	require.Empty(t, otherVendor)
	_, err = ResolveTargetPacks("board", results)
	require.ErrorContains(t, err, "unresolved")
}

func TestSolarInvalidQualifiedPattern(t *testing.T) {
	client := solarClient{}
	for _, pattern := range []string{"::chip", "Vendor::", "Vendor::Other::chip"} {
		t.Run(pattern, func(t *testing.T) {
			_, err := client.search(context.Background(), "device", pattern)
			require.ErrorContains(t, err, "Vendor::pattern")
		})
	}
}

func TestSolarBoardMountedDeviceVendor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		response := `{"data":{"search":{"metadata":{"total":1},"results":[{"id":"board","name":"Board"}]}}}`
		if strings.Contains(body.Query, "get_board") {
			response = `{"data":{"target":{"name":"Board","vendor":{"name":"Vendor"},"cmsis":{"pack_family_id":"bsp"}}}}`
		}
		_, err := writer.Write([]byte(response))
		require.NoError(t, err)
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	pdsc := &xml.PdscXML{Boards: []xml.BoardTag{{Vendor: "Vendor", Name: "Board", MountedDevices: []xml.MountedDeviceTag{{Vendor: "Vendor:1", Name: "Chip"}}}}}
	provider := func(vendor, name, pack string) TargetResult {
		result := TargetResult{Name: name, Pack: TargetPack{Vendor: "Packs", Name: pack, Version: "1.0.0"}}
		result.Vendor.Name = vendor
		return result
	}
	wrong := provider("OtherVendor", "Chip", "WrongDFP")
	for _, test := range []struct {
		name      string
		providers []TargetResult
		want      string
	}{
		{"exact", []TargetResult{wrong, provider("vEnDoR", "Chip", "DFP")}, "Chip"},
		{"prefix", []TargetResult{wrong, provider("Vendor", "ChipVariant", "DFP")}, "ChipVariant"},
		{"unresolved", []TargetResult{wrong}, "Chip"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cache := &solarSearchCache{families: map[string]solarFamily{"bsp": {pack: TargetPack{Vendor: "Packs", Name: "BSP", Version: "1.0.0"}, pdsc: pdsc}}, devices: map[string][]TargetResult{"vendor::chip": test.providers}, mountedDevices: true}
			results, err := client.searchCached(context.Background(), "board", "Board", cache, false)
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Len(t, results[0].Devices, 1)
			require.Equal(t, test.want, results[0].Devices[0].Name)
			packs, err := ResolveTargetPacks("board", results)
			if test.name == "unresolved" {
				require.ErrorContains(t, err, "unresolved")
			} else {
				require.NoError(t, err)
				require.Equal(t, []string{"Packs::BSP@1.0.0", "Packs::DFP@1.0.0"}, packs)
			}
		})
	}
}

func TestSolarBoardMountedDeviceFamily(t *testing.T) {
	requests := make(map[string]int)
	exact := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			requests[request.URL.Path]++
			pdsc := `<package><vendor>NXP</vendor><name>LPC812_DFP</name><releases><release version="13.0.0"/></releases><devices><family Dvendor="NXP:11"><device Dname="LPC812M101JD20"/><device Dname="LPC812M101JDH16"/><device Dname="LPC812M101JDH20"/><device Dname="LPC812M101JTB16"/></family></devices></package>`
			if exact {
				pdsc = strings.Replace(pdsc, `<device Dname="LPC812M101JD20"/>`, `<device Dname="LPC812M101JD20"/><device Dname="LPC812"/>`, 1)
			}
			if request.URL.Path == "/NXP.LPCXpresso812MAX_BSP.pdsc" {
				pdsc = `<package><vendor>NXP</vendor><name>LPCXpresso812MAX_BSP</name><releases><release version="13.0.0"/></releases><boards><board vendor="NXP" name="LPCXpresso812MAX"><mountedDevice Dvendor="NXP:11" Dname="LPC812" deviceIndex="0"/><mountedDevice Dvendor="NXP:11" Dname="LPC812" deviceIndex="1"/></board></boards></package>`
			}
			_, err := writer.Write([]byte(pdsc))
			require.NoError(t, err)
			return
		}
		var body struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		var data interface{}
		switch {
		case strings.Contains(body.Query, "search_boards"):
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 1}, "results": []map[string]string{{"id": "board", "name": "LPCXpresso812MAX"}}}}
		case strings.Contains(body.Query, "get_board"):
			data = map[string]interface{}{"target": map[string]interface{}{"name": "LPCXpresso812MAX", "vendor": map[string]string{"name": "NXP"}, "cmsis": map[string]string{"pack_family_id": "bsp"}}}
		case strings.Contains(body.Query, "search_devices"):
			require.Equal(t, "LPC812", body.Variables["q"])
			requests["search_devices"]++
			matches := []map[string]string{}
			for _, name := range []string{"LPC812M101JD20", "LPC812M101JDH16", "LPC812M101JDH20", "LPC812M101JTB16"} {
				matches = append(matches, map[string]string{"id": name, "name": name})
			}
			if exact {
				matches = append(matches, map[string]string{"id": "LPC812", "name": "LPC812"})
			}
			matches = append(matches, map[string]string{"id": "other", "name": "OtherLPC812"})
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": len(matches)}, "results": matches}}
		case strings.Contains(body.Query, "get_device"):
			data = map[string]interface{}{"target": map[string]interface{}{"name": body.Variables["id"], "vendor": map[string]string{"name": "NXP"}, "source_pack_slug": "dfp"}}
		case strings.Contains(body.Query, "get_pack_family"):
			identity := "LPC812_DFP"
			if body.Variables["id"] == "bsp" {
				identity = "LPCXpresso812MAX_BSP"
			}
			requests[identity]++
			data = map[string]interface{}{"pack": map[string]interface{}{"cmsis_id": TargetPack{Vendor: "NXP", Name: identity}, "current_release": map[string]string{"version": "13.0.0", "url": "http://" + request.Host + "/NXP." + identity + ".pack"}}}
		default:
			t.Errorf("unexpected query: %s", body.Query)
		}
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": data}))
	}))
	defer server.Close()
	client := solarClient{endpoint: server.URL, http: server.Client()}
	results, err := client.search(context.Background(), "board", "NXP::LPCXpresso812MAX")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Len(t, results[0].Devices, 8)
	packs, err := ResolveTargetPacks("board", results)
	require.NoError(t, err)
	require.Equal(t, []string{"NXP::LPC812_DFP@13.0.0", "NXP::LPCXpresso812MAX_BSP@13.0.0"}, packs)
	require.Equal(t, 1, requests["LPC812_DFP"])
	require.Equal(t, 1, requests["/NXP.LPC812_DFP.pdsc"])
	require.Equal(t, 1, requests["search_devices"])
	withoutDevices, err := client.search(context.Background(), "board", "NXP::LPCXpresso812MAX", false)
	require.NoError(t, err)
	require.Len(t, withoutDevices, 1)
	require.Empty(t, withoutDevices[0].Devices)
	require.Equal(t, results[0].Pack, withoutDevices[0].Pack)
	require.Equal(t, 1, requests["search_devices"])
	require.Equal(t, 1, requests["LPC812_DFP"])
	require.Equal(t, 1, requests["/NXP.LPC812_DFP.pdsc"])
	results[0].Devices[1].Pack.Name = "AlternativeDFP"
	_, err = ResolveTargetPacks("board", results)
	require.ErrorContains(t, err, "ambiguous")
	exact = true
	results, err = client.search(context.Background(), "board", "NXP::LPCXpresso812MAX")
	require.NoError(t, err)
	require.Len(t, results[0].Devices, 2)
	require.Equal(t, "LPC812", results[0].Devices[0].Name)
	require.Equal(t, 2, requests["LPC812_DFP"])
	require.Equal(t, 2, requests["/NXP.LPC812_DFP.pdsc"])
	require.Equal(t, 2, requests["search_devices"])
}
