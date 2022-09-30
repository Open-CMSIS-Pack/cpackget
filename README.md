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
  add              Add Open-CMSIS-Pack packages
  checksum-create  Generates a .checksum file containing the digests of a pack
  checksum-verify  Verifies the integrity of a pack using its .checksum file
  help             Help about any command
  init             Initializes a pack root folder
  list             List installed packs
  rm               Remove Open-CMSIS-Pack packages
  signature-create Create a digest list of a pack and signs it
  signature-verify Verifies the integrity of a .checksum against its signature
  update-index     Update the public index

Flags:
  -C, --concurrent-downloads uint   Number of concurrent batch downloads. Set to 0 to disable concurrency (default 5)
  -h, --help                        help for cpackget
  -R, --pack-root string            Specifies pack root folder. Defaults to CMSIS_PACK_ROOT environment variable
  -q, --quiet                       Run cpackget silently, printing only error messages
  -T, --timeout uint                Set maximum duration (in seconds) of a download. Disabled by default
  -v, --verbose                     Sets verboseness level: None (Errors + Info + Warnings), -v (all + Debugging). Specify "-q" for no messages
  -V, --version                     Prints the version number of cpackget and exit

Use "cpackget [command] --help" for more information about a command.
```

For example, if one wanted help removing a pack, running `cpackget rm --help` would print out useful information on the subject.

### Specifying the working pack root folder

If cpackget is going to work on an existing pack root folder, there are two ways to specify it:

1. `export CMSIS_PACK_ROOT=path/to/pack-root; cpackget add ARM.CMSIS`
2. `cpackget --pack-root path/to/pack-root pack add ARM.CMSIS`

To create a new pack root folder with an up-to-date index file of publicly available Open-CMSIS-Pack packs run:

```
$ cpackget init --pack-root path/to/new/pack-root https://www.keil.com/pack/index.pidx
```

The command will create a folder called `path/to/new/pack-root` and the following subfolders: `.Download`, `.Local`, `.Web`.
A copy of the index file (if specified) is placed in `.Web/index.pidx`.

If later it is needed to update the public index file, just run `cpackget index https://vendor.com/index.pidx` and
`.Web/index.pidx` will be updated accordingly.

**As of v0.7.0, the pack root is read-only, with permissions being handled by cpackget.** Changing any permissions manually inside the pack root might cause erratic behavior, potentially breaking functionality.

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

The extracted license file will be placed next to the pack's. For example if Vendor.PackName.x.y.z had a license file
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

Remove a local pack, or remove all instances of a local pack that were added via different PDSC file locations
* `cpackget rm Vendor.PackName.pdsc`

Remove a specific PDSC of a pack
* `cpackget rm path/to/Vendor.PackName.pdsc` (`cpackget list` displays the absolute path of PDSC installed packs)

### Updating the index

It is common that the index.pidx file gets outdated sometime after the pack installation is initialized.
A good practice is to keep it always updated. One can do that by running
* `cpackget update-index`

This will use the address from the `<url>` tag inside index.pidx to retrieve a new version of the file.
cpackget will also go through all PDSC files within `.Web/` checking if the latest version has been
oudated by the one matching the pack tag in index.pidx.

If wanted, the behavior above can be disabled by using `--sparse` flag, thus updating only the index.pidx.

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

### Specifying timeouts

It's possible to set timeouts on commands that perform HTTP downloads, like `cpackget add`. \
By default there's no timeout. Use the `-T/--timeout` global flag to specify in seconds the maximum waiting period for an HTTP GET:

```bash
$ cpackget add Vendor::PackName --timeout 5 # Maximum timeout of 5 seconds
```

**Note**: This feature will be reworked as not to set a hard timeout but an "exponential backoff" based on a number of retries. Some connections might take a lot longer than others, so if an operation like installing a public pack fails, increase the timeout or do not use it at all.

### Parallel downloads

By default  commands that mass download, like `update-index`, use 5 parallel connections to speed up the process. Use the `-C/--concurrent-downloads` global flag to specify the maximum number of parallel HTTP connections that can be opened:

```bash
$ cpackget add Vendor::PackName --concurrent-downloads 7 # Maximum 7 parallel downloads
```

Setting it to 0 will disable any parallel downloads.

**Note**: Some hosts might have firewalls/attack mitigation software that might identify multiple fast connections being opened as an attack. If downloading from a certain domain keeps failing, disable both concurrent downloads (set it to 0) and maximum timeout (by not using the flag).

## Security features

The following features are a WIP and under constant review/discussion. These might suddenly change from release to release, with potential breaking changes. Always check the release/changelog first.

### Integrity checking
As of release **v0.7.0**, it's possible to create a `.checksum` file of a local .pack. This file resembles a common digest file, used to confirm that an obtained piece of information matches the source's content. \
Instead of just including the digest of the entire .pack as one, it lists the digests of all the files.

The extension includes the pack's name in its canonical form and hash algorithm used, appended by ".checksum": `Pack.Vendor.1.0.0.sha256.checksum`. Currently only works for local packs.

To create it:

```bash
$ cpackget checksum-create Vendor.PackName.1.0.0
```

To verify a .pack against it's checksum file:

```bash
$ cpackget checksum-verify Vendor.PackName.1.0.0
```

(the .checksum path is assumed to be the same as the .pack, but it can be specified with the `-p` flag)

### Signed Packs

Likewise, this capability can also be extended to check for _authenticity_. With the usage of x509 certificates and/or PGP signatures, it's possible to create a chain of trust where pack vendors can provide verifiable proof that mitigates malicious attacks (like man-in-the-middle) and assure cpackget users
the legitimacy and authenticity of their published packs.

For more info on the current proof-of-concept implementation: `cpackget help signature-create` and `cpackget help signature-verify`.

## Contributing to cpackget tool

Found a bug? Want a new feature? Or simply want to fix a typo somewhere? If so please refer to our [contributing guide](CONTRIBUTING.md).
