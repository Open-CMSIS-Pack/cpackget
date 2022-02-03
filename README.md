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
Usage:
  cpackget [command] [flags]

Available Commands:
  help        Help about any command
  index       Updates public index
  init        Initializes a pack root folder
  pack        Adds/Removes Open-CMSIS-Pack packages
  pdsc        Adds or removes Open-CMSIS-Pack packages in the local file system via PDSC files.

Flags:
  -h, --help               help for cpackget
  -R, --pack-root string   Specifies pack root folder. Defaults to CMSIS_PACK_ROOT environment variable
  -q, --quiet              Run cpackget silently, printing only error messages
  -v, --verbose            Sets verboseness level: None (Errors + Info + Warnings), -v (all + Debugging). Specify "-q" for no messages
  -V, --version            Prints the version number of cpackget and exit

Use "cpackget [command] --help" for more information about a command.
```

For example, if one wanted help removing a pack, running `cpackget pack rm --help` would print out useful information on the subject.


### Sepecifying the working pack root folder

If cpackget is going to work on an existing pack root folder, there are two ways to specify it:

1. `export CMSIS_PACK_ROOT=path/to/pack-root; cpackget pack add ARM.CMSIS`
2. `cpackget --pack-root path/to/pack-root pack add ARM.CMSIS`

The first example is more common because exporting the environment variables can happen only once.

But cpackget is also capable of creating a new pack root folder if needed. For example:

```
$ cpackget init --pack-root path/to/new/pack-root https://keil.com/pack/index.pidx
```

The command will create a folder called `path/to/new/pack-root`, a few subfolders (`.Download`, `.Local`, `.Web`)
and will place a copy of the index file (if specified) to `.Web/index.pidx`.

If later it is needed to update the public index file, just run `cpackget index https://vendor.com/index.pidx` and
`.Web/index.pidx` will be updated accordingly.


### Adding packs

The commands below are examples on how to add packs:

* `cpackget pack add path/to/Vendor.PackName.x.y.z.pack`
* `cpackget pack add https://vendor.com/example/Vendor.PackName.x.y.z.pack`
* `cpackget pack add Vendor.PackName.x.y.z`
* `cpackget pack add Vendor.PackName`
* `cpackget pack add -f list-of-packs.txt`, the conten is simply a list of packs, one per line

The command below is an example how to add packs via PDSC files:
* `cpackget pdsc add path/to/Vendor.PackName.pdsc`

Note that for adding packs via PDSC files is not possible to provide an URL as input. Only local files are allowed.

### Bypassing End User License Agreement (EULA)

Some packs come with licenses and by default cpackget will prompt the user for agreement. This can be avoided
by using the `--agree-embedded-license` flag:

* `cpackget pack add --agree-embedded-license Vendor.PackName`

Also there are cases where users might want to only extract the pack's license and not install it:

* `cpackget pack add --extract-embedded-license Vendor.PackName`

The extracted license file will be placed next to the pack's. For example if Vendor.PackName.x.y.z had a licese file
named `LICENSE.txt`, cpackget would extract it to `.Download/Vendor.PackName.x.y.z.LICENSE.txt`.

### Removing packs

Removing packs uses slightly different pack specification. Here are a few examples:

* `cpackget pack rm Vendor.PackName.x.y.z`: removes only the *x.y.z* version of the `Vendor.PackName` pack.
* `cpackget pack rm Vendor.PackName`: removes all installed versions of the `Vendor.PackName` pack.
* `cpackget pack rm --purge Vendor.PackName`: using `--purge` triggers removal of any downloaded files.

And for removing packs that were installed via PDSC files, consider the example commands below:
* `cpackget pdsc rm Vendor.PackName.x.y.z`: removes only the *x.y.z* version of the `Vendor.PackName` pack in the local reference.
* `cpackget pdsc rm Vendor.PackName`: removes all installed versions of the `Vendor.PackName` pack.

Note that removing packs does not require pack of PDSC file location specification, e.g. no need to provide the path for the PDSC file or the URL of the pack.

## Contributing to cpackget tool

Found a bug? Want a new feature? Or simply want to fix a typo somewhere? If so please refer to our [contributing guide](CONTRIBUTING.md).
