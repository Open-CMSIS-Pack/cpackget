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

// AlreadyLogged returns true if the error log has already been logged
func AlreadyLogged(err error) bool {
	return err == ErrAlreadyLogged
}

var (
	// Errors related to package file name
	ErrBadPackName = errors.New("bad pack name: it should be one of the following formats: Vendor.Pack, Vendor.Pack.x.y.z, Vendor.Pack.x.y.z.pack, or Vendor.Pack.pdsc")
	ErrBadPackURL  = errors.New("bad pack url: the url provided for this pack is malformed")

	// Errors related to package content
	ErrPdscFileNotFound      = errors.New("pdsc not found")
	ErrPackNotInstalled      = errors.New("pack not installed")
	ErrPackNotPurgeable      = errors.New("pack not purgeable")
	ErrPdscEntryExists       = errors.New("pdsc already in index")
	ErrPdscEntryNotFound     = errors.New("pdsc not found in index")
	ErrEula                  = errors.New("user does not agree with the pack's license")
	ErrExtractEula           = errors.New("user wants to extract embedded license only")
	ErrLicenseNotFound       = errors.New("embedded license not found")
	ErrPackRootNotFound      = errors.New("no CMSIS Pack Root directory specified. Either the environment CMSIS_PACK_ROOT needs to be set or the path specified using the command line option -R/--pack-root string")
	ErrPackRootDoesNotExist  = errors.New("the specified CMSIS Pack Root directory does NOT exist! Please take a moment to review if the value is correct or create a new one via `cpackget init` command")
	ErrPdscFileTooDeepInPack = errors.New("pdsc file is too deep in pack file")

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
	ErrDirectoryNotFound         = errors.New("directory not found")
	ErrPathAlreadyExists         = errors.New("path already exists")
	ErrCopyingEqualPaths         = errors.New("failed copying files: source is the same as destination")
	ErrMovingEqualPaths          = errors.New("failed moving files: source is the same as destination")

	// Cryptography errors
	ErrBadPackIntegrity     = errors.New("bad pack integrity")
	ErrIntegrityCheckFailed = errors.New("checksum verification failed")
	ErrInvalidHashFunction  = errors.New("provided hash function is not supported")
	ErrKeyGenerationFailure = errors.New("error generating new private key")

	// Security errors
	ErrInsecureZipFileName = errors.New("zip file contains insecure characters: ../")
	ErrFileTooBig          = errors.New("files cannot be over 20G")
	ErrIndexPathNotSafe    = errors.New("index url path does not start with HTTPS")

	// Errors that can't be be predicted
	ErrUnknownBehavior = errors.New("unknown behavior")

	// Cmdline errors
	ErrIncorrectCmdArgs = errors.New("incorrect setup of command line arguments")

	// Errors on installation strucuture
	ErrCannotOverwritePublicIndex      = errors.New("cannot replace \"index.pidx\", use the flag \"-f/--force\" to force overwritting it")
	ErrPackPdscCannotBeFound           = errors.New("the URL for the pack pdsc file seems not to exist or it didn't return the file")
	ErrPackVersionNotFoundInPdsc       = errors.New("pack version not found in the pdsc file")
	ErrPackVersionNotLatestReleasePdsc = errors.New("pack version is not the latest in the pdsc file")
	ErrPackVersionNotAvailable         = errors.New("target pack version is not available")
	ErrPackURLCannotBeFound            = errors.New("URL for the pack cannot be determined. Please consider updating the public index. Ex: cpackget index --force https://keil.com/pack/index.pidx")

	// Hack to allow multiple error logs while still avoiding duplicating the last error log
	ErrAlreadyLogged = errors.New("already logged")

	// Error/Flag to detect when a user has requested early termination
	ErrTerminatedByUser = errors.New("terminated by user request")
)
