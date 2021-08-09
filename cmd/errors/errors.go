/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package errors

import (
	"errors"
)

var (
	// Errors related to package file name
	BadPackName                 = errors.New("bad pack name: it should follow the standard PackVendor.PackName.PackVersion.pack")
	BadPackNameInvalidVendor    = errors.New("bad pack name: pack vendor should be a string containing letters")
	BadPackNameInvalidName      = errors.New("bad pack name: pack name should be a string containing letters")
	BadPackNameInvalidVersion   = errors.New("bad pack name: pack version should be versioned like 0.0.0, and optionally have a suffix containing letters")
	BadPackNameInvalidExtension = errors.New("bad pack name: pack extension should be \"pdsc\", \"pack\" or \"zip\"")

	// Errors related to package content
	PdscNotFound         = errors.New("pdsc not found")
	PackAlreadyInstalled = errors.New("pack already installed")
	PdscEntryExists      = errors.New("pdsc already in index")

	// Errors related to network
	BadRequest            = errors.New("bad request")
	FailedDownloadingFile = errors.New("failed to download file")

	// Errors related to file system
	FailedCreatingFile        = errors.New("failed to create a local file")
	FailedWrittingToLocalFile = errors.New("failed writing HTTP stream to local file")
	FailedDecompressingFile   = errors.New("fail to decompress file")
	FailedInflatingFile       = errors.New("fail to inflate file")
	FileNotFound              = errors.New("file not found")

	// Errors that can't be be predicted
	UnknownBehavior = errors.New("unknown behavior")
)
