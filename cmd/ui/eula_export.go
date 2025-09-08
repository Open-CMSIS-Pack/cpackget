/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package ui

// NOTE: These are small exported wrappers intended for tests.
// They expose internal layout computations without changing runtime behavior.

// ComputeLayoutRectsForTest exposes computeLayoutRects for tests.
func ComputeLayoutRectsForTest(w, h int) (int, int, int, int, int, int, int, int, error) {
	return computeLayoutRects(w, h)
}
