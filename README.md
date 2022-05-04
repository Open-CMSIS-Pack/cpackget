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
  index       Update public index
  init        Initialize a pack root folder
  add         Add Open-CMSIS-Pack packages
  rm          Remove Open-CMSIS-Pack packages
  list        List installed packs

Flags:
  -h, --help               help for cpackget
  -R, --pack-root string   Specifies pack root folder. Defaults to CMSIS_PACK_ROOT environment variable
  -q, --quiet              Run cpackget silently, printing only error messages
  -v, --verbose            Sets verboseness level: None (Errors + Info + Warnings), -v (all + Debugging). Specify "-q" for no messages
  -V, --version            Prints the version number of cpackget and exit

Use "cpackget [command] --help" for more information about a command.
```

For example, if one wanted help removing a pack, running `cpackget rm --help` would print out useful information on the subject.


### Sepecifying the working pack root folder

If cpackget is going to work on an existing pack root folder, there are two ways to specify it:

1. `export CMSIS_PACK_ROOT=path/to/pack-root; cpackget add ARM.CMSIS`
2. `cpackget --pack-root path/to/pack-root pack add ARM.CMSIS`

To create a new pack root folder with an up-to-date index file of publicly available Open-CMSIS-Pack packs run:

```
$ cpackget init --pack-root path/to/new/pack-root https://keil.com/pack/index.pidx
```

The command will create a folder called `path/to/new/pack-root` and the following subfolders: `.Download`, `.Local`, `.Web`.
A copy of the index file (if specified) is placed in `.Web/index.pidx`.

If later it is needed to update the public index file, just run `cpackget index https://vendor.com/index.pidx` and
`.Web/index.pidx` will be updated accordingly.


### Adding packs

The commands below demonstrate how to add packs:

Install a pack version that is present in the file system already:
* `cpackget add path/to/Vendor.PackName.x.y.z.pack`

Install a pack version that can be downloaded using a web link:
* `cpackget add https://vendor.com/example/Vendor.PackName.x.y.z.pack`

Install a pack version from the public package index. The download url will be looked up by the tool:
* `cpackget add Vendor.PackName.x.y.z` or `cpackget pack add Vendor::PackName@x.y.z`

Install the latest published version of a public package listed in the package index:
* `cpackget add Vendor.PackName` or `cpackget pack add Vendor::PackName`

Install packs using version modifiers:
* `cpackget add Vendor::PackName>=x.y.z`, check if there is any version greater than or equal to x.y.z, install latest otherwise
* `cpackget add Vendor::PackName@~x.y.z`, check if there is any version greater than or equal to x.y.z 

Install the pack versions specified in the ascii file. Each line specifies a single pack.
* `cpackget add -f list-of-packs.txt`

The command below is an example how to add packs via PDSC files:
* `cpackget add path/to/Vendor.PackName.pdsc`

Note that for adding packs via PDSC files is not possible to provide an URL as input. Only local files are allowed.

### Listing installed packs

One could get a list of all installed packs by running the list command:
* `cpackget list`

This will include all packs that got installed via `cpackget add` command, including packs
that were added via PDSC file.

There are also a couple of flags that allow listing extra information.

List all cached packs, that are present in the ".Download/" folder:
* `cpackget list --cached`

List all packs present in index.pidx:
* `cpackget list --public`

### Accepting the End User License Agreement (EULA) from the command line

Some packs come with licenses and by default cpackget will prompt the user for agreement. This can be avoided
by using the `--agree-embedded-license` flag:

* `cpackget add --agree-embedded-license Vendor.PackName`

Also there are cases where users might want to only extract the pack's license and not install it:

* `cpackget add --extract-embedded-license Vendor.PackName`

The extracted license file will be placed next to the pack's. For example if Vendor.PackName.x.y.z had a licese file
named `LICENSE.txt`, cpackget would extract it to `.Download/Vendor.PackName.x.y.z.LICENSE.txt`.

### Removing packs

The commands below demonstrate how to remove packs.

Remove pack `Vendor.PackName` version `x.y.z` only, leave others untouched
* `cpackget rm Vendor.PackName.x.y.z` or `cpackget rm Vendor::PackName@x.y.z`

Remove all versions of pack `Vendor.PackName`
* `cpackget rm Vendor.PackName` or `cpackget rm Vendor::PackName`

Same as above, except that now it also removes the cached pack file.
* `cpackget rm --purge Vendor.PackName`: using `--purge` triggers removal of any downloaded files.

And for removing packs that were installed via PDSC files, consider the example commands below:

Remove pack `Vendor.PackName` version `x.y.z` only, from the local packs.
* `cpackget rm Vendor.PackName.x.y.z` or `cpackget rm Vendor::PackName@x.y.z`

Remove all versions of pack `Vendor.PackName`, from the local packs.
* `cpackget rm Vendor.PackName` or `cpackget rm Vendor::PackName`

Note that removing packs does not require pack of PDSC file location specification, e.g. no need to provide the path for the PDSC file or the URL of the pack.

### Working behind a proxy

Some use cases might require network access via a proxy. This can be done via environment variables that are used
by `cpackget`:

```bash
# Windows
% set HTTP_PROXY=http://my-proxy         # proxy used for HTTP requests
% set HTTPS_PROXY=https://my-https-proxy # proxy used for HTTPS requests

# Unix
$ export HTTP_PROXY=http://my-proxy         # proxy used for HTTP requests
$ export HTTPS_PROXY=https://my-https-proxy # proxy used for HTTPS requests
```

Then **all** HTTP/HTTPS requests will be going through the specified proxy.

## Contributing to cpackget tool

Found a bug? Want a new feature? Or simply want to fix a typo somewhere? If so please refer to our [contributing guide](CONTRIBUTING.md).
