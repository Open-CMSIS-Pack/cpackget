/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newFindCommand() *cobra.Command {
	parent := &cobra.Command{Use: "find", Short: "Find devices and boards online"}
	for _, kind := range []string{"device", "board"} {
		command := &cobra.Command{
			Use:   kind + " <pattern>",
			Short: "Find matching " + kind + " targets online",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := configureInstallerGlobalCmd(cmd, args); err != nil {
					return err
				}
				insecure, err := cmd.Flags().GetBool("insecure-skip-verify")
				if err != nil {
					return err
				}
				results, err := findTargetsWithProgress(cmd, kind, args[0], insecure)
				if err != nil {
					return err
				}
				statuses := make(map[string]string)
				lookup := installer.NewTargetStatusLookup(viper.GetString("pack-root"))
				status := func(pack installer.TargetPack) (string, error) {
					if pack.Vendor == "" || pack.Name == "" || pack.Version == "" {
						return "", nil
					}
					if annotation, ok := statuses[pack.ID()]; ok {
						return annotation, nil
					}
					annotation, err := lookup.Status(pack)
					if err == nil {
						statuses[pack.ID()] = annotation
					}
					return annotation, err
				}
				for _, result := range results {
					name := result.Vendor.Name + "::" + result.Name
					if result.Revision != "" {
						name += ":" + result.Revision
					}
					if result.Pack.Vendor == "" || result.Pack.Name == "" || result.Pack.Version == "" {
						if kind == "board" {
							log.Infof("%s (board pack unresolved)", name)
						} else {
							log.Info(name)
						}
					} else {
						annotation, err := status(result.Pack)
						if err != nil {
							return err
						}
						log.Infof("%s [%s%s]", name, result.Pack.ID(), annotation)
					}
					for _, device := range result.Devices {
						if device.Pack.Vendor == "" {
							log.Infof("  mounted device: %s::%s (device pack unresolved)", device.Vendor.Name, device.Name)
						} else {
							annotation, err := status(device.Pack)
							if err != nil {
								return err
							}
							log.Infof("  mounted device: %s::%s [%s%s]", device.Vendor.Name, device.Name, device.Pack.ID(), annotation)
						}
					}
				}
				if len(results) == 0 {
					log.Info("(no matching targets)")
				}
				return nil
			},
		}
		command.Flags().Bool("insecure-skip-verify", false, "skip verification of server's TLS certificate")
		if kind == "board" {
			command.Flags().Bool("mounted-devices", false, "search and show mounted devices and their providing packs")
		}
		parent.AddCommand(command)
	}
	return parent
}

func findTargetsWithProgress(cmd *cobra.Command, kind, pattern string, insecure bool) ([]installer.TargetResult, error) {
	return searchTargetsWithProgress(cmd, kind, pattern, insecure, false)
}

func searchTargetsWithProgress(cmd *cobra.Command, kind, pattern string, insecure, installation bool) ([]installer.TargetResult, error) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var report func(int)
	if stream, ok := cmd.ErrOrStderr().(*os.File); ok && utils.IsTerminalInteractive() && log.IsLevelEnabled(log.InfoLevel) {
		if info, err := stream.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			progress := newFindProgress(stream)
			report = func(percent int) {
				if err := progress.Set(percent); err != nil {
					log.Debug(err)
				}
			}
			defer func() {
				if err := progress.Finish(); err != nil {
					log.Debug(err)
				}
			}()
		}
	}
	mountedDevices := true
	if cmd.Flags().Lookup("mounted-devices") != nil {
		var err error
		mountedDevices, err = cmd.Flags().GetBool("mounted-devices")
		if err != nil {
			return nil, err
		}
	}
	return installer.FindTargetsWithProgress(ctx, kind, pattern, insecure, viper.GetInt("timeout"), mountedDevices, installation, report)
}

type findProgress struct {
	*progressbar.ProgressBar
	once sync.Once
	err  error
}

func (progress *findProgress) Finish() error {
	progress.once.Do(func() {
		if !progress.IsFinished() {
			progress.err = progress.ProgressBar.Finish()
		}
	})
	return progress.err
}

func newFindProgress(writer io.Writer) *findProgress {
	bar := progressbar.NewOptions(100,
		progressbar.OptionSetWriter(writer),
		progressbar.OptionSetDescription("I: Searching targets"),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionClearOnFinish(),
	)
	return &findProgress{ProgressBar: bar}
}

func init() {
	AllCommands = append(AllCommands, newFindCommand())
}
