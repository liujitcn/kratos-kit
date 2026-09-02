#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "==> 清理前端全部 node_modules"
find "${FRONTEND_DIR}" -type d -name node_modules -prune -print -exec rm -rf -- {} +

while IFS= read -r workspace_file; do
  workspace_dir="${workspace_file%/pnpm-workspace.yaml}"

  # 跳过 CLI 模板等嵌套 workspace，避免把占位项目当作当前项目安装。
  nested_workspace=false
  parent_dir="${workspace_dir}"
  while [[ "${parent_dir}" != "${FRONTEND_DIR}" ]]; do
    parent_dir="${parent_dir%/*}"
    if [[ -f "${parent_dir}/pnpm-workspace.yaml" ]]; then
      nested_workspace=true
      break
    fi
  done

  if [[ "${nested_workspace}" == true ]]; then
    echo "==> 跳过嵌套 workspace：${workspace_dir#"${FRONTEND_DIR}/"}"
    continue
  fi

  echo "==> 安装 ${workspace_dir#"${FRONTEND_DIR}/"} 依赖"
  (
    cd "${workspace_dir}"
    CI=1 pnpm install --frozen-lockfile
  )
done < <(find "${FRONTEND_DIR}" -type f -name pnpm-workspace.yaml -print)

echo "==> 前端依赖重装完成"
