/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"errors"
)

var (
	// Errors related to package file name
	ErrBadPackName                 = errors.New("bad pack name: it should follow the standard PackVendor.PackName.PackVersion.pack")
	ErrBadPackNameInvalidVendor    = errors.New("bad pack name: pack vendor should be a string containing letters")
	ErrBadPackNameInvalidName      = errors.New("bad pack name: pack name should be a string containing letters")
	ErrBadPackNameInvalidVersion   = errors.New("bad pack name: pack version should be versioned like 0.0.0, and optionally have a suffix containing letters")
	ErrBadPackNameInvalidExtension = errors.New("bad pack name: pack extension should be \"pdsc\"")

	// Errors related to package content
	ErrPdscNotFound    = errors.New("pdsc not found")
	ErrPdscEntryExists = errors.New("pdsc entry exists already")

	// Errors related to network
	ErrBadRequest            = errors.New("bad request")
	ErrFailedDownloadingFile = errors.New("failed to download file")

	// Errors related to file system
	ErrFailedCreatingFile        = errors.New("failed to create a local file")
	ErrFailedWrittingToLocalFile = errors.New("failed writing HTTP stream to local file")
	ErrFailedDecompressingFile   = errors.New("fail to decompress file")
	ErrFailedInflatingFile       = errors.New("fail to inflate file")
	ErrFileNotFound              = errors.New("file not found")

	// Errors that can't be be predicted
	ErrUnknownBehavior = errors.New("unknown behavior")
)
