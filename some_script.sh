readonly BASE="$(git merge-base origin/main "${HEAD}")"
for path in $(git diff --name-only "${BASE}" "${HEAD}" | grep -E '^\W*feature' | sort -u); do
  readme_file="${dir}/README.md"
  if [[ "$file" == *README.md ]]; then
    echo "$file"
  fi
done
