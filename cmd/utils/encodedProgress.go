/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

type EncodedProgress struct {
	mu             sync.Mutex
	total          int64
	current        int
	currentPercent int
	instanceNo     int
	name           string
}

func NewEncodedProgress(max int64, instNo int, filename string) *EncodedProgress {
	return &EncodedProgress{
		total:      max,
		instanceNo: instNo,
		name:       filename,
	}
}

func (p *EncodedProgress) Add(count int) int {
	p.mu.Lock()
	newCount := count
	p.current += newCount
	p.Print()
	p.mu.Unlock()
	return newCount
}

func (p *EncodedProgress) Write(bs []byte) (int, error) {
	return p.Add(len(bs)), nil
}

/* Encodes information to show progress when called by GUI or other tools
 * I: Instance number (always counts up), connected to the filename
 * F: Filename currently processed
 * T: Total bytes of file or numbers of files
 * P: Currently processed percentage
 * C: Currently processed bytes or numbers of files
 */
func (p *EncodedProgress) Print() {
	newPercent := int(float64(p.current) / float64(p.total) * 100)
	if p.currentPercent != newPercent {
		if p.currentPercent == 0 {
			log.Infof("[I%d:F%s,T%d,P%d]", p.instanceNo, p.name, p.total, newPercent)
		} else {
			log.Infof("[I%d:P%d,C%d]", p.instanceNo, newPercent, p.current)
		}
		p.currentPercent = newPercent
	}
}
