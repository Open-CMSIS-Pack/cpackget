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
	t.Run("test comparing two equal versions with and without leading zeros", func(t *testing.T) {
		version1 := "01.02.03"
		version2 := "1.2.3"
		assert.True(utils.SemverCompare(version1, version2) == 0)
	})
	t.Run("test comparing two zero major versions with and without leading zeros", func(t *testing.T) {
		version1 := "00.02.03"
		version2 := "0.2.3"
		version3 := "0.2.4"
		assert.True(utils.SemverCompare(version1, version2) == 0)
		assert.True(utils.SemverCompare(version1, version3) < 0)
		assert.True(utils.SemverCompare(version2, version3) < 0)
	})
	t.Run("test comparing two versions with range suffix", func(t *testing.T) {
		version1 := "1.2.3"
		version2 := "1.2.4:2.3.4"
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

func TestSemverMajorMinor(t *testing.T) {
	assert := assert.New(t)

	t.Run("test major minor version", func(t *testing.T) {
		assert.Equal("1.2", utils.SemverMajorMinor("1.2.3"))
		assert.Equal("1.2", utils.SemverMajorMinor("01.2.3"))
		assert.Equal("1.2", utils.SemverMajorMinor("01.02.03"))
		assert.Equal("1.2", utils.SemverMajorMinor("1.02.03"))
	})
}

func S(v ...interface{}) []interface{} {
	return v
}

func TestSemverHasMeta(t *testing.T) {
	assert := assert.New(t)

	t.Run("test has meta", func(t *testing.T) {
		assert.Equal(S("", false), S(utils.SemverHasMeta("1.2.3")))
		assert.Equal(S("meta", true), S(utils.SemverHasMeta("1.2.3+meta")))
	})
}

func TestSemverStripMeta(t *testing.T) {
	assert := assert.New(t)

	t.Run("test strip meta", func(t *testing.T) {
		assert.Equal("1.2.3", utils.SemverStripMeta("1.2.3"))
		assert.Equal("1.2.3", utils.SemverStripMeta("1.2.3+meta"))
	})
}

func TestSemverCompareRange(t *testing.T) {
	assert := assert.New(t)

	t.Run("test version range compare", func(t *testing.T) {
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.3") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.3:") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.3:_") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", ":1.2.3") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.3:1.2.3") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.0:1.2.3") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.3:1.2.4") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.0:1.2.4") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.0") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", ":1.2.4") == 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.4") < 0)
		assert.True(utils.SemverCompareRange("1.2.3", ":1.2.0") > 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.4:1.2.5") < 0)
		assert.True(utils.SemverCompareRange("1.2.3", "1.2.0:1.2.1") > 0)
	})
}
