/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func AnyErr(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func ExitOnError(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}
