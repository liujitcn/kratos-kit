#!/usr/bin/env python3
"""同步后端与三个前端 workspace 的语言包清单和注册产物。"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_LOCALE = "zh-CN"
LANGUAGE_LABEL_PREFIX = "common.language."

def discover_locale_directories() -> dict[str, Path]:
    """发现后端、Core 以及所有前端业务模块的语言目录。"""
    directories = {"backend": ROOT / "backend/internal/i18n/assets"}
    for terminal in ("admin", "uni-app", "taro-app"):
        modules_root = ROOT / f"frontend/{terminal}/packages/modules"
        for path in sorted(modules_root.glob("*/src/locales")):
            if path.is_dir():
                module_name = path.parent.parent.name
                directories[f"{terminal}-{module_name}"] = path
    return directories


def discover_frontend_generated_files(locale_directories: dict[str, Path]) -> dict[str, Path]:
    """根据已发现的前端语言目录定位生成的注册文件。"""
    return {
        name: directory / "generated.ts"
        for name, directory in locale_directories.items()
        if name != "backend"
    }


LOCALE_DIRECTORIES = discover_locale_directories()
FRONTEND_GENERATED_FILES = discover_frontend_generated_files(LOCALE_DIRECTORIES)

CODEGEN_MESSAGE_PREFIX = "system.code.gen."
COMMON_MESSAGE_PREFIX = "common."
DAYJS_LOCALE_DIRECTORY = ROOT / "frontend/admin/packages/core/node_modules/dayjs/locale"
ELEMENT_LOCALE_DIRECTORY = ROOT / "frontend/admin/packages/core/node_modules/element-plus/es/locale/lang"
MIGRATION_VERSION_PATTERN = re.compile(r"^v\d+\.\d+\.\d+$")
LOCALE_CODE_PATTERN = re.compile(r"^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$")


def locale_files(directory: Path) -> dict[str, Path]:
    files = {
        path.stem: path
        for path in directory.glob("*.json")
    }
    if not files:
        raise ValueError(f"语言目录为空: {directory}")
    for locale, path in files.items():
        if not LOCALE_CODE_PATTERN.fullmatch(locale):
            raise ValueError(f"语言文件名不是 BCP 47 代码: {path}")
        try:
            json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as error:
            raise ValueError(f"解析语言包失败: {path}: {error}") from error
    if DEFAULT_LOCALE not in files:
        raise ValueError(f"语言目录缺少默认语言 {DEFAULT_LOCALE}: {directory}")
    return files


def ordered_locales(locales: set[str]) -> list[str]:
    return sorted(locales, key=lambda value: (value != DEFAULT_LOCALE, value))


def required_message_keys(messages: dict[str, object]) -> list[str]:
    """返回不依赖语言集合的必需消息键。"""
    return sorted(key for key in messages if not key.startswith(LANGUAGE_LABEL_PREFIX))


def message_placeholders(value: object) -> list[str]:
    if isinstance(value, dict):
        value = value.get("other", "")
    if not isinstance(value, str):
        return []
    return sorted(re.findall(r"\{([A-Za-z0-9_]+)\}", value))


def validate_locale_sets(file_sets: dict[str, dict[str, Path]]) -> list[str]:
    expected = set(file_sets["backend"])
    for name, files in file_sets.items():
        actual = set(files)
        if actual != expected:
            missing = ", ".join(sorted(expected - actual)) or "无"
            extra = ", ".join(sorted(actual - expected)) or "无"
            raise ValueError(f"{name} 语言集合不一致，缺少: {missing}，多出: {extra}")
    for name, files in file_sets.items():
        reference = json.loads(files[DEFAULT_LOCALE].read_text(encoding="utf-8"))
        reference_keys = required_message_keys(reference)
        for locale, path in files.items():
            messages = json.loads(path.read_text(encoding="utf-8"))
            if required_message_keys(messages) != reference_keys:
                raise ValueError(f"{name}/{locale} 与 {DEFAULT_LOCALE} 的语言键集合不一致")
            for key in reference_keys:
                expected_placeholders = message_placeholders(reference[key])
                actual_placeholders = message_placeholders(messages[key])
                if actual_placeholders != expected_placeholders:
                    raise ValueError(
                        f"{name}/{locale} 文案占位符不一致: {key}，"
                        f"期望 {expected_placeholders}，实际 {actual_placeholders}"
                    )
    backend_messages = {
        locale: json.loads(path.read_text(encoding="utf-8"))
        for locale, path in file_sets["backend"].items()
    }
    codegen_keys = sorted(key for key in backend_messages[DEFAULT_LOCALE] if key.startswith(CODEGEN_MESSAGE_PREFIX))
    for locale, messages in backend_messages.items():
        actual_keys = sorted(key for key in messages if key.startswith(CODEGEN_MESSAGE_PREFIX))
        if actual_keys != codegen_keys:
            raise ValueError(f"后端代码生成文案键集合不一致: {locale}")
        for key in codegen_keys:
            if message_placeholders(messages[key]) != message_placeholders(backend_messages[DEFAULT_LOCALE][key]):
                raise ValueError(f"后端代码生成文案占位符不一致: {locale}/{key}")
    common_keys = sorted(key for key in backend_messages[DEFAULT_LOCALE] if key.startswith(COMMON_MESSAGE_PREFIX))
    for locale, messages in backend_messages.items():
        for key in common_keys:
            if message_placeholders(messages[key]) != message_placeholders(backend_messages[DEFAULT_LOCALE][key]):
                raise ValueError(f"后端通用文案占位符不一致: {locale}/{key}")
    return ordered_locales(expected)


def identifier(locale: str) -> str:
    value = re.sub(r"[^A-Za-z0-9]", " ", locale).title().replace(" ", "")
    return value or "Locale"


def package_locale_path(locale: str, directory: Path) -> str:
    normalized = locale.lower()
    candidates = [normalized]
    base = normalized.split("-", 1)[0]
    if base != normalized:
        candidates.append(base)
    for candidate in candidates:
        if any((directory / f"{candidate}{extension}").exists() for extension in (".js", ".mjs")):
            return candidate
    raise ValueError(
        f"{directory.name} 缺少 {locale} 的组件语言映射，请安装依赖或补充对应 locale 文件"
    )


def dayjs_locale_path(locale: str) -> str:
    return package_locale_path(locale, DAYJS_LOCALE_DIRECTORY)


def element_locale_path(locale: str) -> str:
    return package_locale_path(locale, ELEMENT_LOCALE_DIRECTORY)


def language_metadata(file_sets: dict[str, dict[str, Path]], locales: list[str]) -> list[dict[str, str | int]]:
    core_files = file_sets["admin-__FRONTEND_MODULE__"]
    default_messages = json.loads(core_files[DEFAULT_LOCALE].read_text(encoding="utf-8"))
    metadata: list[dict[str, str | int]] = []
    for index, locale in enumerate(locales, start=1):
        language_key = f"common.language.{locale}"
        locale_messages = json.loads(core_files[locale].read_text(encoding="utf-8"))
        native_name = locale_messages.get(language_key) or locale
        language_name = default_messages.get(language_key) or native_name
        if not isinstance(language_name, str) or not language_name:
            language_name = locale
        if not isinstance(native_name, str) or not native_name:
            native_name = locale
        metadata.append(
            {
                "locale": locale,
                "language_name": language_name,
                "native_name": native_name,
                "sort": index * 10,
            }
        )
    return metadata


def sql_literal(value: str) -> str:
    return "'" + value.replace("\\", "\\\\").replace("'", "\\'") + "'"


def render_language_migration(metadata: list[dict[str, str | int]]) -> str:
    lines = [
        "-- 语言包同步生成的语言初始化数据。",
        "-- 可重复执行：只补充不存在的语言，不覆盖数据库中的启用状态、名称和主语言配置。",
        "",
        "SET NAMES utf8mb4;",
        "",
        "INSERT IGNORE INTO `base_language` (`language_code`, `language_name`, `native_name`, `sort`, `is_primary`, `status`, `created_by`, `updated_by`, `created_at`, `updated_at`, `deleted_at`) VALUES",
    ]
    values = []
    for item in metadata:
        values.append(
            "  ("
            + ", ".join(
                [
                    sql_literal(str(item["locale"])),
                    sql_literal(str(item["language_name"])),
                    sql_literal(str(item["native_name"])),
                    str(item["sort"]),
                    "0" if item["locale"] != DEFAULT_LOCALE else "1",
                    "1",
                    "1",
                    "1",
                    "CURRENT_TIMESTAMP",
                    "CURRENT_TIMESTAMP",
                    "0",
                ]
            )
            + ")"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines) + "\n"


def render_language_migration_description(locales: list[str]) -> str:
    return (
        "由 `scripts/sync_locales.py` 根据语言包生成。\n\n"
        "该迁移只补充语言目录中不存在的 `base_language` 记录；数据库中的启用状态、排序和主语言标记由部署配置维护。\n\n"
        f"语言集合：{', '.join(locales)}。\n"
    )


def render_frontend_generated(locales: list[str], include_dayjs: bool, use_double_quotes: bool) -> str:
    quote = '"' if use_double_quotes else "'"
    semicolon = ";" if use_double_quotes else ""

    def js_string(value: str) -> str:
        return f"{quote}{value}{quote}"

    lines = [
        "/* 此文件由 scripts/sync_locales.py 生成，请勿手工修改。 */",
    ]
    if include_dayjs:
        for locale in locales:
            dayjs_path = dayjs_locale_path(locale)
            if dayjs_path:
                lines.append(f'import "dayjs/locale/{dayjs_path}";')
        for locale in locales:
            lines.append(
                f'import elementLocale{identifier(locale)} from "element-plus/es/locale/lang/{element_locale_path(locale)}";'
            )
        lines.append("")
    for locale in locales:
        lines.append(f"import locale{identifier(locale)} from {js_string(f'./{locale}.json')}{semicolon}")
    lines.extend(["", "export const LOCALE_MESSAGES = {"])
    for locale in locales:
        lines.append(f"  {js_string(locale)}: locale{identifier(locale)},")
    lines.extend([
        f"}} as const satisfies Record<string, Record<string, string>>{semicolon}",
        "",
        f"export type GeneratedLocale = keyof typeof LOCALE_MESSAGES{semicolon}",
        f"export const DEFAULT_LOCALE: GeneratedLocale = {js_string(DEFAULT_LOCALE)}{semicolon}",
        f"export const SUPPORTED_LOCALES = Object.keys(LOCALE_MESSAGES) as GeneratedLocale[]{semicolon}",
    ])
    if include_dayjs:
        lines.extend(["", "export const DAYJS_LOCALE_MAP: Record<string, string> = {"])
        for locale in locales:
            dayjs_path = dayjs_locale_path(locale)
            if dayjs_path:
                lines.append(f'  "{locale}": "{dayjs_path}",')
        lines.extend(["};"])
        lines.extend(["", "export const ELEMENT_LOCALES = {"])
        for locale in locales:
            lines.append(f'  "{locale}": elementLocale{identifier(locale)},')
        lines.extend(["} as const;"])
    return "\n".join(lines) + "\n"


def ensure_content(path: Path, content: str, write: bool) -> None:
    current = path.read_text(encoding="utf-8") if path.exists() else ""
    if current == content:
        return
    if not write:
        raise ValueError(f"生成产物过期，请执行同步脚本: {path}")
    path.write_text(content, encoding="utf-8")


def ensure_migration_readme(path: Path, content: str, write: bool) -> None:
    """仅在迁移目录尚无统一说明时创建 README。"""
    if path.exists():
        return
    ensure_content(path, content, write)


def main() -> int:
    parser = argparse.ArgumentParser(description="同步语言包集合和前端注册产物")
    parser.add_argument("--write", action="store_true", help="写入生成产物；默认只检查")
    parser.add_argument(
        "--migration-version",
        help="同时生成指定版本的 base_language 迁移，例如 v0.0.1；不传则不生成迁移",
    )
    args = parser.parse_args()

    try:
        file_sets = {name: locale_files(directory) for name, directory in LOCALE_DIRECTORIES.items()}
        locales = validate_locale_sets(file_sets)
        metadata = language_metadata(file_sets, locales)
        for name, path in FRONTEND_GENERATED_FILES.items():
            ensure_content(path, render_frontend_generated(locales, name == "admin-core", name == "admin-core"), args.write)
        if args.migration_version:
            if not MIGRATION_VERSION_PATTERN.fullmatch(args.migration_version):
                raise ValueError("迁移版本必须是 vX.Y.Z 格式")
            migration_directory = ROOT / "backend/migration/assets" / args.migration_version / "mysql"
            if args.write:
                migration_directory.mkdir(parents=True, exist_ok=True)
            ensure_content(
                migration_directory / "language.up.sql",
                render_language_migration(metadata),
                args.write,
            )
            ensure_migration_readme(
                migration_directory / "README.md",
                render_language_migration_description(locales),
                args.write,
            )
    except (OSError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1

    action = "已写入" if args.write else "检查通过"
    print(f"语言包同步{action}：{', '.join(locales)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
