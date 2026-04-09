# cpackget — Design Documentation

## Table of Contents

1. [Introduction](#1-introduction)
2. [High-Level Architecture](#2-high-level-architecture)
3. [Module Overview](#3-module-overview)
4. [Directory Layout on Disk (Pack Root)](#4-directory-layout-on-disk-pack-root)
5. [CLI Layer (`cmd/commands/`)](#5-cli-layer-cmdcommands)
6. [Installer Core (`cmd/installer/`)](#6-installer-core-cmdinstaller)
7. [XML Data Model (`cmd/xml/`)](#7-xml-data-model-cmdxml)
8. [Utilities (`cmd/utils/`)](#8-utilities-cmdutils)
9. [Cryptography (`cmd/cryptography/`)](#9-cryptography-cmdcryptography)
10. [User Interface (`cmd/ui/`)](#10-user-interface-cmdui)
11. [Error Handling (`cmd/errors/`)](#11-error-handling-cmderrors)
12. [Key Data Flows](#12-key-data-flows)
13. [Version Resolution Strategy](#13-version-resolution-strategy)
14. [Security Model](#14-security-model)
15. [Concurrency Model](#15-concurrency-model)
16. [Design Patterns](#16-design-patterns)
17. [External Dependencies](#17-external-dependencies)
18. [Testing Strategy](#18-testing-strategy)

---

## 1. Introduction

**cpackget** is a command-line package manager for
[Open-CMSIS-Pack](https://www.open-cmsis-pack.org/) software packs,
developed as part of the CMSIS-Toolbox.
It enables developers to discover, install, update, and remove
CMSIS-Pack packages on their local systems.
The project is written in Go and licensed under Apache 2.0.

### Key Capabilities

- Install packs from the public index, URLs, or local files
- Manage a local pack root directory with cached downloads
- Resolve and install pack dependencies
- Update the public pack index and cached PDSC files
- List installed, cached, and publicly available packs
- Verify pack integrity via SHA-256 checksums
- Digitally sign and verify packs using X.509 certificates or PGP keys
- Present and manage End User License Agreements (EULA)

---

## 2. High-Level Architecture

```text
┌───────────────────────────────────────────────────────────────────┐
│                          CLI Entry Point                          │
│                         cmd/main.go                               │
│         (logging, signal handling, version, user-agent)           │
└───────────────────────────────┬───────────────────────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────────┐
│                     Command Layer (Cobra)                         │
│                     cmd/commands/*.go                             │
│  ┌──────┐  ┌────┐  ┌──────┐  ┌────────┐  ┌──────────────┐         │
│  │ add  │  │ rm │  │ list │  │ update │  │ update-index │  ...    │
│  └──┬───┘  └─┬──┘  └──┬───┘  └───┬────┘  └──────┬───────┘         │
└─────┼────────┼────────┼──────────┼──────────────┼─────────────────┘
      │        │        │          │              │
      ▼        ▼        ▼          ▼              ▼
┌───────────────────────────────────────────────────────────────────┐
│                    Installer Core Engine                          │
│                    cmd/installer/root.go                          │
│         (pack root management, index I/O, coordination)           │
│                                                                   │
│      ┌───────────────────────┐      ┌──────────────────────┐      │
│      │  cmd/installer/pack   │      │  cmd/installer/pdsc  │      │
│      │  (PackType: fetch,    │      │  (PdscType: install, │      │
│      │   install, uninstall, │      │   uninstall, local   │      │
│      │   validate, EULA)     │      │   repository mgmt)   │      │
│      └──────────┬────────────┘      └──────────┬───────────┘      │
└─────────────────┼──────────────────────────────┼──────────────────┘
                  │                              │
      ┌───────────┴─────┬──────────────────┬─────┴──────────┐
      ▼                 ▼                  ▼                ▼
┌───────────┐    ┌──────────────┐    ┌────────────┐    ┌─────────┐
│ cmd/xml/  │    │ cmd/utils/   │    │ cmd/crypto │    │ cmd/ui/ │
│ (PDSC,    │    │ (download,   │    │ (checksum, │    │ (EULA   │
│  PIDX     │    │  files, I/O, │    │  signing,  │    │  TUI)   │
│  parsing) │    │  semver)     │    │  verify)   │    │         │
└───────────┘    └──────────────┘    └────────────┘    └─────────┘
```

The architecture follows a layered design:

1. **CLI Layer** — Parses flags, checks arguments, and passes them to the installer
2. **Installer Core** — Coordinates all pack operations: add, remove, list, update
3. **Domain Types** — `PackType` and `PdscType` represent a pack/PDSC through its lifecycle
4. **Support Modules** — XML parsing, utilities, cryptography, and UI

---

## 3. Module Overview

| Package | Path | Responsibility |
| --- | --- | --- |
| `main` | `cmd/main.go` | Entry point, logging setup, signal watcher, version |
| `commands` | `cmd/commands/` | Cobra CLI command definitions and flag handling |
| `installer` | `cmd/installer/` | Core business logic for pack management |
| `xml` | `cmd/xml/` | PDSC and PIDX XML data structures and I/O |
| `utils` | `cmd/utils/` | File operations, HTTP downloads, semver, security utilities |
| `cryptography` | `cmd/cryptography/` | Checksum generation/verification, digital signatures |
| `ui` | `cmd/ui/` | Terminal UI for EULA display and interaction |
| `errors` | `cmd/errors/` | Predefined error types |

---

## 4. Directory Layout on Disk (Pack Root)

cpackget manages an on-disk directory structure referred to as the **Pack Root**. The default location is OS-dependent:

- **Windows:** `%LOCALAPPDATA%\Arm\Packs`
- **Linux/macOS:** `$XDG_CACHE_HOME/arm/packs` (fallback: `~/.cache/arm/packs`)

It can also be specified via the `CMSIS_PACK_ROOT` environment variable or the `--pack-root` flag.

```text
<PackRoot>/
├── .Download/                        # Cached downloaded .pack files
│   ├── Vendor.PackName.1.0.0.pack
│   ├── Vendor.PackName.1.0.0.pdsc
│   └── Vendor.PackName.1.0.0.LICENSE.txt   # Extracted licenses
├── .Local/
│   └── local_repository.pidx        # Index of locally-added PDSC packs
├── .Web/
│   ├── index.pidx                   # Public pack index (from Keil/Arm)
│   ├── cache.pidx                   # Local cache index (tracks cached PDSCs)
│   └── Vendor.PackName.pdsc         # Cached PDSC files for public packs
├── Vendor/
│   └── PackName/
│       └── 1.0.0/                   # Extracted pack contents
│           ├── Vendor.PackName.pdsc
│           └── ...pack files...
└── ...
```

### Key Files

| File | Purpose |
| --- | --- |
| `.Web/index.pidx` | Main public pack index, downloaded from the upstream source |
| `.Web/cache.pidx` | Locally-maintained cache tracking which PDSC files exist in `.Web/` |
| `.Local/local_repository.pidx` | Registry of packs added via local PDSC file references |

### File Permissions

As of v0.7.0, the pack root is **read-only by default**.
cpackget manages file permissions internally via
`LockPackRoot()` / `UnlockPackRoot()` and `SetReadOnly()` / `UnsetReadOnly()` utilities.
This prevents accidental modification of managed content.

---

## 5. CLI Layer (`cmd/commands/`)

The CLI is built with [Cobra](https://github.com/spf13/cobra).
Each command is defined in its own file and registered in `root.go`.

### Root Command (`root.go`)

Sets up the Cobra root command with global flags and the `configureInstaller` pre-run hook:

| Global Flag | Short | Description |
| --- | --- | --- |
| `--pack-root` | `-R` | Path to pack root (overrides `CMSIS_PACK_ROOT`) |
| `--verbose` | `-v` | Enable debug-level logging |
| `--quiet` | `-q` | Suppress all non-error output |
| `--concurrent-downloads` | `-C` | Max parallel HTTP connections (default: 5) |
| `--timeout` | `-T` | HTTP download timeout in seconds (0 = disabled) |
| `--version` | `-V` | Print version and exit |

The `configureInstaller` pre-run hook:

1. Resolves the pack root directory (flag → env var → OS default)
2. Sets the pack root via `installer.SetPackRoot()`
3. Validates its existence

### Command Summary

| Command | File | Description |
| --- | --- | --- |
| `init` | `init.go` | Creates pack root and downloads the public index |
| `add` | `add.go` | Installs packs from index, URL, local file, or PDSC |
| `rm` | `rm.go` | Removes installed packs (with optional cache purge) |
| `list` | `list.go` | Lists installed/cached/public packs with filtering |
| `update` | `update.go` | Updates installed packs to latest versions |
| `update-index` | `update_index.go` | Refreshes the public index and cached PDSC files |
| `checksum-create` | `checksum.go` | Generates `.checksum` digest files for packs |
| `checksum-verify` | `checksum.go` | Verifies pack integrity against `.checksum` files |
| `signature-create` | `signature.go` | Digitally signs packs (X.509 or PGP) |
| `signature-verify` | `signature.go` | Verifies signed packs |
| `connection` | `connection.go` | Tests online connectivity |

### Subcommands of `list`

| Subcommand | Description |
| --- | --- |
| `list` (default) | Lists all installed packs |
| `list --cached` | Lists packs in `.Download/` |
| `list --public` | Lists all non-deprecated packs from the public index |
| `list --public --deprecated` | Lists all packs from the public index |
| `list --deprecated` | Lists deprecated packs from the public index |
| `list --updates` | Lists packs with newer versions available |
| `list --filter` | Filters results (case-sensitive, accepts multiple expressions) |
| `list-required` | Lists dependencies of installed packs |

---

## 6. Installer Core (`cmd/installer/`)

This is the heart of cpackget, containing all business logic for managing packs.

### 6.1 Global State (`root.go`)

The `Installation` singleton (type `PacksInstallationType`) holds:

```go
type PacksInstallationType struct {
    PackRoot       string          // Absolute path to pack root
    WebDir         string          // .Web/ directory path
    DownloadDir    string          // .Download/ directory path
    LocalDir       string          // .Local/ directory path
    PublicIndex    xml.PidxXML     // Parsed public index (index.pidx)
    PublicIndexXML xml.PidxXML     // In-memory public index state
    LocalPidx      xml.PidxXML     // Parsed local repository index
    CacheIdx       xml.PidxXML     // Parsed cache index (cache.pidx)
    // ...
}
```

- `GetDefaultCmsisPackRoot()` — Returns OS-dependent default pack root path
- `AddPack()` — Main entry point for installing a pack
- `RemovePack()` — Main entry point for removing a pack
- `AddPdsc()` / `RemovePdsc()` — Manages local PDSC file references
- `UpdatePack()` — Updates one or all packs to latest versions
- `InitializeCache()` — Builds `cache.pidx` from existing PDSC files in `.Web/`
- `CheckConcurrency()` — Validates and adjusts the concurrent-downloads setting
- `DownloadPDSCFiles()` — Downloads all PDSC files from the public
  index in parallel, optionally skipping deprecated packs
- `UpdateInstalledPDSCFiles()` — Refreshes already-cached PDSC files from the index
- `UpdatePublicIndexIfOnline()` — Updates the public index only when connectivity is available
- `UpdatePublicIndex()` — Downloads and updates the public index and
  PDSC files, with option to skip deprecated PDSC files
- `ListInstalledPacks()` — Lists packs with various filter modes;
  supports `--deprecated` flag to show only deprecated packs
  (hidden by default in `--public` listing)
- `FindPackURL()` — Resolves a pack ID to a download URL from the index
- `SetPackRoot()` — Initializes the `Installation` singleton and directory paths
- `ReadIndexFiles()` — Loads `index.pidx`, `local_repository.pidx`, and `cache.pidx`
- `LockPackRoot()` / `UnlockPackRoot()` — Manages read-only permissions on pack root

### 6.2 PackType (`pack.go`)

Represents a single pack through all stages of its life: discovery → download → validation → installation → removal.

```go
type PackType struct {
    xml.PdscTag                    // Embedded: Vendor, Name, Version, URL
    path             string        // Source path (URL, file path, or pack ID)
    isPackID         bool          // True if input was a pack ID (vs file/URL)
    isPublic         bool          // True if found in public index
    isInstalled      bool          // True if already installed locally
    isDownloaded     bool          // True if cached in .Download/
    versionModifier  int           // Version resolution strategy
    targetVersion    string        // Resolved target version
    Requirements     Requirements  // Dependencies from PDSC
    zipReader        *zip.ReadCloser
    // ...
}
```

#### Pack Lifecycle Methods

```text
preparePack()  →  fetch()  →  validate()  →  install()
                                               ├── checkEula()
                                               ├── extract files
                                               └── update indexes
```

- `preparePack()` — Parses the pack path/ID, looks up metadata, checks if already installed
- `fetch()` — Downloads the `.pack` file or validates a local file reference
- `validate()` — Checks that the pack content is intact and contains a PDSC file
- `purge()` — Removes the cached `.pack` file from `.Download/`
- `install()` — Extracts the ZIP, handles EULA, updates cache index
- `uninstall()` — Removes extracted pack directory, cleans empty parent dirs
- `checkEula()` / `extractEula()` — Handles license agreement display and extraction
- `resolveVersionModifier()` — Picks the actual version to install based on a modifier (`@^`, `@~`, `@>=`, `@latest`)
- `loadDependencies()` — Parses the PDSC `<requirements>` tag for package dependencies
- `RequirementsSatisfied()` — Returns `true` if all pack dependencies are installed
- `PackID()` — Returns `Vendor.PackName` identifier
- `PackIDWithVersion()` — Returns `Vendor.PackName.Version` identifier
- `PackFileName()` — Returns `Vendor.PackName.Version.pack` filename
- `PdscFileName()` / `PdscFileNameWithVersion()` — Returns PDSC filename (with or without version)
- `GetVersion()` / `GetVersionNoMeta()` — Returns the resolved version (with or without metadata)
- `Lock()` / `Unlock()` — Sets or removes read-only permissions on the pack directory

### 6.3 PdscType (`pdsc.go`)

Represents packs registered through local PDSC file references (not from the public index).

```go
type PdscType struct {
    xml.PdscTag                 // Embedded: Vendor, Name, Version, URL
    path        string          // Local file path to the PDSC
}
```

- `preparePdsc()` — Validates the PDSC file and extracts pack metadata
- `toPdscTag()` — Converts to an `xml.PdscTag` so it can be written to an index file
- `install()` — Adds an entry to `local_repository.pidx`
- `uninstall()` — Removes the entry from `local_repository.pidx`

---

## 7. XML Data Model (`cmd/xml/`)

### 7.1 PIDX — Pack Index (`pidx.go`)

Represents the `.pidx` index files (`index.pidx`, `local_repository.pidx`, `cache.pidx`).

```go
type PidxXML struct {
    SchemaVersion string
    Vendor        string
    URL           string
    Timestamp     string
    Pindex        struct {
        Pdscs []PdscTag     // List of all pack references
    }
    // Internal lookup maps for O(1) access
    pdscList       map[string][]PdscTag // key → PdscTags
    pdscListName   map[string]string    // vendor.name → key
    deprecatedDate time.Time            // today UTC, set once per Read()
}

type PdscTag struct {
    URL          string `xml:"url,attr"`
    Vendor       string `xml:"vendor,attr"`
    Name         string `xml:"name,attr"`
    Version      string `xml:"version,attr"`
    Deprecated   string `xml:"deprecated,attr,omitempty"`
    Replacement  string `xml:"replacement,attr,omitempty"`
    isDeprecated bool   // cached flag, computed on insert
}
```

- `NewPidxXML()` — Creates a new PidxXML instance for a given file
- `GetFileName()` / `SetFileName()` — Access the underlying file path
- `GetListSizes()` — Returns the number of entries in both lookup maps
- `Clear()` — Resets all entries
- `AddPdsc()` / `AddReplacePdsc()` — Insert or update entries
- `ReplacePdscVersion()` — Updates the version of an existing entry
- `Empty()` — Checks if the index has no entries
- `RemovePdsc()` — Remove entries by key
- `HasPdsc()` — Returns the index of a PdscTag if present, or `PdscIndexNotFound`
- `ListPdscTags()` / `FindPdscTags()` / `FindPdscNameTags()` — Query with optional filtering
- `CheckTime()` — Validates 24-hour freshness for update throttling
- `Read()` / `Write()` — XML file I/O with timestamp handling
- `Key()` / `VName()` — Returns `Vendor.Name.Version` or `Vendor.Name` identifier
- `YamlPackID()` — Returns `Vendor::Name@Version` format
- `PackURL()` — Constructs the full `.pack` download URL (PdscTag method)
- `PdscFileName()` — Returns the `.pdsc` filename (PdscTag method)
- `IsDeprecated()` — Returns the cached deprecated flag.
  Computed via `computeIsDeprecated()` when a PdscTag is
  inserted (`Read`, `AddPdsc`, `AddReplacePdsc`).
  Uses `PidxXML.deprecatedDate` (today UTC, set once per
  `NewPidxXML`/`Read`) as reference (PdscTag method)

### 7.2 PDSC — Pack Description (`pdsc.go`)

Represents a `.pdsc` pack description file (PACK.xsd schema).

```go
type PdscXML struct {
    Vendor  string
    Name    string
    License string
    URL     string

    ReleasesTag struct {
        Releases []ReleaseTag      // <release> tags
    }

    RequirementsTag struct {
        Packages []PackagesTag     // <packages> groups inside <requirements>
    }

    FileName string
}

type ReleaseTag struct {
    Version string
    Date    string
    URL     string   // Override download URL per release
}

// PackagesTag wraps a <packages> group, which contains one or more <package> entries
type PackagesTag struct {
    Packages []PackageTag
}

type PackageTag struct {
    Vendor  string
    Name    string
    Version string   // Version range (e.g., "1.0.0:2.0.0")
}
```

- `NewPdscXML()` — Creates a new PdscXML instance for a given file
- `LatestVersion()` — Returns the newest release version
- `AllReleases()` — Lists all available versions
- `FindReleaseTagByVersion()` — Looks up a specific release by version string
- `Tag()` — Converts to a PdscTag for use in index files
- `Read()` / `Write()` — Loads and saves the PDSC XML file
- `BaseURL()` — Returns the pack URL with a trailing slash
- `PackURL()` — Constructs the download URL for a specific version
- `Dependencies()` — Parses requirements with version range resolution

---

## 8. Utilities (`cmd/utils/`)

### 8.1 Pack Parsing (`packs.go`)

This is how cpackget interprets user input. Validation helpers:

- `IsPackVendorNameValid()` / `IsPackNameValid()` — Checks vendor/pack names against the PACK.xsd regex
- `IsPackVersionValid()` — Checks that a version string matches the semver pattern
- `FormatPackVersion()` — Converts an internal `[Name, Vendor, Version]` triple to the `Vendor::Name@Version` format
- `FormatVersions()` — Converts internal version ranges (e.g. `1.0.0:_`) to display format (e.g. `>=1.0.0`)

The `ExtractPackInfo()` function parses any of these formats:

| Input Format | Example | Type |
| --- | --- | --- |
| Pack ID (dotted) | `Vendor.PackName.1.0.0` | Pack ID |
| Pack ID (YAML) | `Vendor::PackName@1.0.0` | Pack ID |
| Version modifier | `Vendor::PackName@^1.0.0` | Pack ID with modifier |
| Pack file path | `/path/to/Vendor.PackName.1.0.0.pack` | File reference |
| Pack URL | `https://host/Vendor.PackName.1.0.0.pack` | URL reference |
| PDSC file | `Vendor.PackName.pdsc` | PDSC reference |

Returns a `PackInfo` struct:

```go
type PackInfo struct {
    Location        string    // URL or file path (empty for pack IDs)
    Vendor          string
    Pack            string
    Version         string
    Extension       string    // ".pack", ".pdsc", or ""
    IsPackID        bool
    VersionModifier int
}
```

#### Version Modifiers

| Constant | Syntax | Meaning |
| --- | --- | --- |
| `ExactVersion` (0) | `@1.2.3` | Install exactly this version |
| `LatestVersion` (1) | `@latest` | Install the latest available version |
| `AnyVersion` (2) | (none) | Use any/latest if not installed |
| `GreaterVersion` (3) | `@>=1.2.3` | Install any version ≥ specified |
| `GreatestCompatibleVersion` (4) | `@^1.2.3` | Same major version, latest minor/patch |
| `PatchVersion` (5) | `@~1.2.3` | Same major.minor, latest patch |
| `RangeVersion` (6) | `1.0.0:2.0.0` | Within specified range |

### 8.2 Semantic Versioning (`semver.go`)

Extended semver comparison functions built on top of `golang.org/x/mod/semver`:

- `SemverCompare()` — Compares two version strings (handles leading zeros)
- `SemverCompareRange()` — Checks if a version is within a `low[:high]` range
- `SemverMajor()` / `SemverMajorMinor()` — Extracts version components
- `SemverHasMeta()` — Checks if a version string contains `+metadata`, returns the metadata and a boolean
- `SemverStripMeta()` — Removes `+metadata` suffix
- `VersionList()` — Joins a slice of versions into a comma-separated string (strips metadata)

### 8.3 File Operations and Downloads (`utils.go`)

- `FileURLToPath()` — Converts `file://` URIs to filesystem paths
- `SetEncodedProgress()` / `GetEncodedProgress()` — Enables/queries machine-readable progress mode
- `SetSkipTouch()` / `GetSkipTouch()` — Controls whether touch operations are skipped
- `SetUserAgent()` — Sets the HTTP User-Agent string for downloads
- `DownloadFile()` — HTTP GET with progress bar, ETag caching, and configurable timeout
- `CheckConnection()` — Connectivity test to a given URL
- `FileExists()` / `DirExists()` — Checks existence of files or directories
- `EnsureDir()` — Recursive directory creation
- `SameFile()` — Checks if two paths point to the same file (via `os.SameFile`)
- `CopyFile()` / `MoveFile()` — File manipulation with permission handling
- `ReadXML()` / `WriteXML()` — Read/write Go structs from/to XML files
- `ListDir()` — Lists files/directories with optional regex filtering (non-recursive)
- `TouchFile()` — Creates or updates the modify timestamp of a file
- `IsBase64()` — Checks whether a string is valid base64
- `IsEmpty()` — Checks whether a directory is empty
- `RandStringBytes()` — Generates a random alphanumeric string of length n
- `CountLines()` — Returns the number of lines in a string
- `FilterPackID()` — Matches a pack ID against a space-separated filter string
- `IsTerminalInteractive()` — Returns `true` if stdout is a character device
- `CleanPath()` — Path normalization
- `SetReadOnly()` / `SetReadOnlyR()` — Sets file/directory to read-only (optionally recursive)
- `UnsetReadOnly()` / `UnsetReadOnlyR()` — Sets file/directory to read-write (optionally recursive)
- `GetListFiles()` — Reads lines from a file and returns them as a pack argument list

### 8.4 Security Utilities (`security.go`)

- Constants: `MaxDownloadSize` (20 GB), `DownloadBufferSize` (4 KB)
- `SecureCopy()` — Size-limited streaming with abort support
- `SecureInflateFile()` — ZIP extraction with path traversal prevention (rejects `../`)

### 8.5 Signal Handling (`signal.go`)

- `StartSignalWatcher()` — Starts a goroutine that listens for `SIGINT`/`SIGTERM`
- `StopSignalWatcher()` — Stops the signal listener cleanly
- `ShouldAbortFunction` — Global callback checked during long-running operations

### 8.6 Encoded Progress (`encodedProgress.go`)

Machine-readable progress output for tool integration (IDEs, CI systems).

`NewEncodedProgress(max int64, instNo int, filename string)` creates a new progress tracker:

| Parameter | Description |
| --- | --- |
| `max` | Total size in bytes (used to calculate percentage) |
| `instNo` | Instance number (auto-incremented per download, links output to a filename) |
| `filename` | Name of the file being processed |

The `EncodedProgress` struct implements `io.Writer`, so it can be passed directly
to an `io.MultiWriter` alongside the file output.
Each `Write()` call updates the internal byte counter and emits a log line
when the percentage changes.

Output format — first message includes all fields, subsequent messages only percentage and count:

```text
[I<instNo>:F"<filename>",T<max>,P<percent>]    // initial
[I<instNo>:P<percent>,C<current>]               // updates
```

Field codes used across encoded progress messages:

| Code | Meaning |
| --- | --- |
| `I` | Instance number (always counts up), connected to the filename |
| `F` | Filename currently processed |
| `T` | Total bytes of file or number of files |
| `P` | Currently processed percentage |
| `C` | Currently processed bytes or number of files |
| `J` | Total number of files being processed |
| `L` | License file follows |
| `O` | Online connection status (`offline` / `online`) |

---

## 9. Cryptography (`cmd/cryptography/`)

### 9.1 Checksum Verification (`checksum.go`)

Generates and verifies `.checksum` files containing SHA-256 digests of all files inside a `.pack` (ZIP).

**File naming convention:** `Vendor.PackName.1.0.0.sha256.checksum`

**File format:** Standard digest file with lines of `<hex-digest>  <filename>`

- `WriteChecksumFile()` — Writes the digest file to disk
- `GenerateChecksum()` — Creates a `.checksum` file for a given pack
- `VerifyChecksum()` — Validates a pack against its `.checksum` file
- `getDigestList()` — Computes SHA-256 of every file in the ZIP

### 9.2 Digital Signatures (`signature.go`)

Signs packs by embedding a cryptographic signature in the ZIP comment field.

#### Signature Format

```text
cpackget-v<version>:<mode>:<payload1>[:<payload2>]
```

| Mode | Code | Payload | Security Level |
| --- | --- | --- | --- |
| Full (X.509) | `f` | `<base64-cert>:<base64-signed-hash>` | Highest — integrity + authenticity |
| Cert-only | `c` | `<base64-cert>` | Medium — authenticity only (no digest) |
| PGP | `p` | `<base64-pgp-message>` | High — integrity + authenticity |

#### Signing Flow (X.509 Full Mode)

```text
.pack file
  │
  ├─→ calculatePackHash()     →  SHA-256 of ZIP content
  ├─→ loadCertificate()       →  Parse X.509 certificate
  ├─→ signPackHashX509()      →  RSA-sign the hash with private key
  ├─→ Build signature string  →  "cpackget-v...:f:<cert>:<signed-hash>"
  └─→ embedPack()             →  Write signature into ZIP comment → .pack.signed
```

Main entry points:

- `SignPack()` — Top-level function for creating a pack signature (X.509 or PGP)
- `VerifyPackSignature()` — Top-level function for verifying a signed pack

#### Verification Flow

```text
.pack.signed file
  │
  ├─→ validateSignatureScheme()  →  Parse mode from ZIP comment
  ├─→ Extract certificate/key    →  Decode base64 payload
  ├─→ calculatePackHash()        →  SHA-256 of ZIP content
  └─→ verifyPackFullSignature()  →  RSA-verify signed hash with certificate
```

### 9.3 Crypto Utilities (`utils.go`)

- `calculatePackHash()` — SHA-256 hash of ZIP file contents
- `detectKeyType()` — Identifies PKCS#1 vs. PKCS#8 private key format
- `displayCertificateInfo()` — Pretty-prints X.509 certificate details
- `getUnlockedKeyring()` — Loads and unlocks PGP keyrings
- `isPrivateKeyFromCertificate()` — Validates that a private key matches a certificate
- `sanitizeVersionForSignature()` — Normalizes a version string for embedding in a signature

---

## 10. User Interface (`cmd/ui/`)

### EULA Display (`eula.go`)

An interactive terminal UI built with [gocui](https://github.com/jroimartin/gocui) for displaying pack licenses:

```text
┌─── License: PackName ─────────────────────┐
│                                           │
│  <scrollable license text>                │
│                                           │
│  (Arrow keys / Page Up / Page Down)       │
└───────────────────────────────────────────┘
┌───────────────────────────────────────────┐
│ [A]ccept   [D]ecline   [E]xtract License  │
└───────────────────────────────────────────┘
```

Key functions:

- `DisplayAndWaitForEULA()` — High-level function that shows a license and returns whether the user accepted
- `NewLicenseWindow()` — Creates and configures the TUI window
- `Setup()` — Initializes the gocui layout and key bindings
- `PromptUser()` — Runs the event loop and returns the user's choice

**Fallback behavior:** When the terminal is not interactive (pipes, CI), falls back to `stdin` prompt.

**Minimum terminal dimensions:** 8 rows × prompt text width.

---

## 11. Error Handling (`cmd/errors/`)

All errors are predefined constants in `errors.go`, allowing consistent error checking with `errors.Is()`:

| Category | Examples |
| --- | --- |
| Pack naming | `ErrBadPackName`, `ErrBadPackURL` |
| Pack state | `ErrPackNotInstalled`, `ErrPackVersionNotAvailable`, `ErrPackAlreadyInstalled` |
| File system | `ErrFileNotFound`, `ErrFailedDecompressingFile`, `ErrInsecureZipFileName` |
| Network | `ErrOffline`, `ErrHTTPtimeout`, `ErrBadRequest` |
| Security | `ErrIntegrityCheckFailed`, `ErrCannotVerifySignature`, `ErrFileTooBig` |
| Index | `ErrIndexPathNotSafe`, `ErrInvalidPublicIndexReference` |
| EULA | `ErrEula` (internal, not surfaced to user) |

Helper functions:

- `Is()` — Wraps `errors.Is()` for convenience
- `AlreadyLogged()` — Wraps errors to prevent the same message from
  being logged twice as the error travels up the call stack

---

## 12. Key Data Flows

### 12.1 Pack Installation (`cpackget add`)

```text
User input (pack ID / URL / path / PDSC)
  │
  ▼
commands/add.go — Parse flags, iterate pack arguments
  │
  ▼
installer.AddPack()
  ├── ReadIndexFiles() — Load index.pidx, local_repository.pidx, cache.pidx
  ├── preparePack() — Parse input, resolve metadata, check install status
  ├── FindPackURL() — Look up download URL in public index (if pack ID)
  ├── pack.fetch() — Download .pack file or validate local file
  ├── pack.install()
  │     ├── Validate ZIP contents and PDSC presence
  │     ├── Check EULA (display TUI or auto-accept)
  │     ├── Extract files to <PackRoot>/Vendor/Name/Version/
  │     ├── Cache .pack in .Download/
  │     └── Update cache.pidx
  └── pack.loadDependencies() — Recursively call AddPack() for requirements
```

### 12.2 Public Index Update (`cpackget update-index`)

```text
commands/update_index.go
  │
  ▼
installer.UpdatePublicIndex()
  ├── Check timestamp (24-hour freshness rule)
  ├── Download new index.pidx from upstream URL
  ├── Compare old vs. new entries
  ├── Download updated/new PDSC files (concurrent, via semaphore)
  │     └── Skip PDSC files where Deprecated date ≤ today
  ├── Update cache.pidx to reflect changes
  └── Remove deprecated entries
```

### 12.3 Pack Removal (`cpackget rm`)

```text
commands/rm.go
  │
  ▼
installer.RemovePack()
  ├── ReadIndexFiles()
  ├── preparePack() — Locate installed pack
  ├── pack.uninstall() — Remove extracted directory
  ├── (if --purge) pack.purge() — Remove from .Download/
  └── Update cache.pidx
```

### 12.4 Version Resolution

```text
User specifies: Vendor::PackName@^1.2.0
  │
  ▼
ExtractPackInfo() → versionModifier = GreatestCompatibleVersion
  │
  ▼
resolveVersionModifier()
  ├── Query public index for all available versions
  ├── Filter by major version = 1
  ├── Sort by semver
  └── Return highest match (e.g., 1.9.3)
```

---

## 13. Version Resolution Strategy

cpackget supports flexible version matching, similar to npm or apt:

| Modifier | Symbol | Resolution Logic |
| --- | --- | --- |
| Exact | `@1.2.3` | Must match exactly |
| Latest | `@latest` | Highest available version |
| Any | (omitted) | Latest if not installed; current if installed |
| Greater-or-equal | `@>=1.2.3` | Highest version ≥ 1.2.3 |
| Compatible (caret) | `@^1.2.3` | Highest with same major (≥1.2.3, <2.0.0) |
| Patch (tilde) | `@~1.2.3` | Highest with same major.minor (≥1.2.3, <1.3.0) |
| Range | `1.0.0:2.0.0` | Any version in [1.0.0, 2.0.0] |

Versions follow [Semantic Versioning 2.0](https://semver.org/) with optional
pre-release (`-alpha.1`) and build metadata (`+meta`) suffixes.
Build metadata is stripped during comparison.

---

## 14. Security Model

### 14.1 Download Security

- **Size limits:** `MaxDownloadSize` (20 GB) prevents decompression bombs
- **Secure copy:** `SecureCopy()` enforces byte-level size limits during streaming
- **Path traversal prevention:** `SecureInflateFile()` rejects ZIP entries containing `../`
- **Insecure ZIP filename detection:** `ErrInsecureZipFileName` error

### 14.2 Integrity Verification

- SHA-256 checksums of individual files within `.pack` archives
- Checksum files use a standard digest format so other tools can read them too

### 14.3 Authenticity Verification

- **X.509 Full mode:** Certificate + RSA-signed SHA-256 hash provides integrity and authenticity
- **PGP mode:** Detached PGP signature of pack hash
- **Certificate validation:** Expiry checks, key usage validation, key-cert matching

### 14.4 Pack Root Permissions

- Pack root directory and contents are set read-only after installation
- Permissions are managed exclusively by cpackget to prevent tampering
- `LockPackRoot()` / `UnlockPackRoot()` are called before and after every write operation

### 14.5 Network Security

- Proxy support via `HTTP_PROXY` / `HTTPS_PROXY` environment variables
- Configurable TLS certificate verification (can be disabled for testing)
- User-agent identification: `CMSIS-Toolbox cpackget/<version>`

---

## 15. Concurrency Model

### Parallel Downloads

Batch downloads (e.g., `update-index`) use a semaphore to limit the number of parallel goroutines:

```go
sem := semaphore.NewWeighted(int64(concurrentDownloads))

for _, pdsc := range pdscsToUpdate {
    sem.Acquire(ctx, 1)
    go func(p PdscTag) {
        defer sem.Release(1)
        downloadPdsc(p)
    }(pdsc)
}
```

- Default concurrency: 5 parallel connections (`--concurrent-downloads`)
- Setting to 0 disables parallelism entirely
- Results are collected in a thread-safe `lockedSlice` struct

### Signal Handling

A dedicated goroutine listens for OS signals (`SIGINT`, `SIGTERM`).
Long-running operations check the abort flag via `ShouldAbortFunction`
so they can stop cleanly when the user presses Ctrl+C.

### Progress Tracking

`EncodedProgress` uses a mutex so multiple goroutines can safely update
progress at the same time.
The output format is machine-readable, designed for IDE and tool integration.

---

## 16. Design Patterns

| Pattern | Usage |
| --- | --- |
| **Singleton** | `installer.Installation` — Single global pack root state |
| **Command** | Each Cobra command wraps a CLI action into its own object |
| **Factory** | `preparePack()` / `preparePdsc()` — Build pack/PDSC objects from raw user input |
| **Strategy** | Version modifiers pick the right version-matching logic at runtime |
| **Template Method** | `configureInstaller` pre-run hook runs the same setup for every command |
| **Observer** | Signal watcher notifies long-running operations of user interrupts |
| **Wrapper** | `installer.AddPack()` / `RemovePack()` — simple interface hiding complex steps |
| **Fixed Error Constants** | Predefined error values for consistent error-type checking |

---

## 17. External Dependencies

| Dependency | Version | Purpose |
| --- | --- | --- |
| `spf13/cobra` | v1.10.2 | CLI framework and command routing |
| `spf13/pflag` | v1.0.10 | POSIX/GNU-style flag parsing (used by Cobra) |
| `spf13/viper` | v1.21.0 | Configuration management |
| `sirupsen/logrus` | v1.9.4 | Structured logging |
| `schollz/progressbar` | v3.19.0 | Terminal progress bars for downloads |
| `jroimartin/gocui` | v0.5.0 | Terminal UI for EULA display |
| `ProtonMail/gopenpgp` | v2.9.0 | PGP key handling and signature creation |
| `golang.org/x/mod` | v0.33.0 | Semantic version comparison |
| `golang.org/x/sync` | v0.19.0 | Semaphore for concurrent downloads |
| `golang.org/x/net` | v0.51.0 | Network utilities |
| `golang.org/x/term` | v0.40.0 | Terminal capability detection |
| `stretchr/testify` | v1.11.1 | Test assertions and mocking |
| `lu4p/cat` | v0.1.5 | File content type detection |

**Go version:** 1.25.0

---

## 18. Testing Strategy

### Unit Tests

Each package has comprehensive unit tests (`*_test.go` files)
co-located with production code.
Tests use the `testify` assertion library.

### Platform-Specific Tests

- `root_unix_test.go` / `root_windows_test.go` — OS-specific file permission behavior
- `signal_unix_test.go` / `signal_windows_test.go` — Signal handling differences

### Test Data

The `testdata/` directory provides fixtures for testing:

| Path | Purpose |
| --- | --- |
| `testdata/*.pidx` | Sample and malformed index files for parsing tests |
| `testdata/devpack/` | Mock pack PDSC files at different versions |
| `testdata/integration/` | Full integration test fixtures with versioned packs |
| `testdata/utils/` | File listing test data |

### Integration Testing

Integration test fixtures in `testdata/integration/` include:

- Sample public indexes (`SamplePublicIndex.pidx`)
- Packs at various versions (`0.1.0/`, `1.2.3/`, `1.2.4/`)
- Pre-release versions (`1.2.3-alpha.1.0/`)
- Metadata versions (`1.2.3+meta/`)
- Dependency chains (`dependencies/`)
- Concurrent download scenarios (`concurrent/`)
- Bad/malformed inputs for negative testing

### Build and CI

- **Makefile targets:** `make test`, `make build`, `make coverage-report`
- **CI pipelines:** GitHub Actions for build, test, and release workflows
- **Linting:** GolangCI-Lint for static analysis
