/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// sigs holds signals to be monitored
var sigs chan os.Signal

// terminationRequested is a boolean flag that needs to be checked on
// long operationgs. The monitoring thread will use this to notify the
// main thread about a termination request.
var terminationRequested bool

// startSignalWatcher spins off a thread monitoring termination signals
// and retuns a function that returns whether termination was requested
func StartSignalWatcher() {
	log.Debug("Starting monitoring thread")

	// Create a channel to receive signals and pass to the monitoring thread
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM) // SA1016: syscall.SIGKILL cannot be trapped

	terminationRequested = false

	// Spin off the monitoring thread
	go func() {
		sig := <-sigs
		log.Debugf("Monitoring thread detected a signal: %v", sig)
		terminationRequested = true
	}()

	// Function that needs running to check if a termination request
	// has been triggered
	ShouldAbortFunction = func() bool {
		return terminationRequested
	}
}

// stopSignalWatcher sends a fake signal to the monitoring thread
// making it terminate
func StopSignalWatcher() {
	sigs <- syscall.SIGTERM
}
