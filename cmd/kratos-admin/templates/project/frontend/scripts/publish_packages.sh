#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

registry="${NPM_REGISTRY:-https://registry.npmjs.org/}"
access="${NPM_ACCESS:-public}"
tag="${NPM_TAG:-latest}"
skip_git_checks="${NPM_SKIP_GIT_CHECKS:-true}"
expected_version="${NPM_EXPECTED_VERSION:-}"

publish_args=(--registry "${registry}" --access "${access}" --tag "${tag}")
if [[ "${skip_git_checks}" == "true" ]]; then
  publish_args+=(--no-git-checks)
fi

if [[ "$#" -eq 0 ]]; then
  set -- \
    admin/packages/core \
    admin/packages/modules/system \
    admin/packages/cli \
    uni-app/packages/core \
    uni-app/packages/modules/system \
    uni-app/packages/cli \
    taro-app/packages/core \
    taro-app/packages/ui \
    taro-app/packages/modules/system \
    taro-app/packages/cli
fi

for package_dir in "$@"; do
  package_path="${FRONTEND_DIR}/${package_dir}/package.json"
  if [[ ! -f "${package_path}" ]]; then
    echo "npm 包配置不存在: ${package_path}" >&2
    exit 1
  fi

  metadata="$({ node -e '
const fs = require("fs")
const pkg = JSON.parse(fs.readFileSync(process.argv[1], "utf8"))
if (typeof pkg.name !== "string" || typeof pkg.version !== "string") {
  throw new Error(`invalid package metadata: ${process.argv[1]}`)
}
process.stdout.write(`${pkg.name}\t${pkg.version}`)
' "${package_path}"; })"
  IFS=$'\t' read -r package_name package_version <<<"${metadata}"

  if [[ -n "${expected_version}" && "${package_version}" != "${expected_version}" ]]; then
    echo "npm 包版本与发布 tag 不一致: ${package_name}@${package_version}，期望 ${expected_version}" >&2
    exit 1
  fi

  if npm view "${package_name}@${package_version}" version --registry "${registry}" --json >/dev/null 2>&1; then
    echo "==> 已发布，跳过 ${package_name}@${package_version}"
    continue
  fi

  echo "==> 正在发布 ${package_name}@${package_version}"
  (
    cd "${FRONTEND_DIR}/${package_dir}"
    pnpm publish "${publish_args[@]}"
  )
  echo "==> 发布完成 ${package_name}@${package_version}"
done
