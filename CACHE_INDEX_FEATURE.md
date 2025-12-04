# Cache Index Feature - Documentation for Testing

## Overview

As of the recent update, cpackget has introduced a new **cache index file** (`cache.pidx`) to improve reliability and performance when managing PDSC files. This document describes the changes and how users can test the new system.

## Background

Previously, cpackget relied on the `index.pidx` file in the `.Web` directory to retrieve information about installed PDSC files. This could lead to inconsistencies.

## What Changed

### New Cache File: `cache.pidx`

The major improvement is the introduction of a **cache index file** located at:

```
<CMSIS_PACK_ROOT>/.Web/cache.pidx
```

This cache file maintains an accurate record of all PDSC files that are currently cached in the `.Web` directory.

### Key Benefits

1. **Improved Reliability**: The cache index accurately reflects what is actually present in the `.Web` directory, eliminating discrepancies between the public index and local cache.

2. **Better Performance**: Operations that check for cached PDSC files no longer need to scan the entire `.Web` directory or rely on potentially outdated information from `index.pidx`.

3. **Automatic Synchronization**: The cache index is automatically updated when:
   - PDSC files are downloaded
   - The public index is updated
   - Packs are installed or removed

### How It Works

#### Cache Initialization

When cpackget starts, it can initialize the cache index by scanning existing PDSC files:

```bash
# The cache is automatically initialized when needed
cpackget update-index
```

The initialization process:
- Scans all `*.pdsc` files in the `.Web` directory
- Extracts pack information (Vendor, Name, Version)
- Reads each PDSC file to get the latest version and URL information
- Builds the `cache.pidx` file with this information

#### Cache Updates

The cache is automatically maintained during:

- **Pack Installation**: When a pack is installed, its PDSC entry is added to the cache
- **Index Updates**: When `update-index` is run, the cache is synchronized with newly downloaded PDSC files
- **PDSC Downloads**: Individual PDSC file downloads update the cache accordingly

### File Format

The `cache.pidx` file uses the same XML format as `index.pidx`:

```xml
<index schemaVersion="1.1.0">
  <vendor>Various</vendor>
  <url></url>
  <pindex>
    <pdsc url="https://www.keil.com/pack/" vendor="ARM" name="CMSIS" version="5.9.0"/>
    <pdsc url="https://www.keil.com/pack/" vendor="Keil" name="STM32F4xx_DFP" version="2.17.1"/>
    <!-- More entries... -->
  </pindex>
</index>
```

## Testing the New System

### 1. Fresh Installation Test

Start with a clean pack root to see the cache being built from scratch:

```bash
# Create a new pack root
cpackget init --pack-root /path/to/test-pack-root https://www.keil.com/pack/index.pidx

# The cache.pidx will be created automatically
# Check if it exists
ls /path/to/test-pack-root/.Web/cache.pidx
```

### 2. Pack Installation Test

Test that the cache is correctly updated when installing packs:

```bash
# Install a pack
cpackget add ARM::CMSIS

# Check the cache contains the PDSC entry
# On Windows:
type %CMSIS_PACK_ROOT%\.Web\cache.pidx | findstr "CMSIS"
# On Linux/Mac:
grep CMSIS $CMSIS_PACK_ROOT/.Web/cache.pidx
```

### 3. Index Update Test

Verify that updating the index properly synchronizes the cache:

```bash
# Update the public index
cpackget update-index

# The cache should now reflect any new PDSC files downloaded
# Compare the number of entries:
# - index.pidx shows all available packs
# - cache.pidx shows only cached PDSC files
```

### 4. Cache Consistency Test

Test the automatic cache initialization for existing installations:

```bash
# If you have an existing pack root without cache.pidx:
# Just run any command that needs the cache

cpackget list --public

# cpackget will automatically initialize the cache from existing PDSC files
```

### 5. Error Recovery Test

Test how the system handles corrupted or missing cache files:

```bash
# Delete the cache file
rm $CMSIS_PACK_ROOT/.Web/cache.pidx

# Run a command - the cache should be automatically rebuilt
cpackget update-index
```

## Expected Behavior Changes

### Before (using `index.pidx`)

- PDSC file information could be out of sync with actual cached files

### After (using `cache.pidx`)

- PDSC cache information is always accurate
- Automatic synchronization reduces manual intervention

## Troubleshooting

### Cache is Empty or Missing

If the `cache.pidx` file is empty or missing:

```bash
# Force cache initialization by updating the index
cpackget update-index
```

### Cache Out of Sync

If you suspect the cache is out of sync:

```bash
# Remove the cache and let it rebuild
rm $CMSIS_PACK_ROOT/.Web/cache.pidx
cpackget update-index
```

### Debugging

Enable verbose logging to see cache operations:

```bash
cpackget -v add ARM::CMSIS
```

This will show:
- Cache reads and writes
- PDSC file downloads
- Cache synchronization operations

## Migration from Previous Versions

If you're upgrading from a version without the cache feature:

1. **Automatic Migration**: Simply run any cpackget command. The cache will be automatically created from your existing `.Web` PDSC files.

2. **Manual Verification**: After migration, compare your installed packs:
   ```bash
   cpackget list
   ```

3. **No Breaking Changes**: All existing commands work the same way. The cache is an internal optimization.

## Technical Details

### File Locations

- Public index: `<CMSIS_PACK_ROOT>/.Web/index.pidx`
- Cache index: `<CMSIS_PACK_ROOT>/.Web/cache.pidx`  (NEW)
- Local index: `<CMSIS_PACK_ROOT>/.Local/local_repository.pidx`

### Constants

```go
const PublicIndexName = "index.pidx"
const PublicCacheIndex = "cache.pidx"  // NEW
```

### API Changes

The changes are internal and backward compatible. No API changes affect end users.

## Reporting Issues

If you encounter any issues with the cache system:

1. Enable verbose logging: `cpackget -v [command]`
2. Check if `cache.pidx` exists and is properly formatted
3. Try rebuilding the cache by removing it and running `update-index`
4. Report issues with:
   - Your OS and cpackget version
   - The command that failed
   - The verbose log output
   - The state of your `.Web` directory

## Summary

The introduction of `cache.pidx` significantly improves the performance of cpackget by maintaining an accurate record of cached PDSC files. The feature is transparent to users and requires no changes to existing workflows, while providing automatic error recovery and synchronization.

For most users, the transition will be seamless - the cache will be automatically created and maintained. Power users can verify cache operations using verbose logging and can manually trigger cache rebuilds when needed.
