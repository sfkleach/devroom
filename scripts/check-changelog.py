#!/usr/bin/env python3
"""Check that CHANGELOG.md is ready to release.

Usage: check-changelog.py [tag]

If <tag> is given, checks that CHANGELOG.md's top ## heading contains its
version. If omitted, the version is instead taken from the top ## heading
itself — i.e. the top entry is assumed to be the one about to be released.

Exits with a non-zero status if:
- The top ## section heading contains the word 'unreleased', or
- A <tag> was given and the top heading does not contain its version, or
- No <tag> was given and the top heading has no version number to infer
  one from, or
- Any version number (X.Y.Z) appears in more than one ## heading.
"""
import re
import sys


def main() -> None:
    if len(sys.argv) not in (1, 2):
        print(f"Usage: {sys.argv[0]} [tag]", file=sys.stderr)
        sys.exit(1)

    tag = sys.argv[1] if len(sys.argv) == 2 else None

    with open("CHANGELOG.md") as f:
        content = f.read()

    # Split on level-2 headings.
    parts = re.split(r"\n(?=## )", content)
    headings = []
    for part in parts:
        if not part.startswith("## "):
            continue
        heading_end = part.index("\n") if "\n" in part else len(part)
        headings.append(part[:heading_end])

    if not headings:
        print("FAIL: No ## heading found in CHANGELOG.md.")
        sys.exit(1)

    # Extract version strings (X.Y.Z) from all headings and check for duplicates.
    seen: dict[str, str] = {}
    for heading in headings:
        m = re.search(r"\d+\.\d+\.\d+", heading)
        if not m:
            continue
        ver = m.group(0)
        if ver in seen:
            print(
                f"FAIL: Version '{ver}' appears more than once in CHANGELOG.md "
                f"('{seen[ver]}' and '{heading}')."
            )
            sys.exit(1)
        seen[ver] = heading

    heading = headings[0]
    if "unreleased" in heading.lower():
        print(f"FAIL: Top CHANGELOG section is '{heading}' — release is not ready.")
        sys.exit(1)

    if tag is not None:
        version = tag.lstrip("v")
        if version not in heading:
            print(
                f"FAIL: Top CHANGELOG section '{heading}' does not contain "
                f"version '{version}' for tag '{tag}'."
            )
            sys.exit(1)
        print(f"OK: Top CHANGELOG section is '{heading}' (matches tag {tag}).")
    else:
        m = re.search(r"\d+\.\d+\.\d+", heading)
        if not m:
            print(
                f"FAIL: Top CHANGELOG section '{heading}' has no version "
                f"number to infer a release tag from."
            )
            sys.exit(1)
        print(f"OK: Top CHANGELOG section is '{heading}' (inferred version {m.group(0)}).")

    sys.exit(0)


if __name__ == "__main__":
    main()
