/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/spf13/cobra"
)

var All = []*cobra.Command {
	PackCmd,
	PdscCmd,
}
