/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package ui

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

var Agreed = true
var Disagreed = false
var LicenseAgreed *bool
var Extract = false
var terminalWidth, terminalHeight int
var minTerminalWidth int

// LicenseWindowType defines the struct to handle UI
type LicenseWindowType struct {
	// LayoutManager is a function that defines the elements in the ui
	LayoutManager func(g *gocui.Gui) error

	Scroll   func(v *gocui.View, dy int) error
	ScrollUp func(g *gocui.Gui, v *gocui.View) error

	ScrollDown func(g *gocui.Gui, v *gocui.View) error

	Agree func(g *gocui.Gui, v *gocui.View) error

	Disagree func(g *gocui.Gui, v *gocui.View) error

	Extract func(g *gocui.Gui, v *gocui.View) error

	Gui *gocui.Gui
}

// DisplayAndWaitForEULA prints out the license to the user through a UI
// and waits for user confirmation.
func DisplayAndWaitForEULA(licenseTitle, licenseContents string) (bool, error) {
	if Extract {
		return false, errs.ErrExtractEula
	}

	promptText := "License Agreement: [A]ccept [D]ecline [E]xtract"

	if !utils.IsTerminalInteractive() {
		// Show input on non-interactive terminals
		promptText = "License Agreement: [A]ccept [D]ecline [E]xtract: "
		fmt.Printf("*** %v ***", licenseTitle)
		fmt.Println()
		fmt.Println(licenseContents)
		fmt.Println()
		fmt.Print(promptText)

		if LicenseAgreed != nil {
			return *LicenseAgreed, nil
		}

		if Extract {
			return false, errs.ErrExtractEula
		}

		var input string
		_, _ = fmt.Scanln(&input)

		if input == "a" || input == "A" {
			return true, nil
		}

		if input == "e" || input == "E" {
			return false, errs.ErrExtractEula
		}

		return false, nil
	}

	licenseWindow := NewLicenseWindow(licenseTitle, licenseContents, promptText)
	if err := licenseWindow.Setup(); err != nil {
		return false, err
	}

	defer licenseWindow.Gui.Close()

	return licenseWindow.PromptUser()
}

func NewLicenseWindow(licenseTitle, licenseContents, promptText string) *LicenseWindowType {
	licenseWindow := &LicenseWindowType{}
	licenseContents = strings.ReplaceAll(licenseContents, "\r", "")
	licenseHeight := utils.CountLines(licenseContents)
	licenseMarginBottom := 10
	minTerminalWidth = len(promptText) + 3 // add margins and one space behind "(E)xtract"

	// The LayoutManager is called on events, like key press or window resize
	// and it is going to look more or less like
	//
	// +-- license file name -------+
	// |  License contents line 1   |
	// |  License contents line 2   |
	// |  License contents line N   |
	// +----------------------------+
	// +----------------------------+
	// | promptText                 |
	// +----------------------------+
	licenseWindow.LayoutManager = func(g *gocui.Gui) error {
		terminalWidth, terminalHeight = g.Size()

		// Compute and validate rectangles
		lbx, lby, lex, ley, pbx, pby, pex, pey, err := computeLayoutRects(terminalWidth, terminalHeight)
		if err != nil {
			return err
		}
		if v, err := g.SetView("license", lbx, lby, lex, ley); err != nil {
			if err != gocui.ErrUnknownView {
				log.Error("Cannot modify license window: ", err)
				return err
			}
			v.Wrap = true
			v.Title = licenseTitle
			// fmt.Fprintf(v, "tW: %d, tH: %d, lbx: %d, lby: %d, lex: %d, ley: %d, pbx: %d, pby: %d, pex: %d, pey: %d\n", terminalWidth, terminalHeight, lbx, lby, lex, ley, pbx, pby, pex, pey)
			fmt.Fprint(v, licenseContents)
		}

		if v, err := g.SetView("prompt", pbx, pby, pex, pey); err != nil {
			if err != gocui.ErrUnknownView {
				log.Error("Cannot modify prompt window: ", err)
				return err
			}
			fmt.Fprint(v, promptText)
		}

		_, err = g.SetCurrentView("license")
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}

		if LicenseAgreed != nil {
			return gocui.ErrQuit
		}

		if Extract {
			return errs.ErrExtractEula
		}

		return nil
	}

	licenseWindow.ScrollUp = func(g *gocui.Gui, v *gocui.View) error {
		return licenseWindow.Scroll(v, -1)
	}

	licenseWindow.ScrollDown = func(g *gocui.Gui, v *gocui.View) error {
		return licenseWindow.Scroll(v, 1)
	}

	licenseWindow.Scroll = func(v *gocui.View, dy int) error {
		if v != nil {
			ox, oy := v.Origin()
			_, terminalHeight := licenseWindow.Gui.Size()
			y := oy + dy
			if y < 0 || y+terminalHeight-licenseMarginBottom >= licenseHeight {
				return nil
			}
			if err := v.SetOrigin(ox, oy+dy); err != nil {
				log.Errorf("Cannot scroll to %v, %v: %v", ox, oy+dy, err.Error())
				return err
			}
		}
		return nil
	}

	licenseWindow.Agree = func(g *gocui.Gui, v *gocui.View) error {
		LicenseAgreed = &Agreed
		return gocui.ErrQuit
	}

	licenseWindow.Disagree = func(g *gocui.Gui, v *gocui.View) error {
		LicenseAgreed = &Disagreed
		return gocui.ErrQuit
	}

	licenseWindow.Extract = func(g *gocui.Gui, v *gocui.View) error {
		Extract = true
		return gocui.ErrQuit
	}

	return licenseWindow
}

// Helper to compute rectangles for license and prompt windows based on terminal size.
// Uses the same constants as LayoutManager (marginSize=1, promptWindowHeight=3).
// computeLayoutRects consolidates rectangle computation and validation.
// Returns rectangles for license and prompt windows or an error when the
// terminal is too small.
func computeLayoutRects(terminalWidth, terminalHeight int) (lbx, lby, lex, ley, pbx, pby, pex, pey int, err error) {
	marginSize := 1
	promptWindowHeight := 3
	const minTerminalHeight = 8

	if terminalWidth < minTerminalWidth || terminalHeight < minTerminalHeight {
		err = fmt.Errorf("increase window size to display license information and obtain user response to at least %dx%d", minTerminalWidth, minTerminalHeight)
		return
	}

	// License window dimensions
	lbx = 0
	lby = 0
	lex = terminalWidth - marginSize
	ley = terminalHeight - marginSize - promptWindowHeight

	// Validate license rect (fallback guard – should not trigger if min constraints above are correct)
	if lbx >= lex || lby >= ley {
		err = fmt.Errorf("increase window size to display prompt window and obtain user response to at least %dx%d", minTerminalWidth, minTerminalHeight)
		return
	}

	// Prompt window dimensions
	pbx = lbx
	pby = ley + marginSize
	pex = lex
	pey = terminalHeight - marginSize
	return
}

func (l *LicenseWindowType) Setup() error {
	log.Debug("Setting up UI to display license")

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Error("Cannot initialize UI: ", err)
		return err
	}

	terminalWidth, terminalHeight = g.Size()
	g.SetManagerFunc(l.LayoutManager)

	bindings := []struct {
		key         interface{}
		funcPointer func(g *gocui.Gui, v *gocui.View) error
	}{
		// Agree with 'a' or 'A'
		{'a', l.Agree},
		{'A', l.Agree},

		// Disagree with 'd'  or 'D'
		{'d', l.Disagree},
		{'D', l.Disagree},

		// Extract with 'e'  or 'E'
		{'e', l.Extract},
		{'E', l.Extract},

		// Scroll up with mouse/page/arrow up
		{gocui.MouseWheelUp, l.ScrollUp},
		{gocui.KeyArrowUp, l.ScrollUp},
		{gocui.KeyPgup, l.ScrollUp},

		// Scroll down with mouse/page/arrow down, enter or space
		{gocui.MouseWheelDown, l.ScrollDown},
		{gocui.KeyPgdn, l.ScrollDown},
		{gocui.KeyArrowDown, l.ScrollDown},
		{gocui.KeyEnter, l.ScrollDown},
		{gocui.KeySpace, l.ScrollDown},

		// Exit with Ctrl+C
		{gocui.KeyCtrlC, quit},
	}

	for _, binding := range bindings {
		_ = g.SetKeybinding("license", binding.key, gocui.ModNone, binding.funcPointer)
	}

	l.Gui = g
	return nil
}

func (l *LicenseWindowType) PromptUser() (bool, error) {
	log.Debug("Prompting user for license agreement")
	err := l.Gui.MainLoop()
	if err != nil && err != gocui.ErrQuit && err != errs.ErrExtractEula {
		//		log.Error("Cannot obtain user response: ", err)
		return false, err
	}

	if LicenseAgreed != nil {
		return *LicenseAgreed, nil
	}

	if Extract {
		return false, errs.ErrExtractEula
	}

	return false, nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	log.Warn("Aborting license agreement")
	return gocui.ErrQuit
}
