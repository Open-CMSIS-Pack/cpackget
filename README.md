[![Release](https://github.com/open-cmsis-pack/cpackget/actions/workflows/release.yml/badge.svg)](https://github.com/Open-CMSIS-Pack/cpackget/actions/workflows/release.yml)
[![Build](https://github.com/open-cmsis-pack/cpackget/actions/workflows/build.yml/badge.svg)](https://github.com/open-cmsis-pack/cpackget/actions/workflows/build.yml/badge.svg)
[![Tests](https://github.com/open-cmsis-pack/cpackget/actions/workflows/test.yml/badge.svg)](https://github.com/open-cmsis-pack/cpackget/actions/workflows/test.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/open-cmsis-pack/cpackget)](https://goreportcard.com/report/github.com/open-cmsis-pack/cpackget)
[![GoDoc](https://godoc.org/github.com/open-cmsis-pack/cpackget?status.svg)](https://godoc.org/github.com/open-cmsis-pack/cpackget)

# cpackget: Open-CMSIS-Pack Package Installer

This utility allows embedded developers to install (or uninstall) Open-CMSIS-Pack software packs to their local environments.

## How to get `cpackget`

Please visit the [latest stable release](https://github.com/Open-CMSIS-Pack/cpackget/releases/latest) page and download the binary for your system, decompress it and run the binary named `cpackget` in the folder.

## Usage

`cpackget` installs packs via a CLI interface and it's intended to be easy to understand it. Here is how the command line looks like:

```bash
$ cpackget pack|pdsc add|rm <pack-path>

Options:

  -h, --help        Help for cpackget.
  -V, --version     Outout the version number of cpackget and exit.
  -v, --verbose     Set verbosiness: None (Errors only), -v (Info) and -vv (Debugging)

Use "cpackget [command] --help" for more information about a command.
```

For example, if one wanted help removing a pack, running `cpackget pack rm --help` would print out useful information on the subject.

### Examples

The commands below are examples on how to add packs:
- `cpackget pack add path/to/Vendor.PackName.x.y.z.pack`
- `cpackget pack add https://vendor.com/example/Vendor.PackName.x.y.z.pack`

If the pack is specified by an URL, cpackget command will download `Vendor.PackName.x.y.z.pack` before installing it under the **Open-CMSIS-Pack** root folder.
This folder should be specified via system environment variable named **CMSIS_PACK_ROOT**. If empty, cpackget will attempt creating a folder called `.cpackget` in the local directory and that will become the installation folder.

The command below is an example how to add packs via PDSC files:
- `cpackget pdsc add path/to/Vendor.PackName.pdsc`

Note that for adding packs via PDSC files is not possible to provide an URL as input. Only local files are allowed.

Removing packs uses slightly different pack specification. Here are a few examples
- `cpackget pack rm Vendor.PackName.x.y.z`: removes only the *x.y.z* version of the `Vendor.PackName` pack.
- `cpackget pack rm Vendor.PackName`: removes all installed versions of the `Vendor.PackName` pack.
- `cpackget pack rm --purge Vendor.PackName`: using `--purge` triggers removal of any downloaded files.

And for removing packs that were installed via PDSC files, consider the example commands below:
- `cpackget pdsc rm Vendor.PackName.x.y.z`: removes only the *x.y.z* version of the `Vendor.PackName` pack in the local reference.
- `cpackget pdsc rm Vendor.PackName`: removes all installed versions of the `Vendor.PackName` pack.

Note that removing packs does not require pack of PDSC file location specification, e.g. no need to provide the path for the PDSC file or the URL of the pack.

## Contributing to cpackget tool

Found a bug? Want a new feature? Or simply want to fix a typo somewhere? If so please refer to our [contributing guide](CONTRIBUTING.md).
