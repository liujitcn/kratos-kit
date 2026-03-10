#!/usr/bin/env python3
"""根据远程更新状态自动创建并推送 tag。"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

TAG_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")


def run(cmd: list[str], check: bool = True) -> str:
    result = subprocess.run(cmd, capture_output=True, text=True)
    if check and result.returncode != 0:
        raise RuntimeError(f"命令执行失败: {' '.join(cmd)}\n{result.stderr.strip()}")
    return result.stdout.strip()


def run_code(cmd: list[str]) -> int:
    result = subprocess.run(cmd, capture_output=True, text=True)
    return result.returncode


def detect_remote_ref() -> str:
    remote_head = run(["git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"], check=False)
    if remote_head.startswith("origin/"):
        branch = remote_head.split("/", 1)[1]
    elif remote_head:
        branch = remote_head
    else:
        # 兼容未设置 origin/HEAD 的仓库。
        branch = "main"
    remote_ref = f"origin/{branch}"
    if run_code(["git", "rev-parse", "--verify", remote_ref]) != 0:
        # 兜底尝试 master。
        remote_ref = "origin/master"
    if run_code(["git", "rev-parse", "--verify", remote_ref]) != 0:
        raise RuntimeError(f"远程分支不存在: {remote_ref}")
    return remote_ref


def module_dirs() -> list[str]:
    paths = sorted(str(p.parent) for p in Path(".").rglob("go.mod") if ".git" not in p.parts)
    return ["." if p == "." else p for p in paths]


def prefix_of(module_dir: str) -> str:
    if module_dir == ".":
        return ""
    return f"{module_dir.lstrip('./')}/"


def parse_semver(tag: str) -> tuple[int, int, int]:
    match = TAG_RE.match(tag)
    if not match:
        raise ValueError(f"非法语义化版本: {tag}")
    return int(match.group(1)), int(match.group(2)), int(match.group(3))


def latest_tag(prefix: str) -> str | None:
    tags = run(["git", "tag", "-l", f"{prefix}v[0-9]*.[0-9]*.[0-9]*"]).splitlines()
    parsed: list[tuple[tuple[int, int, int], str]] = []
    for full in tags:
        if not full:
            continue
        raw = full[len(prefix) :] if prefix else full
        if not TAG_RE.match(raw):
            continue
        parsed.append((parse_semver(raw), full))
    if not parsed:
        return None
    parsed.sort(key=lambda x: x[0], reverse=True)
    return parsed[0][1]


def next_tag(latest: str | None, prefix: str) -> str:
    if not latest:
        return f"{prefix}v0.0.1"
    raw = latest[len(prefix) :] if prefix else latest
    major, minor, patch = parse_semver(raw)
    return f"{prefix}v{major}.{minor}.{patch + 1}"


def has_remote_update(latest: str | None, remote_ref: str, module_dir: str) -> bool:
    if latest:
        count = run(["git", "rev-list", "--count", f"{latest}..{remote_ref}", "--", module_dir])
    else:
        count = run(["git", "rev-list", "--count", remote_ref, "--", module_dir])
    return int(count or "0") > 0


def remote_tag_exists(tag: str) -> bool:
    return run_code(["git", "ls-remote", "--exit-code", "--tags", "origin", f"refs/tags/{tag}"]) == 0


def local_tag_exists(tag: str) -> bool:
    return run_code(["git", "rev-parse", "--verify", tag]) == 0


def ensure_local_tag(tag: str, remote_ref: str) -> None:
    if not local_tag_exists(tag):
        run(["git", "tag", tag, remote_ref])


def push_tag(tag: str) -> None:
    run(["git", "push", "origin", tag])


def process_module(module_dir: str, remote_ref: str) -> bool:
    prefix = prefix_of(module_dir)
    latest = latest_tag(prefix)
    nxt = next_tag(latest, prefix)

    if not has_remote_update(latest, remote_ref, module_dir):
        print(f"{module_dir} => 最新 tag: {latest or '<无>'}，远程无更新，跳过。")
        return False

    if remote_tag_exists(nxt):
        print(f"{module_dir} => 远程 tag {nxt} 已存在，跳过。")
        return False

    ensure_local_tag(nxt, remote_ref)
    push_tag(nxt)
    print(f"{module_dir} => 已推送远程 tag: {nxt} -> {remote_ref}")
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description="远程更新触发的 tag 发布脚本")
    parser.add_argument("mode", choices=["tag", "sub-tag"], help="tag=根模块，sub-tag=全部模块")
    args = parser.parse_args()

    try:
        run(["git", "fetch", "origin", "--tags"])
        remote_ref = detect_remote_ref()
        print(f"远程分支引用: {remote_ref}")

        if args.mode == "tag":
            ok = process_module(".", remote_ref)
            if not ok:
                print("根模块远程无更新，未推送任何 tag。")
            return 0

        pushed = False
        for d in module_dirs():
            if process_module(d, remote_ref):
                pushed = True
        if not pushed:
            print("所有模块远程均无更新，未推送任何 tag。")
        return 0
    except RuntimeError as err:
        print(str(err), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
