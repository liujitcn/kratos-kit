#!/usr/bin/env python3
"""发布前只读校验国际化语言包及其生成产物。"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
ROOT = SCRIPT_DIR.parent
sys.path.insert(0, str(SCRIPT_DIR))

from generate_locale_drafts import parse_primary_i18n_sources  # noqa: E402
from sync_locales import (  # noqa: E402
    DEFAULT_LOCALE,
    FRONTEND_GENERATED_FILES,
    LOCALE_DIRECTORIES,
    locale_files,
    render_frontend_generated,
    validate_locale_sets,
)


LOCALE_PATTERN = re.compile(r"^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$")
SQL_FILE_PATTERN = re.compile(r"^i18n\.(?P<locale>[^.]+)\.up\.sql$")
SQL_INSERT_PREFIX = "INSERT IGNORE INTO `base_i18n`"
OPENAPI_LOCALIZED_FIELD_PATTERN = re.compile(
    r"^(?P<indent>\s*)(?P<field>description|summary|title):(?:\s.*)?$"
)
OPENAPI_REQUIRED_MARKERS = ("openapi:", "info:", "paths:", "components:", "tags:")
OPENAPI_CJK_PATTERN = re.compile(r"[一-龥]")
OPENAPI_JA_UNTRANSLATED_TERMS = ("出库", "入库")
SQL_SOURCE_LOCALE = "en-US"
SOURCE_LOCALE = DEFAULT_LOCALE


class VerificationError(ValueError):
    """表示一个国际化校验项失败。"""


def parse_locales(value: str) -> list[str]:
    """解析并去重命令行传入的目标语言。"""
    locales: list[str] = []
    for item in value.split(","):
        locale = item.strip()
        if not locale:
            continue
        if not LOCALE_PATTERN.fullmatch(locale):
            raise VerificationError(f"语言代码不是有效的 BCP 47 代码: {locale}")
        if locale not in locales:
            locales.append(locale)
    if not locales:
        raise VerificationError("目标语言列表不能为空")
    return locales


def parse_sql_values(line: str) -> list[str | None] | None:
    """解析单行 SQL VALUES，兼容转义单引号、反斜杠和文本逗号。"""
    values_match = re.search(r"VALUES\s*\((.*)\)\s*;\s*$", line)
    if not values_match:
        return None

    values: list[str | None] = []
    current: list[str] = []
    quoted = False
    escaped = False
    value_text = values_match.group(1)
    index = 0
    while index < len(value_text):
        char = value_text[index]
        if quoted:
            if escaped:
                current.append(char)
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == "'":
                if index + 1 < len(value_text) and value_text[index + 1] == "'":
                    current.append("'")
                    index += 1
                else:
                    quoted = False
            else:
                current.append(char)
        elif char == "'":
            quoted = True
        elif char == ",":
            value = "".join(current).strip()
            values.append(None if value.upper() == "NULL" else value)
            current = []
        else:
            current.append(char)
        index += 1

    if quoted or escaped:
        return None
    value = "".join(current).strip()
    values.append(None if value.upper() == "NULL" else value)
    return values


def parse_sql_records(path: Path) -> list[tuple[int, int, str, str]]:
    """读取一个 i18n SQL 文件中的翻译记录。"""
    records: list[tuple[int, int, str, str]] = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if SQL_INSERT_PREFIX not in line:
            continue
        values = parse_sql_values(line)
        if values is None or len(values) != 4:
            raise VerificationError(f"{path}:{line_number} 的 base_i18n INSERT 格式无效")
        try:
            target_type = int(values[0] or "")
            target_id = int(values[1] or "")
        except ValueError as error:
            raise VerificationError(f"{path}:{line_number} 的翻译目标编号无效") from error
        locale = str(values[2] or "")
        name = str(values[3] or "")
        if not locale:
            raise VerificationError(f"{path}:{line_number} 的翻译语言为空")
        records.append((target_type, target_id, locale, name))
    if not records:
        raise VerificationError(f"SQL 文件没有 base_i18n 翻译记录: {path}")
    return records


def verify_language_bundles() -> list[str]:
    """校验语言集合、语言键、占位符和前端注册生成物。"""
    file_sets = {name: locale_files(directory) for name, directory in LOCALE_DIRECTORIES.items()}
    locales = validate_locale_sets(file_sets)
    for name, path in FRONTEND_GENERATED_FILES.items():
        expected = render_frontend_generated(locales, name == "admin-core", name == "admin-core")
        if not path.exists() or path.read_text(encoding="utf-8") != expected:
            raise VerificationError(f"前端语言注册生成物过期，请执行 make i18n-sync: {path}")
    return locales


def verify_sql(locales: list[str], source_locale: str) -> None:
    """校验每个迁移目录中的语言 SQL 文件集合和翻译主键集合。"""
    expected_locales = set(locales) - {source_locale}
    migration_root = ROOT / "backend/migration/assets"
    groups: dict[Path, dict[str, Path]] = {}
    for path in migration_root.glob("*/mysql/i18n.*.up.sql"):
        match = SQL_FILE_PATTERN.fullmatch(path.name)
        if match is None or not LOCALE_PATTERN.fullmatch(match.group("locale")):
            raise VerificationError(f"SQL 文件名不是 i18n.<locale>.up.sql: {path}")
        groups.setdefault(path.parent, {})[match.group("locale")] = path

    if not groups:
        return

    covered_locales: set[str] = set()
    covered_keys: set[tuple[int, int]] = set()
    for directory, files in sorted(groups.items(), key=lambda item: str(item[0])):
        actual_locales = set(files)
        extra = sorted(actual_locales - expected_locales)
        if extra:
            raise VerificationError(f"{directory} 包含未注册语言 SQL: {', '.join(extra)}")
        covered_locales.update(actual_locales)

        reference_locale = SQL_SOURCE_LOCALE if SQL_SOURCE_LOCALE in files else sorted(files)[0]
        reference_records = parse_sql_records(files[reference_locale])
        reference_keys = {(item[0], item[1]) for item in reference_records}
        covered_keys.update(reference_keys)
        if len(reference_keys) != len(reference_records):
            raise VerificationError(f"SQL 翻译记录重复: {files[reference_locale]}")
        for locale, path in sorted(files.items()):
            records = parse_sql_records(path)
            keys = {(item[0], item[1]) for item in records}
            if len(keys) != len(records):
                raise VerificationError(f"SQL 翻译记录重复: {path}")
            if keys != reference_keys:
                missing_keys = sorted(reference_keys - keys)
                extra_keys = sorted(keys - reference_keys)
                raise VerificationError(
                    f"{path} 与 {files[reference_locale]} 的翻译目标集合不一致: "
                    f"缺少 {missing_keys[:5]}，多出 {extra_keys[:5]}"
                )
            invalid_locales = sorted({item[2] for item in records if item[2] != locale})
            if invalid_locales:
                raise VerificationError(f"{path} 包含错误的 locale: {', '.join(invalid_locales)}")

    missing_locales = sorted(expected_locales - covered_locales)
    if missing_locales:
        raise VerificationError(f"迁移目录缺少当前语言的 SQL 翻译: {', '.join(missing_locales)}")

    default_translatable_keys: set[tuple[int, int]] = set()
    for default_data in migration_root.glob("*/mysql/default_data.up.sql"):
        default_translatable_keys.update(
            (target_type, target_id)
            for (target_type, target_id), value in parse_primary_i18n_sources(default_data).items()
            if target_type in {2, 3, 4, 5, 6} and value
        )
    missing_translations = sorted(default_translatable_keys - covered_keys)
    if missing_translations:
        raise VerificationError(
            "默认数据缺少国际化 SQL 翻译: "
            + ", ".join(f"({target_type}, {target_id})" for target_type, target_id in missing_translations)
        )


def normalized_openapi(text: str) -> tuple[str, ...]:
    """屏蔽可翻译字段值，保留 OpenAPI 契约结构和非翻译内容。"""
    normalized: list[str] = []
    for line in text.splitlines():
        match = OPENAPI_LOCALIZED_FIELD_PATTERN.fullmatch(line)
        if match:
            normalized.append(f"{match.group('indent')}{match.group('field')}: <localized>")
        else:
            normalized.append(line)
    return tuple(normalized)


def verify_openapi(target_locales: list[str]) -> None:
    """校验 OpenAPI 源文档和各语言文档的契约结构一致。"""
    output_dir = ROOT / "backend/internal/openapi/assets"
    source_path = output_dir / "openapi.yaml"
    source_text = source_path.read_text(encoding="utf-8")
    if any(marker not in source_text for marker in OPENAPI_REQUIRED_MARKERS):
        raise VerificationError(f"OpenAPI 源文档缺少必要结构: {source_path}")
    source_shape = normalized_openapi(source_text)
    for locale in target_locales:
        path = output_dir / f"openapi.{locale}.yaml"
        if not path.exists():
            raise VerificationError(f"缺少 OpenAPI {locale} 文档: {path}")
        text = path.read_text(encoding="utf-8")
        if any(marker not in text for marker in OPENAPI_REQUIRED_MARKERS):
            raise VerificationError(f"OpenAPI {locale} 文档缺少必要结构: {path}")
        if normalized_openapi(text) != source_shape:
            raise VerificationError(
                f"OpenAPI {locale} 文档与源文档结构不一致，请执行 make i18n-openapi: {path}"
            )
        localized_fields = [
            line
            for line in text.splitlines()
            if OPENAPI_LOCALIZED_FIELD_PATTERN.fullmatch(line)
        ]
        if locale == "en-US":
            residual = [line.strip() for line in localized_fields if OPENAPI_CJK_PATTERN.search(line)]
            if residual:
                raise VerificationError(
                    f"OpenAPI {locale} 文案仍包含中文，请补充本地化术语: {path}: {residual[:3]}"
                )
        if locale == "ja-JP":
            residual = [
                line.strip()
                for line in localized_fields
                if any(term in line for term in OPENAPI_JA_UNTRANSLATED_TERMS)
            ]
            if residual:
                raise VerificationError(
                    f"OpenAPI {locale} 文案仍包含简体中文术语，请补充本地化术语: {path}: {residual[:3]}"
                )


def main() -> int:
    """执行全部国际化发布前校验。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-locale", default=SOURCE_LOCALE, help="语言包主语言")
    parser.add_argument("--locales", default="en-US,zh-TW,ja-JP", help="OpenAPI 目标语言")
    args = parser.parse_args()

    try:
        target_locales = parse_locales(args.locales)
        target_locales = [locale for locale in target_locales if locale != args.source_locale]
    except VerificationError as error:
        print(f"国际化发布校验失败: {error}", file=sys.stderr)
        return 1

    errors: list[str] = []
    locales: list[str] | None = None
    try:
        locales = verify_language_bundles()
    except (OSError, json.JSONDecodeError, VerificationError, ValueError) as error:
        errors.append(f"语言包: {error}")

    if locales is not None:
        if args.source_locale not in locales:
            errors.append(f"语言包: 缺少主语言 {args.source_locale}")
        try:
            verify_sql(locales, SOURCE_LOCALE)
        except (OSError, json.JSONDecodeError, VerificationError, ValueError) as error:
            errors.append(f"SQL: {error}")

    try:
        verify_openapi(target_locales)
    except (OSError, json.JSONDecodeError, VerificationError, ValueError) as error:
        errors.append(f"OpenAPI: {error}")

    if errors:
        print("国际化发布校验失败:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    assert locales is not None

    print(
        "国际化发布校验通过："
        f"语言包({len(locales)})、SQL({len(set(locales) - {SOURCE_LOCALE})}种目标语言)、"
        f"OpenAPI({len(target_locales)}种目标语言)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
