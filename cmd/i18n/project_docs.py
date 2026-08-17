#!/usr/bin/env python3
"""为 project-docs 生成的 docs.json 补充缺失语言内容。"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import time
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any, Iterator


DEFAULT_SOURCE_LOCALE = "zh-CN"
TRANSLATION_ENDPOINT = "http://translate.googleapis.com/translate_a/single"
LOCALE_PATTERN = re.compile(r"^[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})*$")
ENTRY_PATTERN = re.compile(r"__KRATOS_ENTRY_(\d{4})__")
PROTECTED_PATTERN = re.compile(
    r"(?s)```.*?```|`[^`]+`|\{\{[^{}]+\}\}|\$\{[^{}]+\}|"
    r"\{[A-Za-z_][A-Za-z0-9_.-]*\}|%[sdv]|</?[^>]+>|"
    r"https?://[^\s<>()]+|/(?:api|events|mcp|v[0-9]+)/[A-Za-z0-9_./:{}-]+"
)
FENCE_PATTERN = re.compile(r"^\s*(```|~~~)")


def normalize_locale(value: str) -> str:
    """规范化语言代码，用于比较而不改变输出键的大小写。"""
    return value.strip().replace("_", "-").lower()


def parse_locales(value: str) -> list[str]:
    """解析逗号分隔的 BCP 47 语言代码列表。"""
    result: list[str] = []
    seen: set[str] = set()
    for item in value.split(","):
        locale = item.strip()
        if not locale:
            continue
        if not LOCALE_PATTERN.fullmatch(locale):
            raise ValueError(f"目标语言不是有效的 BCP 47 代码: {locale}")
        normalized = normalize_locale(locale)
        if normalized not in seen:
            seen.add(normalized)
            result.append(locale)
    return sorted(result)


def protect_text(value: str) -> tuple[str, dict[str, str]]:
    """保护 Markdown 代码、链接和占位符，避免机器翻译破坏格式。"""
    values: dict[str, str] = {}

    def replace(match: re.Match[str]) -> str:
        token = f"__KRATOS_TOKEN_{len(values):04d}__"
        values[token] = match.group(0)
        return token

    return PROTECTED_PATTERN.sub(replace, value), values


def restore_text(value: str, protected: dict[str, str]) -> str:
    """恢复 Markdown 中被保护的原始片段。"""
    for token, original in protected.items():
        value = value.replace(token, original)
    return value


def google_translate(value: str, source: str, target: str) -> str:
    """调用与 kratos-admin 翻译脚本一致的 Google V1 接口。"""
    query = urllib.parse.urlencode(
        [
            ("client", "gtx"),
            ("sl", source.split("-", 1)[0]),
            ("tl", target.split("-", 1)[0]),
            ("dt", "t"),
            ("q", value),
        ]
    )
    endpoint = os.environ.get("I18N_TRANSLATE_ENDPOINT", TRANSLATION_ENDPOINT)
    request = urllib.request.Request(
        f"{endpoint}{'&' if '?' in endpoint else '?'}{query}",
        headers={"User-Agent": "kratos-admin-i18n/1.0", "Connection": "close"},
    )
    last_error: Exception | None = None
    for attempt in range(3):
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                payload = json.loads(response.read().decode("utf-8"))
            return "".join(item[0] for item in payload[0] if item and item[0])
        except Exception as error:  # noqa: BLE001 - provider failure is retried
            last_error = error
            if attempt < 2:
                time.sleep(0.5 * (attempt + 1))
    raise RuntimeError(f"Google V1 翻译失败: {last_error}")


def load_opencc() -> Any:
    """按需加载繁体中文转换器；未安装时返回 None。"""
    try:
        from opencc import OpenCC
    except ImportError:
        return None
    return OpenCC("s2twp")


def translate_markdown(value: str, source: str, target: str, offline: bool) -> str:
    """翻译 Markdown 自然语言，并保留代码块、链接和占位符。"""
    if offline:
        converter = load_opencc() if normalize_locale(target) == "zh-tw" else None
        return converter.convert(value) if converter is not None else value

    lines = value.splitlines(keepends=True)
    output: list[str] = []
    pending: list[tuple[int, str, dict[str, str]]] = []
    pending_size = 0
    in_fence = False

    def flush() -> None:
        nonlocal pending, pending_size
        if not pending:
            return
        source_text = "\n".join(
            f"__KRATOS_ENTRY_{index:04d}__ {protected}"
            for index, (_, protected, _) in enumerate(pending)
        )
        translated = ""
        try:
            translated = google_translate(source_text, source, target)
        except RuntimeError:
            translated = ""
        translated_by_index: dict[int, str] = {}
        matches = list(ENTRY_PATTERN.finditer(translated))
        for position, match in enumerate(matches):
            end = matches[position + 1].start() if position + 1 < len(matches) else len(translated)
            translated_by_index[int(match.group(1))] = translated[match.end() : end].strip()
        for line_index, original, protected_values in pending:
            translated_line = translated_by_index.get(line_index, "")
            output.append(restore_text(translated_line, protected_values) if translated_line else original)
        pending = []
        pending_size = 0

    for line in lines:
        body = line.rstrip("\r\n")
        ending = line[len(body) :]
        stripped = body.strip()
        if FENCE_PATTERN.match(stripped):
            flush()
            output.append(line)
            in_fence = not in_fence
            continue
        if in_fence or not any(character.isalpha() or character.isdigit() for character in stripped):
            flush()
            output.append(line)
            continue
        protected, protected_values = protect_text(body)
        if pending and pending_size + len(protected) > 1200:
            flush()
        pending.append((len(pending), protected, protected_values))
        pending_size += len(protected)
        if ending:
            # 换行符在翻译请求外保留，避免服务改变 Markdown 行结构。
            pending[-1] = (pending[-1][0], pending[-1][1] + ending, pending[-1][2])
    flush()
    return "".join(output)


def iter_documents(node: dict[str, Any]) -> Iterator[dict[str, Any]]:
    """递归遍历 docs.json 中的全部文档节点。"""
    yield from node.get("documents", [])
    for directory in node.get("directories", []):
        yield from iter_documents(directory)


def resolve_output(root: Path, value: str) -> Path:
    """将输出目录解析为绝对路径。"""
    path = Path(value)
    return path if path.is_absolute() else root / path


def main() -> int:
    """读取 docs.json，按目标语言补充 locale 并写回文件。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="项目根目录")
    parser.add_argument("--output", required=True, help="project-docs 输出目录")
    parser.add_argument("--source-locale", default=DEFAULT_SOURCE_LOCALE, help="源语言")
    parser.add_argument("--locales", required=True, help="逗号分隔的目标语言列表")
    parser.add_argument("--offline", action="store_true", help="跳过网络翻译")
    args = parser.parse_args()
    try:
        locales = parse_locales(args.locales)
    except ValueError as error:
        parser.error(str(error))
    output_dir = resolve_output(Path(args.root).resolve(), args.output)
    catalog_path = output_dir / "assets" / "docs.json"
    try:
        catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
    except OSError as error:
        print(f"读取项目文档目录失败: {error}", file=sys.stderr)
        return 1
    except json.JSONDecodeError as error:
        print(f"解析项目文档目录失败: {error}", file=sys.stderr)
        return 1

    changed = 0
    for document in iter_documents(catalog):
        content = document.get("content", "")
        if not content:
            continue
        localized = document.setdefault("locale", {})
        if "localized_contents" in document:
            for locale, translated in document.pop("localized_contents").items():
                localized.setdefault(locale, translated)
        existing = {normalize_locale(locale) for locale in localized}
        for locale in locales:
            if normalize_locale(locale) == normalize_locale(args.source_locale) or normalize_locale(locale) in existing:
                continue
            localized[locale] = translate_markdown(content, args.source_locale, locale, args.offline)
            existing.add(normalize_locale(locale))
            changed += 1

    catalog_path.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"已补充 {changed} 条项目文档语言内容到 {catalog_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
