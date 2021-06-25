/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"os"
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
		Logger.Error(err.Error())
		os.Exit(-1)
	}
}
