/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLogFormatter(t *testing.T) {
	for _, test := range []struct {
		name    string
		level   log.Level
		message string
		want    string
	}{
		{
			name:    "single line",
			level:   log.ErrorLevel,
			message: "no targets found",
			want:    "E: no targets found\n",
		},
		{
			name:    "board selection",
			level:   log.ErrorLevel,
			message: "board selection is ambiguous or unresolved; specify a board explicitly:\nNXP::LPCXpresso802\nNXP::LPCXpresso804",
			want:    "E: board selection is ambiguous or unresolved; specify a board explicitly:\nE: NXP::LPCXpresso802\nE: NXP::LPCXpresso804\n",
		},
		{
			name:    "device selection",
			level:   log.ErrorLevel,
			message: "device selection is ambiguous or unresolved; specify a device explicitly:\nVendor::ChipA\nVendor::ChipB",
			want:    "E: device selection is ambiguous or unresolved; specify a device explicitly:\nE: Vendor::ChipA\nE: Vendor::ChipB\n",
		},
		{
			name:    "multiline warning",
			level:   log.WarnLevel,
			message: "first line\nsecond line",
			want:    "W: first line\nW: second line\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			formatted, err := (&LogFormatter{}).Format(&log.Entry{Level: test.level, Message: test.message})
			require.NoError(t, err)
			require.Equal(t, test.want, string(formatted))
		})
	}
}
