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
