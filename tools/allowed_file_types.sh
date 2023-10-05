#!/bin/bash

exitcode=0

echo "Checking for allowed file types..." >&2

if
  find . -name .git -prune -o -name openconfig_public -prune -o \
    -type f -exec file \{} \+ |
  egrep -vi '(ASCII|UTF-8|JSON|Perl|shell|PEM)'
then
  echo "Error: files should be in plain text or non-empty." >&2
  exitcode=1
fi

echo "Checking for regular files..." >&2

if
  find . -name .git -prune -o \
    -name '*.sh' -o -name '*.pl' -o -name '*.py' -o \
    -type f -executable -print | grep .
then
  echo "Error: regular files should not have the executable bit." >&2
  exitcode=1
fi

echo "Checking for script files..." >&2

if
  find . -name .git -prune -o -name openconfig_public -prune -o \
    '(' -name '*.sh' -o -name '*.pl' -o -name '*.py' ')' \
    -type f '!' -executable -print | grep .
then
  echo "Error: script files should have the executable bit." >&2
  exitcode=1
fi

if ((exitcode)); then
  echo 'Please see "Allowed File Types" in CONTRIBUTING.md for detail.' >&2
  exit "${exitcode}"
fi
