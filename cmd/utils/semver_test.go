/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

func TestSemverCompare(t *testing.T) {
	assert := assert.New(t)

	t.Run("test comparing two versions", func(t *testing.T) {
		version1 := "1.2.3"
		version2 := "1.2.4"
		assert.True(utils.SemverCompare(version1, version2) < 0)
		assert.True(utils.SemverCompare(version2, version1) > 0)
	})

	t.Run("test comparing two versions with leading zeros", func(t *testing.T) {
		version1 := "01.02.03"
		version2 := "01.02.04"
		assert.True(utils.SemverCompare(version1, version2) < 0)
		assert.True(utils.SemverCompare(version2, version1) > 0)
	})

	t.Run("test comparing two versions with and without leading zeros", func(t *testing.T) {
		version1 := "01.02.03"
		version2 := "1.2.4"
		assert.True(utils.SemverCompare(version1, version2) < 0)
		assert.True(utils.SemverCompare(version2, version1) > 0)
	})
}

func TestSemverMajor(t *testing.T) {
	assert := assert.New(t)

	t.Run("test major version", func(t *testing.T) {
		assert.Equal("1", utils.SemverMajor("1.2.3"))
		assert.Equal("1", utils.SemverMajor("01.2.3"))
		assert.Equal("1", utils.SemverMajor("01.02.03"))
		assert.Equal("1", utils.SemverMajor("1.02.03"))
	})
}
