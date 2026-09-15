/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteUpdateCfg(t *testing.T) {
	webDir := t.TempDir()
	installation := PacksInstallationType{WebDir: webDir}

	tests := []struct {
		name     string
		conf     updateCfg
		expected string
	}{
		{
			name:     "enabled",
			conf:     updateCfg{Date: "1-1-2000", Auto: true, UpdateDaily: true},
			expected: "Date=1-1-2000\nAuto=true\nUpdateDaily=true\n",
		},
		{
			name:     "disabled",
			conf:     updateCfg{Date: "2-2-2000", Auto: false, UpdateDaily: false},
			expected: "Date=2-2-2000\nAuto=false\nUpdateDaily=false\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := installation.writeUpdateCfg(&test.conf); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(webDir, "update.cfg"))
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != test.expected {
				t.Fatalf("unexpected update.cfg content: %q", content)
			}
		})
	}

	t.Run("write error", func(t *testing.T) {
		installation := PacksInstallationType{WebDir: filepath.Join(t.TempDir(), "missing")}
		if err := installation.writeUpdateCfg(&updateCfg{}); err == nil {
			t.Fatal("expected an error for a missing web directory")
		}
	})
}

func TestRecordPublicIndexUpdate(t *testing.T) {
	originalInstallation := Installation
	t.Cleanup(func() { Installation = originalInstallation })

	t.Run("preserves settings and records current date", func(t *testing.T) {
		webDir := t.TempDir()
		Installation = &PacksInstallationType{WebDir: webDir}
		path := filepath.Join(webDir, "update.cfg")
		if err := os.WriteFile(path, []byte("Date=1-1-2000\nAuto=false\nUpdateDaily=false\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := RecordPublicIndexUpdate(); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := "Date=" + time.Now().Local().Format("2-1-2006") + "\nAuto=false\nUpdateDaily=false\n"
		if string(content) != expected {
			t.Fatalf("unexpected update.cfg content: %q", content)
		}
	})

	t.Run("returns write error", func(t *testing.T) {
		Installation = &PacksInstallationType{WebDir: filepath.Join(t.TempDir(), "missing")}
		if err := RecordPublicIndexUpdate(); err == nil {
			t.Fatal("expected an error for a missing web directory")
		}
	})
}
