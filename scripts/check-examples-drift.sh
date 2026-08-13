#!/usr/bin/env bash
# Assert that the integration script references exactly the fixture files that exist.
set -euo pipefail

fixtures_dir="${PWD}/examples/_fixtures"
integration_script="${PWD}/scripts/integration-linux.sh"

fail() {
  printf 'drift check failure: %s\n' "$*" >&2
  exit 1
}

# Collect fixture files that exist.
declare -a existing
while IFS= read -r -d '' file; do
  existing+=("$(basename "${file}")")
done < <(find "${fixtures_dir}" -maxdepth 1 -type f -name '*.yaml' -print0)

# Collect fixture filenames referenced by the integration script.
# The script uses ${FIXTURES_DIR}/<name>.yaml, so we grep for the .yaml filenames.
declare -a referenced
while IFS= read -r line; do
  referenced+=("${line}")
done < <(grep -oE '[a-zA-Z0-9_-]+\.yaml' "${integration_script}" | sort -u)

# Compare: every existing fixture must be referenced, and vice versa.
for file in "${existing[@]}"; do
  found=false
  for ref in "${referenced[@]}"; do
    if [[ "${file}" == "${ref}" ]]; then
      found=true
      break
    fi
  done
  if [[ "${found}" == false ]]; then
    fail "fixture ${file} exists but is not referenced by the integration script"
  fi
done

for ref in "${referenced[@]}"; do
  found=false
  for file in "${existing[@]}"; do
    if [[ "${file}" == "${ref}" ]]; then
      found=true
      break
    fi
  done
  if [[ "${found}" == false ]]; then
    fail "integration script references ${ref} but it does not exist in ${fixtures_dir}"
  fi
done

printf 'Examples drift check passed: %d fixture(s) referenced.\n' "${#existing[@]}"
