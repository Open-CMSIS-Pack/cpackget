/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	viperType "github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestFindProgress(t *testing.T) {
	var output bytes.Buffer
	progress := newFindProgress(&output)
	require.Contains(t, output.String(), "I: Searching targets")
	output.Reset()
	require.NoError(t, progress.Finish())
	require.True(t, progress.IsFinished())
	require.Equal(t, "\x1b[2K\r", output.String())
	output.Reset()
	output.WriteString("Installing board pack\nInstalling device pack\n")
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	require.Equal(t, "Installing board pack\nInstalling device pack\n", output.String())
	require.NoError(t, progress.Finish())
	require.Equal(t, "Installing board pack\nInstalling device pack\n", output.String())
}

func TestFindProgressPercentage(t *testing.T) {
	var output bytes.Buffer
	progress := newFindProgress(&output)
	require.NoError(t, progress.Set(25))
	require.Contains(t, output.String(), "25%")
	require.NoError(t, progress.Set(75))
	require.Contains(t, output.String(), "75%")
	require.NotContains(t, output.String(), "[0s")
	output.Reset()
	require.NoError(t, progress.Set(100))
	require.True(t, progress.IsFinished())
	require.Equal(t, "\x1b[2K\r", output.String())
	output.Reset()
	require.NoError(t, progress.Finish())
	require.Empty(t, output.String())
}

func TestFindMountedDevicesFlag(t *testing.T) {
	cmd := newFindCommand()
	board, _, err := cmd.Find([]string{"board"})
	require.NoError(t, err)
	enabled, err := board.Flags().GetBool("mounted-devices")
	require.NoError(t, err)
	require.False(t, enabled)
	require.NoError(t, board.Flags().Set("mounted-devices", "true"))
	enabled, err = board.Flags().GetBool("mounted-devices")
	require.NoError(t, err)
	require.True(t, enabled)
	device, _, err := cmd.Find([]string{"device"})
	require.NoError(t, err)
	require.Nil(t, device.Flags().Lookup("mounted-devices"))
}

func TestTargetSearchCancellation(t *testing.T) {
	originalViper := viper
	viper = viperType.New()
	t.Cleanup(func() { viper = originalViper })
	for _, kind := range []string{"device", "board"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetErr(&bytes.Buffer{})
			results, err := findTargetsWithProgress(cmd, kind, "chip", false)
			require.ErrorIs(t, err, context.Canceled)
			require.Empty(t, results)
		})
	}
}

func TestFindBoardMissingPackProvider(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		response := `{"data":{"search":{"metadata":{"total":1},"results":[{"id":"LPC810","name":"LPC810"}]}}}`
		if strings.Contains(string(body), "get_board") {
			response = `{"data":{"target":{"name":"LPC810","vendor":{"name":"NXP"},"cmsis":null,"devices":[]}}}`
		}
		_, err = io.WriteString(writer, response)
		require.NoError(t, err)
	}))
	defer server.Close()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	defer transport.CloseIdleConnections()
	originalTransport, originalViper := http.DefaultTransport, viper
	originalOutput, originalLevel := log.StandardLogger().Out, log.GetLevel()
	http.DefaultTransport = transport
	viper = viperType.New()
	t.Cleanup(func() {
		http.DefaultTransport, viper = originalTransport, originalViper
		log.SetOutput(originalOutput)
		log.SetLevel(originalLevel)
	})
	var output bytes.Buffer
	cmd := newFindCommand()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"board", "NXP::LPC810", "--insecure-skip-verify"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, output.String(), "(no matching targets)")
	require.NotContains(t, output.String(), "LPC810")
}

func TestFindCommandErrors(t *testing.T) {
	originalViper := viper
	originalOutput, originalLevel := log.StandardLogger().Out, log.GetLevel()
	t.Cleanup(func() {
		viper = originalViper
		log.SetOutput(originalOutput)
		log.SetLevel(originalLevel)
	})
	t.Run("conflicting verbosity", func(t *testing.T) {
		viper = viperType.New()
		viper.Set("quiet", true)
		viper.Set("verbose", true)
		cmd := newFindCommand()
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"device", "Chip"})
		require.ErrorContains(t, cmd.Execute(), "please pick only one")
	})
	t.Run("invalid TLS flag type", func(t *testing.T) {
		viper = viperType.New()
		cmd := newFindCommand()
		device, _, err := cmd.Find([]string{"device"})
		require.NoError(t, err)
		device.Flags().String("invalid-type", "", "")
		device.Flags().Lookup("insecure-skip-verify").Value = device.Flags().Lookup("invalid-type").Value
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"device", "Chip"})
		require.ErrorContains(t, cmd.Execute(), "bool")
	})
	t.Run("invalid mounted flag type", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		cmd.SetErr(io.Discard)
		cmd.Flags().String("mounted-devices", "", "")
		_, err := findTargetsWithProgress(cmd, "board", "Board", false)
		require.ErrorContains(t, err, "bool")
	})
}

func TestFindCommandResults(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			pdsc := `<package><vendor>Vendor</vendor><name>DFP</name><releases><release version="1.0.0"/></releases><devices><family Dvendor="Vendor:1"><device Dname="Chip"/></family></devices></package>`
			if strings.Contains(request.URL.Path, "BSP") {
				pdsc = `<package><vendor>Vendor</vendor><name>BSP</name><releases><release version="1.0.0"/></releases><boards><board vendor="Vendor" name="Board" revision="A"><mountedDevice Dvendor="Vendor:1" Dname="Chip" deviceIndex="0"/><mountedDevice Dvendor="Vendor:1" Dname="Missing" deviceIndex="1"/><mountedDevice Dvendor="Vendor:1" Dname="Chip" deviceIndex="2"/></board></boards></package>`
			}
			_, err := io.WriteString(writer, pdsc)
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
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": 1}, "results": []map[string]string{{"id": "board", "name": "Board"}}}}
		case strings.Contains(body.Query, "get_board"):
			data = map[string]interface{}{"target": map[string]interface{}{"name": "Board", "revision": "A", "vendor": map[string]string{"name": "Vendor"}, "cmsis": map[string]string{"pack_family_id": "BSP"}}}
		case strings.Contains(body.Query, "search_devices"):
			matches := []map[string]string{}
			if body.Variables["q"] != "Missing" {
				name, _ := body.Variables["q"].(string)
				matches = append(matches, map[string]string{"id": name, "name": name})
			}
			data = map[string]interface{}{"search": map[string]interface{}{"metadata": map[string]int{"total": len(matches)}, "results": matches}}
		case strings.Contains(body.Query, "get_device"):
			family := "DFP"
			if body.Variables["id"] == "Unresolved" {
				family = ""
			}
			data = map[string]interface{}{"target": map[string]interface{}{"name": body.Variables["id"], "vendor": map[string]string{"name": "Vendor"}, "source_pack_slug": family}}
		case strings.Contains(body.Query, "get_pack_family"):
			name, _ := body.Variables["id"].(string)
			data = map[string]interface{}{"pack": map[string]interface{}{"cmsis_id": map[string]string{"vendor": "Vendor", "name": name}, "current_release": map[string]string{"version": "1.0.0", "url": "https://" + request.Host + "/Vendor." + name + ".pack"}}}
		default:
			t.Errorf("unexpected query: %s", body.Query)
		}
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]interface{}{"data": data}))
	}))
	defer server.Close()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	defer transport.CloseIdleConnections()
	originalTransport, originalViper := http.DefaultTransport, viper
	originalOutput, originalLevel := log.StandardLogger().Out, log.GetLevel()
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport, viper = originalTransport, originalViper
		log.SetOutput(originalOutput)
		log.SetLevel(originalLevel)
	})
	for _, test := range []struct {
		name      string
		args      []string
		installed bool
		badIndex  bool
		want      []string
		absent    string
	}{
		{"board", []string{"board", "Board", "--mounted-devices"}, false, false, []string{"Vendor::Board:A [Vendor::BSP@1.0.0]", "mounted device: Vendor::Chip [Vendor::DFP@1.0.0]", "mounted device: Vendor::Missing (device pack unresolved)"}, ""},
		{"board without mounted devices", []string{"board", "Board"}, false, false, []string{"Vendor::Board:A [Vendor::BSP@1.0.0]"}, "mounted device:"},
		{"installed device", []string{"device", "Chip"}, true, false, []string{"Vendor::Chip [Vendor::DFP@1.0.0 (installed)]"}, ""},
		{"search error", []string{"device", "Unresolved"}, false, false, nil, ""},
		{"board status error", []string{"board", "Board"}, false, true, nil, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			viper = viperType.New()
			root := t.TempDir()
			viper.Set("pack-root", root)
			if test.installed {
				path := filepath.Join(root, "Vendor", "DFP", "1.0.0", "Vendor.DFP.pdsc")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, nil, 0600))
			}
			if test.badIndex {
				path := filepath.Join(root, ".Local", "local_repository.pidx")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, []byte(`<index>`), 0600))
			}
			var output bytes.Buffer
			cmd := newFindCommand()
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append(test.args, "--insecure-skip-verify"))
			err := cmd.Execute()
			if test.name == "search error" {
				require.ErrorContains(t, err, "no pack family")
				return
			}
			if test.badIndex {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, want := range test.want {
				require.Contains(t, output.String(), want)
			}
			if test.absent != "" {
				require.NotContains(t, output.String(), test.absent)
			}
		})
	}
}
