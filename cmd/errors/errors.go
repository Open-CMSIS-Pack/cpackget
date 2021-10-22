/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package errors

import (
	"errors"
)

// Is returns true if err is equals to target
func Is(err, target error) bool {
	return err == target
}

var (
	// Errors related to package file name
	ErrBadPackName                 = errors.New("bad pack name: it should follow the standard PackVendor.PackName.PackVersion.pack")
	ErrBadPackNameInvalidVendor    = errors.New("bad pack name: pack vendor should be a string containing letters")
	ErrBadPackNameInvalidName      = errors.New("bad pack name: pack name should be a string containing letters")
	ErrBadPackNameInvalidVersion   = errors.New("bad pack name: pack version should be versioned like 0.0.0, and optionally have a suffix containing letters")
	ErrBadPackNameInvalidExtension = errors.New("bad pack name: pack extension should be \"pdsc\", \"pack\" or \"zip\"")
	ErrBadPackURL                  = errors.New("bad pack url: the url provided for this pack is malformed")

	// Errors related to package content
	ErrPdscFileNotFound     = errors.New("pdsc not found")
	ErrPackAlreadyInstalled = errors.New("pack already installed")
	ErrPackNotInstalled     = errors.New("pack not installed")
	ErrPackNotPurgeable     = errors.New("pack not purgeable")
	ErrPdscEntryExists      = errors.New("pdsc already in index")
	ErrPdscEntryNotFound    = errors.New("pdsc not found in index")
	ErrEula                 = errors.New("user does not agree with the pack's license")
	ErrExtractEula          = errors.New("user wants to extract embedded license only")
	ErrLicenseNotFound      = errors.New("embedded license not found")

	// Errors related to network
	ErrBadRequest            = errors.New("bad request")
	ErrFailedDownloadingFile = errors.New("failed to download file")

	// Errors related to file system
	ErrFailedCreatingFile        = errors.New("failed to create a local file")
	ErrFailedWrittingToLocalFile = errors.New("failed writing HTTP stream to local file")
	ErrFailedDecompressingFile   = errors.New("fail to decompress file")
	ErrFailedInflatingFile       = errors.New("fail to inflate file")
	ErrFailedCreatingDirectory   = errors.New("fail to create directory")
	ErrFileNotFound              = errors.New("file not found")
	ErrCopyingEqualPaths         = errors.New("failed copying files: source is the same as destination")
	ErrMovingEqualPaths          = errors.New("failed moving files: source is the same as destination")

	// Security errors
	ErrInsecureZipFileName = errors.New("zip file contains insecure characters: ../")
	ErrFileTooBig          = errors.New("files cannot be over 20G")
	ErrIndexPathNotSafe    = errors.New("index url path does not start with HTTPS")

	// Errors that can't be be predicted
	ErrUnknownBehavior = errors.New("unknown behavior")

	// Cmdline errors
	ErrIncorrectCmdArgs = errors.New("incorrect setup of command line arguments")

	// Errors on installation strucuture
	ErrCannotOverwritePublicIndex = errors.New("cannot overwrite original public index.pidx")
)
