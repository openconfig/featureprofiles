#!/bin/bash
#
# Outputs a list of tests and their feature paths in CSV format.

if [[ -t 2 ]]; then  # stderr is terminal.
  red() { echo $'\e[1;31m'"$@"$'\e[0m'; }  # use ANSI colors.
else
  red() { echo "$@"; }  # no color.
fi

list_tests() {
  echo '"Feature","ID","Desc","Test Path"'

  local line
  while read line; do
    local path id desc
    IFS=':' read path id desc <<<"${line}"
    path="${path%/*.md}"    # Strip README.md from path.
    id="${id#\# }"          # Strip '# ' from ID.
    desc="$(echo $desc)"    # Strip excess whitespaces.

    local feature
    case "${path}" in
      */ate_tests/*) feature="${path%%/ate_tests/*}";;
      */otg_tests/*) feature="${path%%/otg_tests/*}";;
      */tests/*) feature="${path%%/tests/*}";;
      */kne_tests/*) feature="${path%%/kne_tests/*}";;
      *)
        echo "$(red WARNING:)" "not a valid test path: $line" >&2
        feature="${path}"
        ;;
    esac
    feature="${feature#testing/}"
    feature="${feature#feature/}"

    echo "\"${feature}\",\"${id}\",\"${desc}\",\"${path}\""
  done < <(
    find -L feature -name \*.md -exec egrep '^# [A-Za-z0-9]+-[0-9]+\.[0-9]+:' \{} \+
  )
}

main() {
  cd "${0%/*}"  # Directory containing this script, which is tools.
  cd ..         # Parent of tools, which is the repo.
  list_tests
}

main "$@"
