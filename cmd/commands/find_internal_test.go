/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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
