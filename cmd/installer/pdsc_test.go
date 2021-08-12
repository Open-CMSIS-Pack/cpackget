/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package installer_test

import (
	"testing"

)

func TestPackInstall(t *testing.T) {

	t.Run("test installing same pdsc with different location", func (t *testing.T) {
		// installing same pdsc with different location should be allowed
	})

	t.Run("test installing updated version", func (t *testing.T) {
		// installing same pdsc with newer version should raise an error
	})
}
