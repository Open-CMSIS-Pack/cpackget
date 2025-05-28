# -------------------------------------------------------
# SPDX-License-Identifier: Apache-2.0
# Copyright Contributors to the cpackget project.
# -------------------------------------------------------

"""
Checks the presence of copyright notice in the files
"""

from typing import Optional, Sequence
import argparse
import os
import sys
import magic
from comment_parser import comment_parser

COPYRIGHT_TEXT = "Copyright Contributors to the cpackget project."
LICENSE_TEXT = "SPDX-License-Identifier: Apache-2.0"

def check_file(filename: str) -> int:
    """
    Checks a file for the presence of fixed copyright and license notices.
    Args:
        filename: The name of the file to check.
    Returns:
        0 if both copyright and license are found, 1 otherwise.
    """
    if os.path.getsize(filename) == 0:
        return 0

    try:
        mime_type = magic.from_file(filename, mime=True)
    except Exception as e:
        print(f"# Error reading MIME type of {filename}: {e}")
        return 1

    if mime_type == "text/plain":
        mime_type = "text/x-c++"

    try:
        comments = "\n".join(comment.text() for comment in comment_parser.extract_comments(filename, mime=mime_type))
    except Exception as e:
        print(f"# Failed to parse comments in {filename}: {e}")
        return 1

    copyright_found = COPYRIGHT_TEXT in comments
    license_found = LICENSE_TEXT in comments

    if copyright_found and license_found:
        return 0

    print(f"# Copyright check error(s) in: {filename}")
    if not copyright_found:
        print(f"\t# Missing or invalid copyright. Expected: {COPYRIGHT_TEXT}")
    if not license_found:
        print(f"\t# Missing or invalid license. Expected: {LICENSE_TEXT}")
    return 1

def main(argv: Optional[Sequence[str]] = None) -> int:
    """
    Entry point to check for copyright notices in the provided files.
    Args:
        argv: A list of filenames.
    Returns:
        Non-zero if any file is missing the required notice.
    """
    parser = argparse.ArgumentParser(description="Check for fixed copyright and license headers.")
    parser.add_argument('filenames', nargs='*', help='Files to check.')
    args = parser.parse_args(argv)

    print("Checking copyright headers...")
    ret = 0

    for filename in args.filenames:
        ret |= check_file(filename)

    if ret != 0:
        print(">> error: One or more files are missing a valid copyright or license header")

    return ret

if __name__ == '__main__':
    sys.exit(main())
