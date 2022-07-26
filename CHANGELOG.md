# v0.7.0

This v0.7 release contains:

Bug fixes:
- Not accepting a license is not considered an error

New features:
- Two new commands, `checksum-create` and `checksum-verify`. They are part of a new "cryptography" module, intended to provide advanced security measures for pack installation.
- "Pack root" is now read-only, with the exception of `local_repository.idx`. This measure prevents accidental environment corruption.

# v0.6.0

This release is more of a "symbolic" one, as the last one should've been a minor version bump. This release contains:

Bug fixes
- Update documentation on removing packs via PDSC file

New features
- Allow pack versions with leading zeros

# v0.5.1

This v0.5.1 release contains:

Bug fixes
- Install only minimum and major versions if available
- Fix intermittent testing on Windows

New features
- Filter packs when listing with --filter
- New update-index command

# v0.5.0

This v0.5 release contains:

Bug fixes
- Fix pack installation with semantic versioning

New features
- --force-reinstall flag to force pack installation
- Combine pack and psdc commands
- Make index-url mandatory 
- List commands using Yaml naming standard

# v0.4.1

This v0.4.1 release contains a small bug fix that
prevents cpackget from raising an error when installing
a pack via PDSC file that is already installed

# v0.4.0

This v0.4 release contains:

Bug fixes
- Continue listing packs despite malformed pack names
- Do not raise error when installing a pack already installed
- Do not raise error when using non HTTPS url for updating index.pix

New features
- Avoid displaying progress bar on non-interactive terminals
- Remove extracted licenses when purging
- Add notes on how to configure cpackget behind a proxy
- Support YML pack notation e.g. "ARM::CMSIS@5.7.0"

# v0.3.1

This v0.3.1 release contains a tiny typo fix that prevented
cpackget's version from being injected to its binary, thus
causing it not to display the version of cpackget.

# v0.3.0

This v0.3.0 release makes cpackget more verbose by default. This will
show pack installation progress on both downloading and decompressing.

It also supports gracefully ending an installation when hitting CTRL+C. 

Finally this release supports installing packs using only the pack name,
e.g. "ARM.CMSIS" or "ARM.CMSIS.5.7.0" in case a specific version is required.

# v0.2.0

This v0.2.0 release makes cpackget capable of replacing cp_init and cp_install
in cbuild. It:
- adds an "init" subcommand that allows creation of the pack root installation directory (replaces cp_init.sh)
- supports embedded license agreement step before installing packs
- supports updating the public index in .Web/index.pidx
- supports installing multiple packs at once specified via file (cp_install.sh)

# v0.1.3

This v0.1.3 release fixes a bug when writing the "local_repository.pidx"
without a proper XML header. Also it correctly prefixes local URL of
packs installed via PDSC file.

# v0.1.2

This v0.1.2 release of cpackget fixes a bug when installing
packs on Windows systems.

# v0.1.1

Initial release of cpackget
