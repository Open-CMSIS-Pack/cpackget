/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package ui_test

import (
	"os"
	"testing"

	"github.com/jroimartin/gocui"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/stretchr/testify/assert"
)

// makeNonInteractive sets os.Stdout to a pipe so utils.IsTerminalInteractive returns false.
func makeNonInteractive(t *testing.T) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
		os.Stdout = oldStdout
	})
}

// feedStdin feeds given input into os.Stdin using a pipe.
func feedStdin(t *testing.T, input string) {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	// Write the input asynchronously and close writer.
	go func() {
		_, _ = w.Write([]byte(input))
		_ = w.Close()
	}()
	os.Stdin = r
	t.Cleanup(func() {
		_ = r.Close()
		os.Stdin = oldStdin
	})
}

func resetEulaGlobals(t *testing.T) {
	t.Helper()
	ui.LicenseAgreed = nil
	ui.Extract = false
	t.Cleanup(func() {
		ui.LicenseAgreed = nil
		ui.Extract = false
	})
}

func TestDisplayAndWaitForEULA_ExtractFlagAtStart(t *testing.T) {
	assert := assert.New(t)
	resetEulaGlobals(t)
	ui.Extract = true

	ok, err := ui.DisplayAndWaitForEULA("Title", "Contents")
	assert.False(ok, "expected ok=false when Extract is set")
	assert.ErrorIs(err, errs.ErrExtractEula)
}

func TestDisplayAndWaitForEULA_NonInteractive(t *testing.T) {
	assert := assert.New(t)

	t.Run("accept via input A", func(t *testing.T) {
		resetEulaGlobals(t)
		makeNonInteractive(t)
		feedStdin(t, "A\n")

		ok, err := ui.DisplayAndWaitForEULA("Title", "Contents")
		assert.NoError(err)
		assert.True(ok)
	})

	t.Run("decline via other input D", func(t *testing.T) {
		resetEulaGlobals(t)
		makeNonInteractive(t)
		feedStdin(t, "D\n")

		ok, err := ui.DisplayAndWaitForEULA("Title", "Contents")
		assert.NoError(err)
		assert.False(ok)
	})

	t.Run("extract via input E", func(t *testing.T) {
		resetEulaGlobals(t)
		makeNonInteractive(t)
		feedStdin(t, "E\n")

		ok, err := ui.DisplayAndWaitForEULA("Title", "Contents")
		assert.False(ok)
		assert.ErrorIs(err, errs.ErrExtractEula)
	})

	t.Run("preset agreed true", func(t *testing.T) {
		resetEulaGlobals(t)
		makeNonInteractive(t)
		ui.LicenseAgreed = &ui.Agreed

		ok, err := ui.DisplayAndWaitForEULA("Title", "Contents")
		assert.NoError(err)
		assert.True(ok)
	})

	t.Run("preset agreed false", func(t *testing.T) {
		resetEulaGlobals(t)
		makeNonInteractive(t)
		ui.LicenseAgreed = &ui.Disagreed

		ok, err := ui.DisplayAndWaitForEULA("Title", "Contents")
		assert.NoError(err)
		assert.False(ok)
	})
}

func TestNewLicenseWindow_AgreeDisagreeExtractHandlers(t *testing.T) {
	assert := assert.New(t)
	resetEulaGlobals(t)
	lw := ui.NewLicenseWindow("Title", "Contents", "Prompt")

	err := lw.Agree(nil, nil)
	assert.Equal(gocui.ErrQuit, err)
	if assert.NotNil(ui.LicenseAgreed) {
		assert.True(*ui.LicenseAgreed, "Agree should set LicenseAgreed to true")
	}

	// Reset and test Disagree
	ui.LicenseAgreed = nil
	err = lw.Disagree(nil, nil)
	assert.Equal(gocui.ErrQuit, err)
	if assert.NotNil(ui.LicenseAgreed) {
		assert.False(*ui.LicenseAgreed, "Disagree should set LicenseAgreed to false")
	}

	// Test Extract
	ui.Extract = false
	err = lw.Extract(nil, nil)
	assert.Equal(gocui.ErrQuit, err)
	assert.True(ui.Extract, "Extract handler should set Extract to true")
}

func TestLayoutRectValidation(t *testing.T) {
	assert := assert.New(t)

	t.Run("license rect invalid when terminal too small", func(t *testing.T) {
		lbx, lby, lex, ley, _, _, _, _ := ui.ComputeLicenseAndPromptRectsForTest(1, 1)
		err := ui.ValidateLicenseRectForTest(lbx, lby, lex, ley)
		assert.Error(err)
		assert.Contains(err.Error(), "increase size of license window")
	})

	t.Run("prompt rect invalid when terminal too small", func(t *testing.T) {
		// Choose width=2 so pbx==pex and triggers invalid prompt rect
		_, _, _, _, pbx, pby, pex, pey := ui.ComputeLicenseAndPromptRectsForTest(2, 6)
		err := ui.ValidatePromptRectForTest(pbx, pby, pex, pey)
		if assert.Error(err) {
			assert.Contains(err.Error(), "increase size of prompt window")
		}
	})

	t.Run("rects valid on reasonable terminal size", func(t *testing.T) {
		lbx, lby, lex, ley, pbx, pby, pex, pey := ui.ComputeLicenseAndPromptRectsForTest(80, 24)
		assert.NoError(ui.ValidateLicenseRectForTest(lbx, lby, lex, ley))
		assert.NoError(ui.ValidatePromptRectForTest(pbx, pby, pex, pey))
	})
}
