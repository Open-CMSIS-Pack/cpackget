/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"syscall"
	"testing"
	"time"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

func TestStartSignalWatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("test start and stop watching thread", func(t *testing.T) {
		utils.StartSignalWatcher()
		time.Sleep(time.Second / 10)
		assert.False(utils.ShouldAbortFunction())

		utils.StopSignalWatcher()
		time.Sleep(time.Second / 10)
		assert.True(utils.ShouldAbortFunction())
		utils.ShouldAbortFunction = nil
	})

	t.Run("test if it's really trapping ctrl-c", func(t *testing.T) {
		utils.StartSignalWatcher()
		assert.False(utils.ShouldAbortFunction())
		sendCtrlC(t, syscall.Getpid())
		time.Sleep(time.Second / 10)
		assert.True(utils.ShouldAbortFunction())
		utils.ShouldAbortFunction = nil
	})
}
