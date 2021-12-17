/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

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
func startSignalWatcher() func() bool {
	log.Debug("Starting monitoring thread")

	// Create a channel to receive signals and pass to the monitoring thread
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM) // SA1016: syscall.SIGKILL cannot be trapped

	// Spin off the monitoring thread
	go func() {
		sig := <-sigs
		log.Debugf("Monitoring thread detected a signal: %v", sig)
		terminationRequested = true
	}()

	// Returns a function that needs running to check if a termination request
	// has been triggered
	return func() bool {
		return terminationRequested
	}
}

// stopSignalWatcher sends a fake signal to the monitoring thread
// making it terminate
func stopSignalWatcher() {
	sigs <- syscall.SIGTERM
}
