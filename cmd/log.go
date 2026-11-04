/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// LogFormatter is cpackget's basic log formatter
type LogFormatter struct{}

// Format prints out logs like "I: some message", where the first letter indicates (I)NFO, (D)EBUG, (W)ARNING or (E)RROR
func (s *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	prefix := level[0:1] + ": "
	msg := fmt.Sprintf("%s%s\n", prefix, strings.ReplaceAll(entry.Message, "\n", "\n"+prefix))
	return []byte(msg), nil
}
