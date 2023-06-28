/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var gText string

// LogCapturer reroutes testing.T log output
type LogCapturer interface {
	Release()
}

type logCapturer struct {
	*testing.T
	origOut io.Writer
}

func (tl logCapturer) Write(p []byte) (n int, err error) {
	//tl.Logf((string)(p))
	gText += string(p)
	return len(p), nil
}

func (tl logCapturer) Release() {
	logrus.SetOutput(tl.origOut)
}

// CaptureLog redirects logrus output to testing.Log
func CaptureLog(t *testing.T) LogCapturer {
	lc := logCapturer{T: t, origOut: logrus.StandardLogger().Out}
	if !testing.Verbose() {
		logrus.SetOutput(lc)
	}
	gText = ""
	return &lc
}

func Add() {
	maxCnt := int64(10)
	progressWriter := utils.NewEncodedProgress(maxCnt, 0, "Test progressbar")

	for i := int64(0); i < maxCnt; i++ {
		progressWriter.Add(1)
	}

}

func TestEncodedProgres(t *testing.T) {
	assert := assert.New(t)

	t.Run("test encoded progress", func(t *testing.T) {
		Log := CaptureLog(t)
		defer Log.Release()

		length := int64(10)
		instCnt := 0
		fileBase := "Testing"
		progressWriter := utils.NewEncodedProgress(length, instCnt, fileBase)

		for i := int64(0); i < length; i++ {
			progressWriter.Add(1)
		}

		assert.True(gText == "I: [I0:F\"Testing\",T10,P10]\nI: [I0:P20,C2]\nI: [I0:P30,C3]\nI: [I0:P40,C4]\nI: [I0:P50,C5]\nI: [I0:P60,C6]\nI: [I0:P70,C7]\nI: [I0:P80,C8]\nI: [I0:P90,C9]\nI: [I0:P100,C10]\n")
	})

	t.Run("test encoded progress with write interface", func(t *testing.T) {
		Log := CaptureLog(t)
		defer Log.Release()

		text := "ProgressWriter: Write interface"
		length := int64(len(text))
		instCnt := 0
		fileBase := "Testing"
		progressWriter := utils.NewEncodedProgress(length, instCnt, fileBase)

		fmt.Fprint(progressWriter, text)

		assert.True(gText == "I: [I0:F\"Testing\",T31,P100]\n")
	})

}
