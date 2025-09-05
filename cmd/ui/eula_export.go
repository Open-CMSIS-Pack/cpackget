/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package ui

// NOTE: These are small exported wrappers intended for tests.
// They expose internal layout computations without changing runtime behavior.

// ComputeLicenseAndPromptRectsForTest exposes computeLicenseAndPromptRects for tests.
func ComputeLicenseAndPromptRectsForTest(terminalWidth, terminalHeight int) (int, int, int, int, int, int, int, int) {
	return computeLicenseAndPromptRects(terminalWidth, terminalHeight)
}

// ValidateLicenseRectForTest exposes validateLicenseRect for tests.
func ValidateLicenseRectForTest(bx, by, ex, ey int) error { return validateLicenseRect(bx, by, ex, ey) }

// ValidatePromptRectForTest exposes validatePromptRect for tests.
func ValidatePromptRectForTest(bx, by, ex, ey int) error { return validatePromptRect(bx, by, ex, ey) }
