#!/usr/bin/env python3
"""根据 OpenAPI 源文档和国际化资源生成本地化 OpenAPI 文档。"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

from generate_locale_drafts import (
    request_i18n,
    load_opencc,
    protect_text,
    restore_text,
    i18n_batch,
)
from local_openapi_i18n import contains_simplified, local_translate


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_INPUT = ROOT / "backend/internal/openapi/assets/openapi.yaml"
DEFAULT_OUTPUT_DIR = DEFAULT_INPUT.parent
DEFAULT_CONTENT = [
    ROOT / "backend/internal/i18n/assets",
    ROOT / "frontend/admin/packages/core/src/locales",
]
DEFAULT_LOCALES = ("en-US", "zh-TW", "ja-JP")
LOCALE_PATTERN = re.compile(r"^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$")
FIELD_PATTERN = re.compile(
    r"^(?P<prefix>\s*)(?P<name>description|summary|title)(?P<separator>\s*:\s*)(?P<value>.*?)(?P<newline>\r?\n)?$"
)


class LocaleCatalog:
    """保存按语言合并后的消息和直接源文映射。"""

    def __init__(self) -> None:
        self.messages: dict[str, dict[str, str]] = {}
        self.direct: dict[str, dict[str, str]] = {}

    def add(self, locale: str, key: str, value: str) -> None:
        if not locale or not key or not value:
            return
        messages = self.messages.setdefault(locale, {})
        existing = messages.get(key)
        if existing is not None and existing != value:
            raise ValueError(f"国际化消息键重复且内容不同: {locale}/{key}")
        messages[key] = value
        if looks_like_source_text(key):
            self.direct.setdefault(locale, {}).setdefault(key, value)


def looks_like_source_text(value: str) -> bool:
    return any(char.isspace() for char in value) or any(ord(char) > 127 for char in value)


def contains_cjk(value: str) -> bool:
    return any("\u4e00" <= char <= "\u9fff" for char in value)


def is_locale(value: str) -> bool:
    return bool(LOCALE_PATTERN.fullmatch(value))


def resolve_path(value: str) -> Path:
    path = Path(value)
    if path.is_absolute() or path_exists(path):
        return path
    repository_path = ROOT / path
    if path_exists(repository_path):
        return repository_path
    return path


def path_exists(path: Path) -> bool:
    try:
        return path.exists()
    except (OSError, ValueError):
        return False


def path_is_dir(path: Path) -> bool:
    try:
        return path.is_dir()
    except (OSError, ValueError):
        return False


def parse_content_spec(spec: str) -> tuple[str | None, str]:
    if "=" not in spec:
        return None, spec
    locale, value = spec.split("=", 1)
    if is_locale(locale):
        return locale, value
    return None, spec


def read_json_content(value: str) -> tuple[Any, str]:
    path = resolve_path(value)
    if path_exists(path):
        try:
            return json.loads(path.read_text(encoding="utf-8")), str(path)
        except json.JSONDecodeError as error:
            raise ValueError(f"解析国际化内容失败: {path}: {error}") from error
        except OSError as error:
            raise ValueError(f"读取国际化内容失败: {path}: {error}") from error
    try:
        return json.loads(value), "命令行内容"
    except json.JSONDecodeError as error:
        raise ValueError(f"国际化内容不是 JSON 文件或 JSON 字符串: {value}") from error


def message_text(value: Any, locale: str) -> str | None:
    if isinstance(value, str):
        return value
    if not isinstance(value, dict):
        return None
    if locale in value:
        return message_text(value[locale], locale)
    for key in ("other", "message", "value", "text"):
        if key in value:
            return message_text(value[key], locale)
    string_values = [item for item in value.values() if isinstance(item, str)]
    if len(string_values) == 1:
        return string_values[0]
    return None


def add_bundle(catalog: LocaleCatalog, locale: str, payload: Any) -> None:
    if not isinstance(payload, dict):
        raise ValueError(f"语言 {locale} 的国际化内容必须是 JSON 对象")
    for key, value in payload.items():
        if not isinstance(key, str):
            continue
        if isinstance(value, dict):
            locale_values = {name: item for name, item in value.items() if is_locale(name)}
            if locale_values:
                for target_locale, target_value in locale_values.items():
                    text = message_text(target_value, target_locale)
                    if text:
                        catalog.add(target_locale, key, text)
                continue
        text = message_text(value, locale)
        if text:
            catalog.add(locale, key, text)


def add_content(catalog: LocaleCatalog, explicit_locale: str | None, payload: Any) -> None:
    if not isinstance(payload, dict):
        raise ValueError("国际化内容必须是 JSON 对象")
    nested_locales = {
        name: value
        for name, value in payload.items()
        if is_locale(name) and isinstance(value, dict)
    }
    if nested_locales and len(nested_locales) == len(payload):
        for locale, locale_payload in nested_locales.items():
            add_bundle(catalog, locale, locale_payload)
        return
    if explicit_locale is None:
        has_per_locale_values = any(
            isinstance(value, dict) and any(is_locale(name) for name in value)
            for value in payload.values()
        )
        if has_per_locale_values:
            add_bundle(catalog, "", payload)
            return
        raise ValueError("单个国际化 JSON 无法推断语言，请使用 语言=文件或 语言=JSON 形式传入")
    add_bundle(catalog, explicit_locale, payload)


def load_content(catalog: LocaleCatalog, spec: str) -> None:
    explicit_locale, value = parse_content_spec(spec)
    path = resolve_path(value)
    if path_is_dir(path):
        files = sorted(path.glob("*.json"))
        if not files:
            raise ValueError(f"国际化目录没有可识别的语言 JSON 文件: {path}")
        for file in files:
            payload, _ = read_json_content(str(file))
            locale = file.stem if is_locale(file.stem) else None
            add_content(catalog, locale, payload)
        return
    payload, source = read_json_content(value)
    inferred_locale = explicit_locale
    if inferred_locale is None and path.suffix.lower() == ".json" and is_locale(path.stem):
        inferred_locale = path.stem
    try:
        add_content(catalog, inferred_locale, payload)
    except ValueError as error:
        raise ValueError(f"{source}: {error}") from error


def register_i18n(
    i18ns: dict[str, str], source: str, target: str
) -> None:
    if not source or not target or source == target:
        return
    current = i18ns.get(source)
    if current is None or current == source:
        i18ns[source] = target


def build_i18ns(
    catalog: LocaleCatalog,
    source_locale: str,
    target_locale: str,
    source_values: set[str],
) -> dict[str, str]:
    i18ns: dict[str, str] = {}
    source_messages = catalog.messages.get(source_locale, {})
    target_messages = catalog.messages.get(target_locale, {})
    for key in sorted(set(source_messages) & set(target_messages)):
        register_i18n(i18ns, source_messages[key], target_messages[key])
    for source, target in sorted(catalog.direct.get(target_locale, {}).items()):
        if source in source_values:
            register_i18n(i18ns, source, target)
    for source in sorted(source_values):
        target = target_messages.get(source)
        local = local_translate(source, target_locale) if target_locale in {"en-US", "ja-JP"} else source
        local_is_complete = local != source and (
            (target_locale == "en-US" and not contains_cjk(local))
            or (target_locale != "en-US" and not contains_simplified(local))
        )
        if local_is_complete:
            i18ns[source] = local
        elif target and not contains_simplified(target):
            register_i18n(i18ns, source, target)
        elif local != source:
            register_i18n(i18ns, source, local)
    return i18ns


def automatic_i18ns(
    source_values: set[str],
    existing: dict[str, str],
    source_locale: str,
    target_locale: str,
    offline: bool,
) -> tuple[dict[str, str], int]:
    """为未命中的 OpenAPI 文案生成机器翻译，并返回未翻译数量。"""
    pending = sorted(value for value in source_values if value not in existing)
    if not pending:
        return {}, 0
    if target_locale == "zh-TW":
        converter = load_opencc()
        translated = [converter.convert(value) for value in pending]
    else:
        translated = [local_translate(value, target_locale) for value in pending]
        unresolved = [
            source
            for source, target in zip(pending, translated)
            if (not target or source == target) and contains_cjk(source)
        ]
        if unresolved and not offline:
            remote_translated = i18n_batch(
                unresolved,
                source_locale.split("-", 1)[0],
                target_locale.split("-", 1)[0],
                offline,
            )
            remote_by_source = dict(zip(unresolved, remote_translated))
            translated = [remote_by_source.get(source, target) for source, target in zip(pending, translated)]
        if not offline:
            retry_sources = [
                source
                for source, target in zip(pending, translated)
                if (not target or source == target) and contains_cjk(source)
            ]
            if retry_sources:
                retry_results: list[str] = []
                for source in retry_sources:
                    protected, protected_values = protect_text(source, 0)
                    try:
                        retry = request_i18n(
                            protected,
                            source_locale.split("-", 1)[0],
                            target_locale.split("-", 1)[0],
                        )
                    except RuntimeError:
                        retry = ""
                    retry_results.append(
                        restore_text(retry, protected_values) if retry else ""
                    )
                retry_by_source = dict(zip(retry_sources, retry_results))
                translated = [
                    retry_by_source.get(source, target)
                    for source, target in zip(pending, translated)
                ]
    generated: dict[str, str] = {}
    untranslated = 0
    for source, target in zip(pending, translated):
        if not target or target == source:
            untranslated += 1
            continue
        generated[source] = target
    return generated, untranslated


def parse_scalar(value: str) -> str | None:
    value = value.strip()
    if not value or value in {"|", ">", "|-", ">-", "|+", ">+"}:
        return None
    if value.startswith('"'):
        try:
            result = json.loads(value)
        except json.JSONDecodeError:
            return None
        return result if isinstance(result, str) else None
    if value.startswith("'") and value.endswith("'"):
        return value[1:-1].replace("''", "'")
    comment = re.search(r"\s+#", value)
    if comment:
        value = value[: comment.start()].rstrip()
    return value


def render_scalar(value: str) -> str:
    yaml_boolean_or_null = value.lower() in {
        "null",
        "~",
        "true",
        "false",
        "yes",
        "no",
        "on",
        "off",
        ".nan",
        ".inf",
        "-.inf",
    }
    yaml_number = re.fullmatch(r"[-+]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][-+]?\d+)?", value)
    if (
        value
        and "\n" not in value
        and "\r" not in value
        and not yaml_boolean_or_null
        and not yaml_number
        and not value.startswith(("- ", "? ", ": ", "#", "!", "&", "*", "{", "[", "|", ">"))
        and not re.search(r"[\s]#|:\s", value)
    ):
        return value
    return json.dumps(value, ensure_ascii=False)


def source_values(document: str) -> set[str]:
    values: set[str] = set()
    for line in document.splitlines():
        match = FIELD_PATTERN.match(line)
        if not match:
            continue
        value = parse_scalar(match.group("value"))
        if value:
            values.add(value)
    return values


def localize_document(document: str, i18ns: dict[str, str]) -> tuple[str, int]:
    changed = 0
    lines: list[str] = []
    for line in document.splitlines(keepends=True):
        match = FIELD_PATTERN.match(line)
        if not match:
            lines.append(line)
            continue
        source = parse_scalar(match.group("value"))
        target = i18ns.get(source or "")
        if not target:
            lines.append(line)
            continue
        newline = match.group("newline") or ""
        lines.append(
            f"{match.group('prefix')}{match.group('name')}{match.group('separator')}"
            f"{render_scalar(target)}{newline}"
        )
        changed += 1
    return "".join(lines), changed


def parse_locales(values: list[str] | None, source_locale: str) -> list[str]:
    if not is_locale(source_locale):
        raise ValueError(f"源语言代码不是有效的 BCP 47 代码: {source_locale}")
    if not values:
        return [locale for locale in DEFAULT_LOCALES if locale.casefold() != source_locale.casefold()]
    result: list[str] = []
    for value in values:
        for locale in value.split(","):
            locale = locale.strip()
            if not is_locale(locale):
                raise ValueError(f"语言代码不是有效的 BCP 47 代码: {locale}")
            if locale.casefold() == source_locale.casefold():
                continue
            if locale not in result:
                result.append(locale)
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description="生成本地化 OpenAPI YAML 文件")
    parser.add_argument("--input", "--openapi", default=str(DEFAULT_INPUT), help="OpenAPI 源 YAML 文件")
    parser.add_argument("--output-dir", default=str(DEFAULT_OUTPUT_DIR), help="本地化文件输出目录")
    parser.add_argument("--source-locale", default="zh-CN", help="OpenAPI 源文语言")
    parser.add_argument("--locale", "--locales", dest="locales", action="append", help="目标语言，可重复或逗号分隔")
    parser.add_argument(
        "--i18n-content",
        "--i18n",
        "--content",
        dest="contents",
        action="append",
        help="国际化目录、JSON 文件或 语言=路径/JSON；可重复传入",
    )
    parser.add_argument(
        "--auto-i18n",
        "--machine",
        dest="auto_translate",
        action="store_true",
        help="为未命中的文案启用机器翻译；繁体中文使用 OpenCC，其他语言使用 Google V1",
    )
    parser.add_argument(
        "--offline",
        action="store_true",
        help="使用离线翻译；英文仅使用现有内置术语表，繁体中文使用 OpenCC",
    )
    args = parser.parse_args()

    try:
        input_path = resolve_path(args.input)
        output_dir = resolve_path(args.output_dir)
        document = input_path.read_text(encoding="utf-8")
        if "openapi:" not in document or "paths:" not in document:
            raise ValueError(f"不是有效的 OpenAPI YAML 文档: {input_path}")
        locales = parse_locales(args.locales, args.source_locale)
        catalog = LocaleCatalog()
        content_specs = args.contents or [str(path) for path in DEFAULT_CONTENT]
        for spec in content_specs:
            load_content(catalog, spec)
        values = source_values(document)
        output_dir.mkdir(parents=True, exist_ok=True)
        for locale in locales:
            i18ns = build_i18ns(catalog, args.source_locale, locale, values)
            auto_translated = 0
            untranslated = 0
            if args.auto_translate or args.offline:
                generated, untranslated = automatic_i18ns(
                    values,
                    i18ns,
                    args.source_locale,
                    locale,
                    args.offline,
                )
                i18ns.update(generated)
                auto_translated = len(generated)
            localized, changed = localize_document(document, i18ns)
            output_path = output_dir / f"openapi.{locale}.yaml"
            if output_path == input_path:
                raise ValueError(f"本地化输出文件不能覆盖源文件: {output_path}")
            output_path.write_text(localized, encoding="utf-8")
            message = (
                f"{locale}: 写入 {output_path}，替换 {changed} 个字段，"
                f"匹配 {len(i18ns)} 条消息"
            )
            if args.auto_translate or args.offline:
                message += f"，自动翻译 {auto_translated} 条"
                if untranslated:
                    message += f"，{untranslated} 条未翻译"
                    print(
                        f"{locale}: {untranslated} 条文案未获得机器译文，已保留源文。",
                        file=sys.stderr,
                    )
            print(message)
    except (OSError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
