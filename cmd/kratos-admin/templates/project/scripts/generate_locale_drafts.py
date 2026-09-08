#!/usr/bin/env python3
"""生成非默认语言的固定语言包和初始化翻译数据。"""

from __future__ import annotations

import argparse
from collections import Counter
import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any, Callable


ROOT = Path(__file__).resolve().parents[1]
GOOGLE_CLIENTS = ("gtx", "dict-chrome-ex", "chrome", "at")
DEFAULT_REQUEST_TIMEOUT = 8.0
SQL_DIR = ROOT / "backend/migration/assets/v0.0.1/mysql"
LOCALE_CODE_PATTERN = re.compile(r"^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$")
DEFAULT_TARGET_LOCALES = ("zh-TW",)


def json_sources() -> list[Path]:
    """发现后端 Core 与所有前端业务模块的简体中文语言包。"""
    roots = [
        ROOT / "backend/internal/i18n/assets",
        ROOT / "frontend/admin/packages/modules/__FRONTEND_MODULE__/src/locales",
        ROOT / "frontend/uni-app/packages/modules/__FRONTEND_MODULE__/src/locales",
        ROOT / "frontend/taro-app/packages/modules/__FRONTEND_MODULE__/src/locales",
    ]
    for terminal in ("admin", "uni-app", "taro-app"):
        roots.extend(sorted((ROOT / f"frontend/{terminal}/packages/modules").glob("*/src/locales")))
    return [root / "zh-CN.json" for root in roots if (root / "zh-CN.json").is_file()]


JSON_SOURCES = json_sources()


FALLBACK_I18NS = {
    "ja": {
        "Language Management": "言語管理",
        "Add Language": "言語を追加",
        "Delete Language": "言語を削除",
        "Edit Language": "言語を編集",
        "Change Language Status": "言語ステータスを変更",
        "Set Primary Language": "主言語を設定",
        "Operations Monitoring": "運用監視",
    },
    "ko": {
        "Language Management": "언어 관리",
        "Add Language": "언어 추가",
        "Delete Language": "언어 삭제",
        "Edit Language": "언어 편집",
        "Change Language Status": "언어 상태 변경",
        "Set Primary Language": "기본 언어 설정",
        "Operations Monitoring": "운영 모니터링",
    },
    "fr": {
        "Language Management": "Gestion des langues",
        "Add Language": "Ajouter une langue",
        "Delete Language": "Supprimer une langue",
        "Edit Language": "Modifier une langue",
        "Change Language Status": "Modifier le statut de la langue",
        "Set Primary Language": "Définir la langue principale",
        "Operations Monitoring": "Surveillance des opérations",
    },
    "es": {
        "Language Management": "Gestión de idiomas",
        "Add Language": "Añadir idioma",
        "Delete Language": "Eliminar idioma",
        "Change Language Status": "Cambiar el estado del idioma",
        "Set Primary Language": "Establecer idioma principal",
        "Operations Monitoring": "Supervisión operativa",
        "Please enter": "Introduzca",
        "Please select": "Seleccione",
        "Please upload": "Cargue",
        "cannot be empty": "no puede estar vacío",
        "cannot exceed": "no puede superar",
        "must be greater than": "debe ser mayor que",
        "is required": "es obligatorio",
        "Required": "Obligatorio",
        "Success": "Éxito",
        "Failed": "Error",
        "Loading": "Cargando",
        "Search": "Buscar",
        "Reset": "Restablecer",
        "Cancel": "Cancelar",
        "Confirm": "Confirmar",
        "Save": "Guardar",
        "Delete": "Eliminar",
        "Edit": "Editar",
        "Create": "Crear",
        "Close": "Cerrar",
        "Back": "Volver",
        "Download": "Descargar",
        "Upload": "Cargar",
        "View": "Ver",
        "Enabled": "Habilitado",
        "Disabled": "Deshabilitado",
        "English": "Inglés",
        "Japanese": "Japonés",
        "Korean": "Coreano",
        "French": "Francés",
        "Spanish": "Español",
        "Traditional Chinese": "Chino tradicional",
        "Please": "Por favor",
        "password": "contraseña",
        "Password": "Contraseña",
        "username": "nombre de usuario",
        "Username": "Nombre de usuario",
        "user": "usuario",
        "User": "Usuario",
        "role": "rol",
        "Role": "Rol",
        "menu": "menú",
        "Menu": "Menú",
        "language": "idioma",
        "Language": "Idioma",
        "system": "sistema",
        "System": "Sistema",
        "configuration": "configuración",
        "Configuration": "Configuración",
        "data": "datos",
        "Data": "Datos",
        "file": "archivo",
        "File": "Archivo",
        "page": "página",
        "Page": "Página",
        "name": "nombre",
        "Name": "Nombre",
        "value": "valor",
        "Value": "Valor",
        "type": "tipo",
        "Type": "Tipo",
        "current": "actual",
        "Current": "Actual",
        "No ": "Sin ",
        " not ": " no ",
    },
}

# 迁移脚本以 default_data.up.sql 的中文主数据为源；网络不可用时使用这组固定译文。
SQL_FIXED_I18NS = {
    "ja": {
        "Copyright © 2025 - 2030 Admin All Rights Reserved.": "著作権 © 2025 - 2030 Admin All Rights Reserved.",
        "Admin 管理系统": "Admin 管理システム",
        "应用框架示例": "アプリケーションフレームワークの例",
        "保留通用导航与个人中心体验": "共通ナビゲーションと個人センターの体験を維持",
        "<h2>隐私政策</h2><p>感谢您使用本应用。我们重视并保护您的个人信息，仅在提供账号登录、应用功能和客户服务所必需的范围内处理相关信息。</p><h3>一、信息使用</h3><p>我们会按照法律法规和本协议约定使用您的信息，不会将其用于无关目的。</p><h3>二、您的权利</h3><p>您可以依法查询、更正、删除个人信息，或通过应用内公布的渠道联系我们。</p>": "<h2>プライバシーポリシー</h2><p>本アプリをご利用いただきありがとうございます。お客様の個人情報を尊重して保護し、アカウントへのログイン、アプリ機能、カスタマーサービスの提供に必要な範囲でのみ取り扱います。</p><h3>1. 情報の利用</h3><p>適用される法令および本ポリシーに従って情報を利用し、関係のない目的には使用しません。</p><h3>2. お客様の権利</h3><p>法令に従って個人情報の確認、訂正、削除を行うか、アプリに掲載された窓口からお問い合わせいただけます。</p>",
        "<h2>服务条款</h2><p>欢迎使用本应用。使用本应用前，请您阅读并理解本协议。您开始使用应用服务，即表示同意遵守本协议及相关规则。</p><h3>一、服务使用</h3><p>请您依法、合规并按照页面提示使用各项功能，不得利用本应用从事违法或损害他人权益的活动。</p><h3>二、协议变更</h3><p>我们会根据业务和法律法规变化更新本协议，并在应用内向您展示更新后的内容。</p>": "<h2>利用規約</h2><p>本アプリへようこそ。ご利用前に本規約と関連ルールをお読みいただき、内容をご理解ください。アプリサービスを利用することで、本規約に同意したものとみなされます。</p><h3>1. サービスの利用</h3><p>各機能を法令に従い、ページの案内に沿って適切にご利用ください。本アプリを違法行為や他者の権利を侵害する行為に利用してはなりません。</p><h3>2. 規約の変更</h3><p>事業や法令の変更に応じて本規約を更新し、更新内容をアプリ内に表示します。</p>",
        "Copyright": "著作権",
        "ICP备案号": "ICP登録番号",
        "系统名称": "システム名",
        "水印": "透かし",
        "背景图": "背景画像",
        "登录验证码类型": "ログインCAPTCHAの種類",
        "主标题": "メインタイトル",
        "副标题": "サブタイトル",
        "应用LOGO": "アプリケーションロゴ",
        "隐私协议": "プライバシーポリシー",
        "服务条款": "利用規約",
        "微信未绑定自动注册": "未連携WeChatユーザーの自動登録",
        "登录显示租户编号": "ログイン時にテナント番号を表示",
        "状态": "ステータス",
        "系统配置位置": "システム設定の場所",
        "系统配置类型": "システム設定の種類",
        "验证码类型": "CAPTCHAの種類",
        "定时任务日志状态": "スケジュールジョブログのステータス",
        "资源翻译任务": "リソース翻訳タスク",
        "菜单类型": "メニューの種類",
        "用户角色数据范围": "ユーザーロールのデータ範囲",
        "业务模块": "業務モジュール",
        "代码生成表状态": "コード生成テーブルのステータス",
        "用户性别": "ユーザーの性別",
        "启用": "有効",
        "禁用": "無効",
        "系统内置": "システム組み込み",
        "管理端": "管理画面",
        "应用端": "アプリ側",
        "文本": "テキスト",
        "图片": "画像",
        "富文本": "リッチテキスト",
        "字典": "辞書",
        "布尔": "ブール値",
        "随机验证码": "ランダムCAPTCHA",
        "数字验证码": "数字CAPTCHA",
        "字符串验证码": "文字列CAPTCHA",
        "算术验证码": "算術CAPTCHA",
        "中文验证码": "中国語CAPTCHA",
        "滑动拼图验证码": "スライダーパズルCAPTCHA",
        "点击文字验证码": "クリック文字CAPTCHA",
        "旋转验证码": "回転CAPTCHA",
        "成功": "成功",
        "失败": "失敗",
        "目录": "ディレクトリ",
        "菜单": "メニュー",
        "按钮": "ボタン",
        "外部链接": "外部リンク",
        "全部数据": "すべてのデータ",
        "部门及子部门数据": "部門と子部門のデータ",
        "本部门数据": "現在の部門のデータ",
        "本人数据": "自分のデータ",
        "系统管理": "システム管理",
        "草稿": "下書き",
        "已生成": "生成済み",
        "停用": "停止",
        "保密": "機密",
        "男": "男性",
        "女": "女性",
    },
    "ko": {
        "Copyright © 2025 - 2030 Admin All Rights Reserved.": "저작권 © 2025 - 2030 Admin 판권 소유.",
        "Admin 管理系统": "Admin 관리 시스템",
        "应用框架示例": "애플리케이션 프레임워크 예제",
        "保留通用导航与个人中心体验": "공통 탐색과 개인 센터 경험 유지",
        "<h2>隐私政策</h2><p>感谢您使用本应用。我们重视并保护您的个人信息，仅在提供账号登录、应用功能和客户服务所必需的范围内处理相关信息。</p><h3>一、信息使用</h3><p>我们会按照法律法规和本协议约定使用您的信息，不会将其用于无关目的。</p><h3>二、您的权利</h3><p>您可以依法查询、更正、删除个人信息，或通过应用内公布的渠道联系我们。</p>": "<h2>개인정보 보호정책</h2><p>애플리케이션을 이용해 주셔서 감사합니다. 개인정보를 소중히 보호하며 계정 로그인, 애플리케이션 기능과 고객 지원에 필요한 범위에서만 처리합니다.</p><h3>1. 정보 이용</h3><p>관련 법령과 본 정책에 따라 정보를 이용하며 관련 없는 목적으로 사용하지 않습니다.</p><h3>2. 이용자의 권리</h3><p>법률에 따라 개인정보를 조회, 정정 또는 삭제하거나 애플리케이션에 공개된 채널로 문의할 수 있습니다.</p>",
        "<h2>服务条款</h2><p>欢迎使用本应用。使用本应用前，请您阅读并理解本协议。您开始使用应用服务，即表示同意遵守本协议及相关规则。</p><h3>一、服务使用</h3><p>请您依法、合规并按照页面提示使用各项功能，不得利用本应用从事违法或损害他人权益的活动。</p><h3>二、协议变更</h3><p>我们会根据业务和法律法规变化更新本协议，并在应用内向您展示更新后的内容。</p>": "<h2>서비스 약관</h2><p>애플리케이션에 오신 것을 환영합니다. 사용하기 전에 본 약관과 관련 규칙을 읽고 이해해 주세요. 애플리케이션 서비스를 이용하면 본 약관에 동의한 것으로 봅니다.</p><h3>1. 서비스 이용</h3><p>각 기능을 법령과 페이지 안내에 따라 적법하게 이용해야 하며, 불법 행위나 타인의 권리를 침해하는 활동에 사용할 수 없습니다.</p><h3>2. 약관 변경</h3><p>사업 및 법령의 변경에 따라 약관을 업데이트하고 변경된 내용을 애플리케이션에 표시합니다.</p>",
        "Copyright": "저작권",
        "ICP备案号": "ICP 등록 번호",
        "系统名称": "시스템 이름",
        "水印": "워터마크",
        "背景图": "배경 이미지",
        "登录验证码类型": "로그인 CAPTCHA 유형",
        "主标题": "기본 제목",
        "副标题": "부제목",
        "应用LOGO": "애플리케이션 로고",
        "隐私协议": "개인정보 보호정책",
        "服务条款": "서비스 약관",
        "微信未绑定自动注册": "연결되지 않은 WeChat 사용자 자동 등록",
        "登录显示租户编号": "로그인 시 테넌트 번호 표시",
        "状态": "상태",
        "系统配置位置": "시스템 설정 위치",
        "系统配置类型": "시스템 설정 유형",
        "验证码类型": "CAPTCHA 유형",
        "定时任务日志状态": "예약 작업 로그 상태",
        "菜单类型": "메뉴 유형",
        "用户角色数据范围": "사용자 역할 데이터 범위",
        "业务模块": "비즈니스 모듈",
        "代码生成表状态": "코드 생성 테이블 상태",
        "用户性别": "사용자 성별",
        "启用": "활성화",
        "禁用": "비활성화",
        "系统内置": "시스템 기본 제공",
        "管理端": "관리자 콘솔",
        "应用端": "애플리케이션",
        "文本": "텍스트",
        "图片": "이미지",
        "富文本": "서식 있는 텍스트",
        "字典": "사전",
        "布尔": "부울",
        "随机验证码": "무작위 CAPTCHA",
        "数字验证码": "숫자 CAPTCHA",
        "字符串验证码": "문자열 CAPTCHA",
        "算术验证码": "산술 CAPTCHA",
        "中文验证码": "중국어 CAPTCHA",
        "滑动拼图验证码": "슬라이더 퍼즐 CAPTCHA",
        "点击文字验证码": "텍스트 클릭 CAPTCHA",
        "旋转验证码": "회전 CAPTCHA",
        "成功": "성공",
        "失败": "실패",
        "目录": "디렉터리",
        "菜单": "메뉴",
        "按钮": "버튼",
        "外部链接": "외부 링크",
        "全部数据": "모든 데이터",
        "部门及子部门数据": "부서 및 하위 부서 데이터",
        "本部门数据": "현재 부서 데이터",
        "本人数据": "개인 데이터",
        "系统管理": "시스템 관리",
        "草稿": "초안",
        "已生成": "생성됨",
        "停用": "비활성화",
        "保密": "기밀",
        "男": "남성",
        "女": "여성",
    },
    "fr": {
        "Copyright © 2025 - 2030 Admin All Rights Reserved.": "Droits d’auteur © 2025 - 2030 Admin. Tous droits réservés.",
        "Admin 管理系统": "Système d’administration Admin",
        "应用框架示例": "Exemple de framework applicatif",
        "保留通用导航与个人中心体验": "Conserver l’expérience de navigation commune et du centre personnel",
        "<h2>隐私政策</h2><p>感谢您使用本应用。我们重视并保护您的个人信息，仅在提供账号登录、应用功能和客户服务所必需的范围内处理相关信息。</p><h3>一、信息使用</h3><p>我们会按照法律法规和本协议约定使用您的信息，不会将其用于无关目的。</p><h3>二、您的权利</h3><p>您可以依法查询、更正、删除个人信息，或通过应用内公布的渠道联系我们。</p>": "<h2>Politique de confidentialité</h2><p>Merci d’utiliser cette application. Nous respectons et protégeons vos données personnelles et ne les traitons que dans la mesure nécessaire à la connexion au compte, aux fonctionnalités de l’application et au service client.</p><h3>I. Utilisation des informations</h3><p>Nous utilisons vos informations conformément aux lois applicables et à cette politique, et ne les utilisons pas à des fins étrangères.</p><h3>II. Vos droits</h3><p>Vous pouvez consulter, rectifier ou supprimer vos données personnelles conformément à la loi, ou nous contacter via les canaux publiés dans l’application.</p>",
        "<h2>服务条款</h2><p>欢迎使用本应用。使用本应用前，请您阅读并理解本协议。您开始使用应用服务，即表示同意遵守本协议及相关规则。</p><h3>一、服务使用</h3><p>请您依法、合规并按照页面提示使用各项功能，不得利用本应用从事违法或损害他人权益的活动。</p><h3>二、协议变更</h3><p>我们会根据业务和法律法规变化更新本协议，并在应用内向您展示更新后的内容。</p>": "<h2>Conditions d’utilisation</h2><p>Bienvenue dans cette application. Veuillez lire et comprendre le présent accord et les règles associées avant de l’utiliser. L’utilisation des services de l’application vaut acceptation de ces conditions.</p><h3>I. Utilisation du service</h3><p>Utilisez chaque fonctionnalité légalement et conformément aux indications de la page. Il est interdit d’utiliser cette application pour des activités illégales ou portant atteinte aux droits d’autrui.</p><h3>II. Modification de l’accord</h3><p>Nous pouvons mettre à jour cet accord selon l’évolution de l’activité et de la réglementation, puis afficher la version mise à jour dans l’application.</p>",
        "Copyright": "Droits d’auteur",
        "ICP备案号": "Numéro d’enregistrement ICP",
        "系统名称": "Nom du système",
        "水印": "Filigrane",
        "背景图": "Image d’arrière-plan",
        "登录验证码类型": "Type de CAPTCHA de connexion",
        "主标题": "Titre principal",
        "副标题": "Sous-titre",
        "应用LOGO": "Logo de l’application",
        "隐私协议": "Politique de confidentialité",
        "服务条款": "Conditions d’utilisation",
        "微信未绑定自动注册": "Inscription automatique des utilisateurs WeChat non liés",
        "登录显示租户编号": "Afficher le numéro du locataire à la connexion",
        "状态": "Statut",
        "系统配置位置": "Emplacement de la configuration système",
        "系统配置类型": "Type de configuration système",
        "验证码类型": "Type de CAPTCHA",
        "定时任务日志状态": "Statut des journaux des tâches planifiées",
        "菜单类型": "Type de menu",
        "用户角色数据范围": "Périmètre des données du rôle utilisateur",
        "业务模块": "Module métier",
        "代码生成表状态": "Statut de la table de génération de code",
        "用户性别": "Sexe de l’utilisateur",
        "启用": "Activé",
        "禁用": "Désactivé",
        "系统内置": "Intégré au système",
        "管理端": "Console d’administration",
        "应用端": "Application",
        "文本": "Texte",
        "图片": "Image",
        "富文本": "Texte enrichi",
        "字典": "Dictionnaire",
        "布尔": "Booléen",
        "随机验证码": "CAPTCHA aléatoire",
        "数字验证码": "CAPTCHA numérique",
        "字符串验证码": "CAPTCHA alphanumérique",
        "算术验证码": "CAPTCHA arithmétique",
        "中文验证码": "CAPTCHA chinois",
        "滑动拼图验证码": "CAPTCHA puzzle coulissant",
        "点击文字验证码": "CAPTCHA de texte cliquable",
        "旋转验证码": "CAPTCHA de rotation",
        "成功": "Réussi",
        "失败": "Échec",
        "目录": "Répertoire",
        "菜单": "Menu",
        "按钮": "Bouton",
        "外部链接": "Lien externe",
        "全部数据": "Toutes les données",
        "部门及子部门数据": "Données du service et des sous-services",
        "本部门数据": "Données du service actuel",
        "本人数据": "Données personnelles",
        "系统管理": "Gestion du système",
        "草稿": "Brouillon",
        "已生成": "Généré",
        "停用": "Désactivé",
        "保密": "Confidentiel",
        "男": "Homme",
        "女": "Femme",
    },
    "es": {
        "Copyright © 2025 - 2030 Admin All Rights Reserved.": "Copyright © 2025 - 2030 Admin. Todos los derechos reservados.",
        "Admin 管理系统": "Sistema de administración Admin",
        "应用框架示例": "Ejemplo de framework de aplicación",
        "保留通用导航与个人中心体验": "Mantener la experiencia de navegación común y centro personal",
        "<h2>隐私政策</h2><p>感谢您使用本应用。我们重视并保护您的个人信息，仅在提供账号登录、应用功能和客户服务所必需的范围内处理相关信息。</p><h3>一、信息使用</h3><p>我们会按照法律法规和本协议约定使用您的信息，不会将其用于无关目的。</p><h3>二、您的权利</h3><p>您可以依法查询、更正、删除个人信息，或通过应用内公布的渠道联系我们。</p>": "<h2>Política de privacidad</h2><p>Gracias por utilizar esta aplicación. Valoramos y protegemos tus datos personales y solo los tratamos cuando es necesario para el inicio de sesión, las funciones de la aplicación y la atención al cliente.</p><h3>I. Uso de la información</h3><p>Utilizamos tus datos conforme a la legislación aplicable y a esta política, y no los destinamos a fines ajenos.</p><h3>II. Tus derechos</h3><p>Puedes consultar, corregir o eliminar tus datos personales conforme a la ley, o contactarnos mediante los canales publicados en la aplicación.</p>",
        "<h2>服务条款</h2><p>欢迎使用本应用。使用本应用前，请您阅读并理解本协议。您开始使用应用服务，即表示同意遵守本协议及相关规则。</p><h3>一、服务使用</h3><p>请您依法、合规并按照页面提示使用各项功能，不得利用本应用从事违法或损害他人权益的活动。</p><h3>二、协议变更</h3><p>我们会根据业务和法律法规变化更新本协议，并在应用内向您展示更新后的内容。</p>": "<h2>Términos del servicio</h2><p>Te damos la bienvenida a esta aplicación. Lee y comprende este acuerdo y sus reglas antes de utilizarla. Al usar los servicios de la aplicación aceptas cumplirlos.</p><h3>I. Uso del servicio</h3><p>Utiliza cada función de forma legal y conforme a las indicaciones de la página. No uses esta aplicación para actividades ilegales o que infrinjan los derechos de otras personas.</p><h3>II. Cambios del acuerdo</h3><p>Podemos actualizar este acuerdo según los cambios del negocio y de la legislación, y mostraremos la versión actualizada en la aplicación.</p>",
        "Copyright": "Copyright",
        "ICP备案号": "Número de registro ICP",
        "系统名称": "Nombre del sistema",
        "水印": "Marca de agua",
        "背景图": "Imagen de fondo",
        "登录验证码类型": "Tipo de CAPTCHA de inicio de sesión",
        "主标题": "Título principal",
        "副标题": "Subtítulo",
        "应用LOGO": "Logotipo de la aplicación",
        "隐私协议": "Política de privacidad",
        "服务条款": "Términos del servicio",
        "微信未绑定自动注册": "Registro automático de usuarios de WeChat no vinculados",
        "登录显示租户编号": "Mostrar el código del inquilino al iniciar sesión",
        "状态": "Estado",
        "系统配置位置": "Ubicación de la configuración del sistema",
        "系统配置类型": "Tipo de configuración del sistema",
        "验证码类型": "Tipo de CAPTCHA",
        "定时任务日志状态": "Estado del registro de tareas programadas",
        "菜单类型": "Tipo de menú",
        "用户角色数据范围": "Ámbito de datos del rol de usuario",
        "业务模块": "Módulo empresarial",
        "代码生成表状态": "Estado de la tabla de generación de código",
        "用户性别": "Sexo del usuario",
        "启用": "Habilitado",
        "禁用": "Deshabilitado",
        "系统内置": "Integrado en el sistema",
        "管理端": "Consola de administración",
        "应用端": "Aplicación",
        "文本": "Texto",
        "图片": "Imagen",
        "富文本": "Texto enriquecido",
        "字典": "Diccionario",
        "布尔": "Booleano",
        "随机验证码": "CAPTCHA aleatorio",
        "数字验证码": "CAPTCHA numérico",
        "字符串验证码": "CAPTCHA de texto",
        "算术验证码": "CAPTCHA aritmético",
        "中文验证码": "CAPTCHA chino",
        "滑动拼图验证码": "CAPTCHA de rompecabezas deslizante",
        "点击文字验证码": "CAPTCHA de texto por clic",
        "旋转验证码": "CAPTCHA de rotación",
        "成功": "Éxito",
        "失败": "Error",
        "目录": "Directorio",
        "菜单": "Menú",
        "按钮": "Botón",
        "外部链接": "Enlace externo",
        "全部数据": "Todos los datos",
        "部门及子部门数据": "Datos del departamento y subdepartamentos",
        "本部门数据": "Datos del departamento actual",
        "本人数据": "Datos personales",
        "系统管理": "Gestión del sistema",
        "草稿": "Borrador",
        "已生成": "Generado",
        "停用": "Deshabilitado",
        "保密": "Confidencial",
        "男": "Hombre",
        "女": "Mujer",
    },
}

SQL_MENU_CATEGORIES = {
    "ja": {
        "首页": "ホーム", "AI助手": "AIアシスタント", "个人信息": "プロフィール", "系统管理": "システム管理",
        "菜单管理": "メニュー管理", "字典管理": "辞書管理", "字典属性": "辞書項目", "字典数据": "辞書データ",
        "系统配置": "システム設定", "定时任务": "スケジュールタスク", "定时任务日志": "スケジュールジョブログ",
        "API管理": "API管理", "区域管理": "地域管理", "系统日志": "システムログ", "升级历史": "アップグレード履歴",
        "语言管理": "言語管理", "用户管理": "ユーザー管理", "租户管理": "テナント管理", "角色管理": "ロール管理",
        "部门管理": "部門管理", "岗位管理": "役職管理", "开发工具": "開発ツール", "代码生成": "コード生成",
        "移动端": "モバイルアプリ", "我的": "マイアカウント", "设置": "設定", "AI 助手": "AIアシスタント",
        "运维监控": "運用監視", "代码生成字段配置": "コード生成フィールド設定", "代码生成Proto配置": "コード生成Proto設定",
        "代码生成页面预览": "コード生成ページプレビュー", "代码生成代码预览": "生成コードプレビュー", "API文档": "APIドキュメント",
        "登录": "ログイン", "协议详情": "規約の詳細",
    },
    "ko": {
        "首页": "홈", "AI助手": "AI 어시스턴트", "个人信息": "개인 정보", "系统管理": "시스템 관리",
        "菜单管理": "메뉴 관리", "字典管理": "사전 관리", "字典属性": "사전 항목", "字典数据": "사전 데이터",
        "系统配置": "시스템 설정", "定时任务": "예약 작업", "定时任务日志": "예약 작업 로그", "API管理": "API 관리",
        "区域管理": "지역 관리", "系统日志": "시스템 로그", "升级历史": "업그레이드 기록", "语言管理": "언어 관리",
        "用户管理": "사용자 관리", "租户管理": "테넌트 관리", "角色管理": "역할 관리", "部门管理": "부서 관리",
        "岗位管理": "직위 관리", "开发工具": "개발 도구", "代码生成": "코드 생성", "移动端": "모바일 앱",
        "我的": "내 계정", "设置": "설정", "AI 助手": "AI 어시스턴트",
        "运维监控": "운영 모니터링", "代码生成字段配置": "코드 생성 필드 설정", "代码生成Proto配置": "코드 생성 Proto 설정",
        "代码生成页面预览": "코드 생성 페이지 미리 보기", "代码生成代码预览": "생성 코드 미리 보기", "API文档": "API 문서",
        "登录": "로그인", "协议详情": "약관 상세",
    },
    "fr": {
        "首页": "Accueil", "AI助手": "Assistant IA", "个人信息": "Profil", "系统管理": "Gestion du système",
        "菜单管理": "Gestion des menus", "字典管理": "Gestion des dictionnaires", "字典属性": "Éléments du dictionnaire",
        "字典数据": "Données du dictionnaire", "系统配置": "Configuration système", "定时任务": "Tâches planifiées",
        "定时任务日志": "Journaux des tâches planifiées", "API管理": "Gestion des API", "区域管理": "Gestion des régions",
        "系统日志": "Journaux système", "升级历史": "Historique des mises à niveau", "语言管理": "Gestion des langues",
        "用户管理": "Gestion des utilisateurs", "租户管理": "Gestion des locataires", "角色管理": "Gestion des rôles",
        "部门管理": "Gestion des services", "岗位管理": "Gestion des postes", "开发工具": "Outils de développement",
        "代码生成": "Génération de code", "移动端": "Application mobile", "我的": "Mon compte", "设置": "Paramètres",
        "AI 助手": "Assistant IA",
        "运维监控": "Surveillance des opérations", "代码生成字段配置": "Configuration des champs de génération de code",
        "代码生成Proto配置": "Configuration Proto de génération de code", "代码生成页面预览": "Aperçu de la page générée",
        "代码生成代码预览": "Aperçu du code généré", "API文档": "Documentation API",
        "登录": "Connexion", "协议详情": "Détails des conditions",
    },
    "es": {
        "首页": "Inicio", "AI助手": "Asistente de IA", "个人信息": "Perfil", "系统管理": "Gestión del sistema",
        "菜单管理": "Gestión de menús", "字典管理": "Gestión de diccionarios", "字典属性": "Elementos del diccionario",
        "字典数据": "Datos del diccionario", "系统配置": "Configuración del sistema", "定时任务": "Tareas programadas",
        "定时任务日志": "Registros de tareas programadas", "API管理": "Gestión de API", "区域管理": "Gestión de regiones",
        "系统日志": "Registros del sistema", "升级历史": "Historial de actualizaciones", "语言管理": "Gestión de idiomas",
        "用户管理": "Gestión de usuarios", "租户管理": "Gestión de inquilinos", "角色管理": "Gestión de roles",
        "部门管理": "Gestión de departamentos", "岗位管理": "Gestión de puestos", "开发工具": "Herramientas de desarrollo",
        "代码生成": "Generación de código", "移动端": "Aplicación móvil", "我的": "Mi cuenta", "设置": "Configuración",
        "AI 助手": "Asistente de IA",
        "运维监控": "Supervisión operativa", "代码生成字段配置": "Configuración de campos de generación de código",
        "代码生成Proto配置": "Configuración Proto de generación de código", "代码生成页面预览": "Vista previa de la página generada",
        "代码生成代码预览": "Vista previa del código generado", "API文档": "Documentación de API",
        "登录": "Iniciar sesión", "协议详情": "Detalles de los términos",
    },
}

SQL_MENU_NOUNS = {
    "ja": {
        "菜单": "メニュー", "字典": "辞書", "字典数据": "辞書データ", "配置": "設定", "定时任务": "スケジュールタスク",
        "区域": "地域", "日志": "ログ", "定时任务日志": "スケジュールジョブログ", "API": "API", "租户": "テナント", "用户": "ユーザー", "角色": "ロール",
        "部门": "部門", "岗位": "役職", "语言": "言語", "代码生成表配置": "コード生成テーブル設定",
        "代码生成字段配置": "コード生成フィールド設定", "代码生成Proto配置": "コード生成Proto設定", "代码生成页面": "コード生成ページ",
        "代码生成文件": "コード生成ファイル", "代码生成结果": "コード生成結果", "API文档": "APIドキュメント",
    },
    "ko": {
        "菜单": "메뉴", "字典": "사전", "字典数据": "사전 데이터", "配置": "설정", "定时任务": "예약 작업", "区域": "지역",
        "日志": "로그", "定时任务日志": "예약 작업 로그", "API": "API", "租户": "테넌트", "用户": "사용자", "角色": "역할", "部门": "부서", "岗位": "직위",
        "语言": "언어", "代码生成表配置": "코드 생성 테이블 설정", "代码生成字段配置": "코드 생성 필드 설정",
        "代码生成Proto配置": "코드 생성 Proto 설정", "代码生成页面": "코드 생성 페이지", "代码生成文件": "코드 생성 파일",
        "代码生成结果": "코드 생성 결과", "API文档": "API 문서",
    },
    "fr": {
        "菜单": "menu", "字典": "dictionnaire", "字典数据": "données du dictionnaire", "配置": "configuration",
        "定时任务": "tâche planifiée", "区域": "région", "日志": "journal", "定时任务日志": "journaux des tâches planifiées", "API": "API", "租户": "locataire",
        "用户": "utilisateur", "角色": "rôle", "部门": "service", "岗位": "poste", "语言": "langue",
        "代码生成表配置": "configuration de table de génération de code", "代码生成字段配置": "configuration de champ de génération de code",
        "代码生成Proto配置": "configuration Proto de génération de code", "代码生成页面": "page de génération de code",
        "代码生成文件": "fichiers générés", "代码生成结果": "résultats de génération de code", "API文档": "documentation API",
    },
    "es": {
        "菜单": "menú", "字典": "diccionario", "字典数据": "datos del diccionario", "配置": "configuración",
        "定时任务": "tarea programada", "区域": "región", "日志": "registro", "定时任务日志": "registros de tareas programadas", "API": "API", "租户": "inquilino",
        "用户": "usuario", "角色": "rol", "部门": "departamento", "岗位": "puesto", "语言": "idioma",
        "代码生成表配置": "configuración de tabla de generación de código", "代码生成字段配置": "configuración de campo de generación de código",
        "代码生成Proto配置": "configuración Proto de generación de código", "代码生成页面": "página de generación de código",
        "代码生成文件": "archivos generados", "代码生成结果": "resultados de generación de código", "API文档": "documentación de API",

    },
}


def i18n_sql_menu(text: str, target: str) -> str:
    """按中文菜单源文生成稳定的菜单译文。"""
    if target not in SQL_MENU_CATEGORIES:
        return fallback_translate(text, target)
    categories = SQL_MENU_CATEGORIES.get(target, {})
    nouns = SQL_MENU_NOUNS.get(target, {})
    if text in categories:
        return categories[text]
    special = {
        "查询API详情": {"ja": "API詳細を表示", "ko": "API 상세 조회", "fr": "Voir les détails de l’API", "es": "Ver detalles de API"},
        "设置API MCP工具状态": {"ja": "API MCPツールの状態を設定", "ko": "API MCP 도구 상태 설정", "fr": "Définir l’état de l’outil MCP de l’API", "es": "Configurar el estado de la herramienta MCP de API"},
        "设置API Agent工具状态": {"ja": "API Agentツールの状態を設定", "ko": "API Agent 도구 상태 설정", "fr": "Définir l’état de l’outil Agent de l’API", "es": "Configurar el estado de la herramienta Agent de API"},
        "编辑API配置": {"ja": "API設定を編集", "ko": "API 설정 편집", "fr": "Modifier la configuration de l’API", "es": "Editar la configuración de API"},
        "刷新配置缓存": {"ja": "設定キャッシュを更新", "ko": "설정 캐시 새로 고침", "fr": "Actualiser le cache de configuration", "es": "Actualizar la caché de configuración"},
        "设置主语言": {"ja": "主言語を設定", "ko": "기본 언어 설정", "fr": "Définir la langue principale", "es": "Establecer el idioma principal"},
        "分配角色权限": {"ja": "ロール権限を割り当て", "ko": "역할 권한 할당", "fr": "Attribuer les autorisations du rôle", "es": "Asignar permisos de rol"},
        "重置用户密码": {"ja": "ユーザーパスワードをリセット", "ko": "사용자 비밀번호 재설정", "fr": "Réinitialiser le mot de passe utilisateur", "es": "Restablecer la contraseña del usuario"},
        "预览代码生成页面": {"ja": "コード生成ページをプレビュー", "ko": "코드 생성 페이지 미리 보기", "fr": "Prévisualiser la page générée", "es": "Previsualizar la página generada"},
        "预览代码生成文件": {"ja": "コード生成ファイルをプレビュー", "ko": "코드 생성 파일 미리 보기", "fr": "Prévisualiser les fichiers générés", "es": "Previsualizar los archivos generados"},
        "执行代码生成": {"ja": "コード生成を実行", "ko": "코드 생성 실행", "fr": "Exécuter la génération de code", "es": "Ejecutar la generación de código"},
        "还原代码生成结果": {"ja": "コード生成結果を復元", "ko": "코드 생성 결과 복원", "fr": "Restaurer les résultats de génération", "es": "Restaurar los resultados de generación"},
        "维护代码生成字段配置": {"ja": "コード生成フィールド設定を保守", "ko": "코드 생성 필드 설정 관리", "fr": "Maintenir la configuration des champs de génération", "es": "Mantener la configuración de campos de generación"},
        "维护代码生成Proto配置": {"ja": "コード生成Proto設定を保守", "ko": "코드 생성 Proto 설정 관리", "fr": "Maintenir la configuration Proto de génération", "es": "Mantener la configuración Proto de generación"},
        "查询定时任务日志详情": {"ja": "スケジュールジョブログの詳細を表示", "ko": "예약 작업 로그 상세 조회", "fr": "Voir les détails des journaux des tâches planifiées", "es": "Ver detalles de los registros de tareas programadas"},
    }
    if text in special:
        return special[text][target]
    if text.startswith("新增"):
        noun = nouns.get(text[2:])
        if noun:
            return {"ja": f"{noun}を追加", "ko": f"{noun} 추가", "fr": f"Ajouter {noun}", "es": f"Añadir {noun}"}[target]
    if text.startswith("删除"):
        noun = nouns.get(text[2:])
        if noun:
            return {"ja": f"{noun}を削除", "ko": f"{noun} 삭제", "fr": f"Supprimer {noun}", "es": f"Eliminar {noun}"}[target]
    if text.startswith("编辑"):
        noun = nouns.get(text[2:])
        if noun:
            return {"ja": f"{noun}を編集", "ko": f"{noun} 편집", "fr": f"Modifier {noun}", "es": f"Editar {noun}"}[target]
    if text.startswith("修改") and text.endswith("状态"):
        noun = nouns.get(text[2:-2])
        if noun:
            return {"ja": f"{noun}のステータスを変更", "ko": f"{noun} 상태 변경", "fr": f"Modifier le statut de {noun}", "es": f"Cambiar el estado de {noun}"}[target]
    if text.startswith("启动"):
        noun = nouns.get(text[2:])
        if noun:
            return {"ja": f"{noun}を開始", "ko": f"{noun} 시작", "fr": f"Démarrer {noun}", "es": f"Iniciar {noun}"}[target]
    if text.startswith("停止"):
        noun = nouns.get(text[2:])
        if noun:
            return {"ja": f"{noun}を停止", "ko": f"{noun} 중지", "fr": f"Arrêter {noun}", "es": f"Detener {noun}"}[target]
    if text.startswith("执行"):
        noun = nouns.get(text[2:])
        if noun:
            return {"ja": f"{noun}を実行", "ko": f"{noun} 실행", "fr": f"Exécuter {noun}", "es": f"Ejecutar {noun}"}[target]
    if text.startswith("查询") and text.endswith("详情"):
        noun = nouns.get(text[2:-2])
        if noun:
            return {"ja": f"{noun}の詳細を表示", "ko": f"{noun} 상세 조회", "fr": f"Voir les détails de {noun}", "es": f"Ver detalles de {noun}"}[target]
    return text


def fallback_sql_translate(text: str, target: str) -> str:
    """翻译 SQL 固定数据，优先完整短语，再处理菜单规则。"""
    fixed = SQL_FIXED_I18NS.get(target, {})
    if text in fixed:
        return fixed[text]
    if text in SQL_MENU_CATEGORIES.get(target, {}) or text in SQL_MENU_NOUNS.get(target, {}):
        return i18n_sql_menu(text, target)
    if text.startswith(("首页", "AI助手", "个人信息", "系统", "菜单", "字典", "新增", "删除", "编辑", "修改", "启动", "停止", "执行", "查询", "设置", "刷新", "重置", "维护", "预览", "还原", "分配")):
        translated = i18n_sql_menu(text, target)
        if translated != text:
            return translated
    return fallback_translate(text, target)
PROTECTED_PATTERN = re.compile(
    r"(?s)```.*?```|`[^`]+`|\{\{[^{}]+\}\}|\$\{[^{}]+\}|\{[A-Za-z_][A-Za-z0-9_.-]*\}|%[sdv]|</?[^>]+>|https?://[^\s<>()]+|/(?:api|events|mcp|v[0-9]+)/[A-Za-z0-9_./:{}-]+"
)
MIGRATION_VERSION_PATTERN = re.compile(r"^v\d+\.\d+\.\d+$")
ENTRY_PATTERN = re.compile(
    r"(?:__\s*)?KRATOS[_ ]?ENTRY[_ ]?(\d{4})(?:\s*__)?\s*:?\s*", re.IGNORECASE
)
TOKEN_PATTERN = re.compile(
    r"(?:__\s*)?KRATOS[_ ]?TOKEN[_ ]?(\d{4})(?:\s*__)?", re.IGNORECASE
)
PLACEHOLDER_PATTERN = re.compile(r"\{\{[^{}]+\}\}|\$\{[^{}]+\}|\{[A-Za-z_][A-Za-z0-9_.-]*\}|%[sdv]")


def load_opencc():
    try:
        from opencc import OpenCC
    except ImportError as exc:
        raise SystemExit(
            "生成 zh-TW 需要 OpenCC，请先执行：python3 -m pip install opencc-python-reimplemented"
        ) from exc
    return OpenCC("s2twp")


def convert_value(value: Any, converter) -> Any:
    if isinstance(value, str):
        return converter.convert(value)
    if isinstance(value, list):
        return [convert_value(item, converter) for item in value]
    if isinstance(value, dict):
        return {key: convert_value(item, converter) for key, item in value.items()}
    return value


def protect_text(text: str, index: int) -> tuple[str, dict[str, str]]:
    values: dict[str, str] = {}

    def replace(match: re.Match[str]) -> str:
        token = f"KRATOSTOKEN{len(values):04d}"
        values[token] = match.group(0)
        return token

    return PROTECTED_PATTERN.sub(replace, text), values


def restore_text(text: str, values: dict[str, str]) -> str:
    for token, value in values.items():
        text = text.replace(token, value)
    for match in TOKEN_PATTERN.finditer(text):
        text = text.replace(match.group(0), values.get(match.group(0), ""))
    for value in values.values():
        if value not in text:
            text = f"{text} {value}".strip()
    return text


def has_expected_placeholders(text: str, protected: dict[str, str]) -> bool:
    expected = Counter(
        placeholder
        for value in protected.values()
        for placeholder in PLACEHOLDER_PATTERN.findall(value)
    )
    return Counter(PLACEHOLDER_PATTERN.findall(text)) == expected


def fallback_translate(text: str, target: str) -> str:
    """使用内置术语表生成无网络环境下的可读语言草稿。"""
    protected, values = protect_text(text, 0)
    replacements = FALLBACK_I18NS.get(target, {})
    for source, translated in sorted(replacements.items(), key=lambda item: len(item[0]), reverse=True):
        protected = re.sub(re.escape(source), translated, protected, flags=re.IGNORECASE)
    return restore_text(protected, values)


def request_i18n(text: str, source: str, target: str) -> str:
    endpoint = os.environ.get(
        "I18N_ENDPOINT", "https://translate.googleapis.com/translate_a/single"
    )
    clients = tuple(
        item.strip()
        for item in os.environ.get("I18N_GOOGLE_CLIENTS", ",".join(GOOGLE_CLIENTS)).split(",")
        if item.strip()
    )
    try:
        timeout = max(1.0, float(os.environ.get("I18N_REQUEST_TIMEOUT", DEFAULT_REQUEST_TIMEOUT)))
    except ValueError:
        timeout = DEFAULT_REQUEST_TIMEOUT
    last_error: Exception | None = None
    for client in clients:
        query = urllib.parse.urlencode(
            [("client", client), ("sl", source), ("tl", target), ("dt", "t"), ("q", text)]
        )
        for attempt in range(2):
            try:
                request = urllib.request.Request(
                    f"{endpoint}{'&' if '?' in endpoint else '?'}{query}",
                    headers={"User-Agent": "kratos-admin-i18n/1.0", "Connection": "close"},
                )
                with urllib.request.urlopen(request, timeout=timeout) as response:
                    payload = json.loads(response.read().decode("utf-8"))
                return "".join(part[0] for part in payload[0] if part and part[0])
            except urllib.error.HTTPError as error:
                last_error = error
                if error.code in (403, 429):
                    break
            except Exception as error:  # noqa: BLE001 - network provider failure is retried
                last_error = error
            if attempt < 1:
                time.sleep(0.5)
    raise RuntimeError(f"Google V1 翻译失败（{source}->{target}）：{last_error}")


def i18n_batch(texts: list[str], source: str, target: str, offline: bool = False) -> list[str]:
    if offline:
        return [fallback_translate(text, target) for text in texts]
    results: list[str] = []
    chunk: list[tuple[str, dict[str, str]]] = []
    chunk_size = 0
    provider_available = True

    def flush() -> None:
        nonlocal chunk, chunk_size, provider_available
        if not chunk:
            return
        if not provider_available:
            results.extend(fallback_translate(restore_text(value, protected), target) for value, protected in chunk)
            chunk = []
            chunk_size = 0
            return
        source_text = "\n".join(f"KRATOSENTRY{index:04d}: {value}" for index, (value, _) in enumerate(chunk))
        try:
            translated = request_i18n(source_text, source, target)
        except RuntimeError:
            provider_available = False
            results.extend(fallback_translate(restore_text(value, protected), target) for value, protected in chunk)
            chunk = []
            chunk_size = 0
            return
        matches = list(ENTRY_PATTERN.finditer(translated))
        translated_by_index: dict[int, str] = {}
        for position, match in enumerate(matches):
            end = matches[position + 1].start() if position + 1 < len(matches) else len(translated)
            translated_by_index[int(match.group(1))] = translated[match.end() : end].strip()
        for index, (_, protected) in enumerate(chunk):
            result = translated_by_index.get(index)
            if result is None:
                try:
                    result = request_i18n(chunk[index][0], source, target)
                except RuntimeError:
                    provider_available = False
                    result = fallback_translate(restore_text(chunk[index][0], protected), target)
            result = restore_text(result, protected)
            if not has_expected_placeholders(result, protected) and provider_available:
                try:
                    result = restore_text(request_i18n(chunk[index][0], source, target), protected)
                except RuntimeError:
                    provider_available = False
            if not has_expected_placeholders(result, protected):
                result = (
                    fallback_translate(restore_text(chunk[index][0], protected), target)
                    if not provider_available
                    else restore_text(chunk[index][0], protected)
                )
            results.append(result)
        chunk = []
        chunk_size = 0

    for index, text in enumerate(texts):
        protected, values = protect_text(text, index)
        if chunk and chunk_size + len(protected) > 1200:
            flush()
        chunk.append((protected, values))
        chunk_size += len(protected)
    flush()
    return results


def collect_strings(value: Any, values: list[str]) -> None:
    if isinstance(value, str):
        values.append(value)
    elif isinstance(value, list):
        for item in value:
            collect_strings(item, values)
    elif isinstance(value, dict):
        for item in value.values():
            collect_strings(item, values)


def replace_strings(value: Any, replacement: Callable[[], str]) -> Any:
    if isinstance(value, str):
        return replacement()
    if isinstance(value, list):
        return [replace_strings(item, replacement) for item in value]
    if isinstance(value, dict):
        return {key: replace_strings(item, replacement) for key, item in value.items()}
    return value


def collect_missing_strings(source: Any, target: Any, values: list[str]) -> None:
    """收集目标语言包中尚未填写的源文案。"""
    if isinstance(source, str):
        if not isinstance(target, str) or not target.strip():
            values.append(source)
        return
    if isinstance(source, list):
        target_list = target if isinstance(target, list) else []
        for index, item in enumerate(source):
            collect_missing_strings(item, target_list[index] if index < len(target_list) else None, values)
        return
    if isinstance(source, dict):
        target_dict = target if isinstance(target, dict) else {}
        for key, item in source.items():
            collect_missing_strings(item, target_dict.get(key), values)


def merge_missing_strings(source: Any, target: Any, replacement: Callable[[], str]) -> Any:
    """按源语言包结构补齐缺失文案，并保留已有译文。"""
    if isinstance(source, str):
        return target if isinstance(target, str) and target.strip() else replacement()
    if isinstance(source, list):
        target_list = target if isinstance(target, list) else []
        return [
            merge_missing_strings(item, target_list[index] if index < len(target_list) else None, replacement)
            for index, item in enumerate(source)
        ]
    if isinstance(source, dict):
        target_dict = target if isinstance(target, dict) else {}
        return {key: merge_missing_strings(item, target_dict.get(key), replacement) for key, item in source.items()}
    return target if target is not None else source


def generate_json(
    source: Path,
    locale: str,
    converter,
    machine: bool,
    offline: bool,
    write: bool,
    merge: bool,
) -> None:
    source_data = json.loads(source.read_text(encoding="utf-8"))
    target = source.with_name(f"{locale}.json")
    if merge and target.exists():
        target_data = json.loads(target.read_text(encoding="utf-8"))
        missing_values: list[str] = []
        collect_missing_strings(source_data, target_data, missing_values)
        if locale == "zh-TW":
            translated = [converter.convert(value) for value in missing_values]
        elif missing_values:
            translated = i18n_batch(missing_values, "zh-CN", locale.split("-")[0], offline)
        else:
            translated = []
        iterator = iter(translated)
        target_data = merge_missing_strings(source_data, target_data, lambda: next(iterator))
        if write:
            target.write_text(json.dumps(target_data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return
    if locale == "zh-TW":
        target_data = convert_value(source_data, converter)
    else:
        source_values: list[str] = []
        collect_strings(source_data, source_values)
        translated = (
            i18n_batch(source_values, "zh-CN", locale.split("-")[0], offline)
            if machine or offline
            else source_values
        )
        translated = [value or source_values[index] for index, value in enumerate(translated)]
        iterator = iter(translated)
        target_data = replace_strings(source_data, lambda: next(iterator))
    if write:
        target.write_text(json.dumps(target_data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def parse_sql_values(line: str) -> list[str | None] | None:
    """解析 INSERT 语句中的值，兼容逗号、SQL 双单引号和反斜杠。"""
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


def i18n_record(line: str) -> tuple[int, int, str, str] | None:
    """读取统一翻译表 INSERT，返回目标类型、资源编号、语言和文本。"""
    table_match = re.search(r"INSERT IGNORE INTO `([^`]+)`", line)
    values = parse_sql_values(line)
    if not table_match or table_match.group(1) != "base_i18n" or not values or len(values) < 4:
        return None
    try:
        return int(values[0] or 0), int(values[1] or 0), str(values[2] or ""), str(values[3] or "")
    except (TypeError, ValueError):
        return None


def parse_primary_i18n_sources(default_data: Path) -> dict[tuple[int, int], str]:
    """从主数据 SQL 提取统一翻译表各目标类型对应的简体中文源文。"""
    sources: dict[tuple[int, int], str] = {}
    for line in default_data.read_text(encoding="utf-8").splitlines():
        table_match = re.search(r"INSERT IGNORE INTO `([^`]+)`", line)
        values = parse_sql_values(line)
        if not table_match or not values:
            continue
        table = table_match.group(1)
        try:
            resource_id = int(values[0] or 0)
        except (TypeError, ValueError):
            continue
        if table == "base_config" and len(values) > 5:
            sources[(1, resource_id)] = str(values[5] or "")
            sources[(2, resource_id)] = str(values[2] or "")
        elif table == "base_dict" and len(values) > 2:
            sources[(3, resource_id)] = str(values[2] or "")
        elif table == "base_dict_item" and len(values) > 3:
            sources[(4, resource_id)] = str(values[3] or "")
        elif table == "base_menu" and len(values) > 7:
            try:
                metadata = json.loads(str(values[7] or ""))
            except json.JSONDecodeError:
                continue
            if isinstance(metadata, dict) and isinstance(metadata.get("title"), str):
                sources[(5, resource_id)] = metadata["title"]
        elif table == "base_job" and len(values) > 1:
            sources[(6, resource_id)] = str(values[1] or "")
    return sources


def extract_i18n(line: str, locale: str = "en-US") -> str:
    record = i18n_record(line)
    return record[3] if record and record[2] == locale else ""


def replace_i18n(line: str, locale: str, translated: str) -> str:
    if "INSERT IGNORE INTO" not in line:
        return line.replace("en-US", locale)
    record = i18n_record(line)
    if not record:
        return line
    target_type, target_id, _, _ = record
    escaped = translated.replace("\\", "\\\\").replace("'", "\\'")
    return (
        "INSERT IGNORE INTO `base_i18n` (`target_type`, `target_id`, `locale`, `name`) "
        f"VALUES ({target_type}, {target_id}, '{locale}', '{escaped}');"
    )


def load_existing_i18n(path: Path) -> dict[tuple[int, int], str]:
    """读取已有语言 SQL，供增量生成时保留人工译文。"""
    if not path.exists():
        return {}
    records: dict[tuple[int, int], str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        record = i18n_record(line)
        if record and record[3].strip():
            records[(record[0], record[1])] = record[3]
    return records


def generate_sql(
    locale: str,
    converter,
    machine: bool,
    offline: bool,
    write: bool,
    sql_directory: Path,
    merge: bool,
) -> None:
    source = SQL_DIR / "i18n.en-US.up.sql"
    if not source.is_file():
        print("未配置业务翻译 SQL，跳过 SQL 语言生成")
        return
    target = sql_directory / f"i18n.{locale}.up.sql"
    primary_sources = parse_primary_i18n_sources(SQL_DIR / "default_data.up.sql")
    existing_i18n = load_existing_i18n(target) if merge else {}
    lines = source.read_text(encoding="utf-8").splitlines()
    translated_by_index: dict[int, str] = {}
    pending_indices: list[int] = []
    pending_values: list[str] = []
    for index, line in enumerate(lines):
        record = i18n_record(line)
        if not record:
            continue
        key = (record[0], record[1])
        if key not in primary_sources:
            raise ValueError(f"默认数据缺少翻译源文本: {SQL_DIR / 'default_data.up.sql'} {key}")
        if merge and key in existing_i18n:
            translated_by_index[index] = existing_i18n[key]
            continue
        pending_indices.append(index)
        pending_values.append(primary_sources[key])
    if locale == "zh-TW":
        translated = [converter.convert(value) for value in pending_values]
    elif offline:
        translated = [fallback_sql_translate(value, locale.split("-")[0]) for value in pending_values]
    elif machine:
        translated = i18n_batch(pending_values, "zh-CN", locale.split("-")[0], False)
    else:
        translated = pending_values
    translated_by_index.update(dict(zip(pending_indices, translated)))
    generated = []
    for index, line in enumerate(lines):
        if "INSERT IGNORE INTO" in line and i18n_record(line):
            generated.append(replace_i18n(line, locale, translated_by_index[index]))
        else:
            generated.append(line.replace("en-US", locale))
    if write:
        target.write_text("\n".join(generated) + "\n", encoding="utf-8")


def render_i18n_migration_description(locales: tuple[str, ...]) -> str:
    return (
        "由 `scripts/generate_locale_drafts.py` 生成动态资源翻译草稿。\n\n"
        f"目标语言：{', '.join(locales)}。\n\n"
        "迁移只写入非空固定译文，已有统一表记录不会被覆盖；运行时仅在记录为空时补充机器译文。\n"
    )


def parse_locales(values: list[str] | None) -> tuple[str, ...]:
    """解析逗号分隔的语言列表，并保留命令行传入顺序。"""
    if not values:
        return DEFAULT_TARGET_LOCALES
    locales: list[str] = []
    for value in values:
        for locale in value.split(","):
            locale = locale.strip()
            if not locale:
                continue
            if not LOCALE_CODE_PATTERN.fullmatch(locale):
                raise SystemExit(f"语言代码不是有效的 BCP 47 代码: {locale}")
            if locale == "zh-CN":
                raise SystemExit("不能把默认语言 zh-CN 作为生成目标")
            if locale not in locales:
                locales.append(locale)
    return tuple(locales)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true", help="写入所有新增语言文件")
    parser.add_argument("--machine", action="store_true", help="使用 Google V1 生成指定语言草稿")
    parser.add_argument("--offline", action="store_true", help="使用内置术语表离线生成机器翻译草稿")
    parser.add_argument("--merge", action="store_true", help="增量补齐已有语言包和翻译 SQL，不覆盖已有译文")
    parser.add_argument("--sql-only", action="store_true", help="只生成动态翻译迁移，不改写固定语言包")
    parser.add_argument("--locale", dest="locales", action="append", help="只生成指定语言，使用逗号分隔，可重复传入")
    parser.add_argument("--migration-version", help="将按语言拆分的翻译 SQL 写入指定版本目录，例如 vX.Y.Z")
    args = parser.parse_args()
    locales = parse_locales(args.locales)
    sql_directory = SQL_DIR
    if args.migration_version:
        if not MIGRATION_VERSION_PATTERN.fullmatch(args.migration_version):
            raise SystemExit("迁移版本必须是 vX.Y.Z 格式")
        sql_directory = ROOT / "backend/migration/assets" / args.migration_version / "mysql"
        if args.write:
            sql_directory.mkdir(parents=True, exist_ok=True)
    converter = load_opencc() if "zh-TW" in locales else None
    for locale in locales:
        if locale != "zh-TW" and not args.machine and not args.offline:
            raise SystemExit("生成指定的非繁体语言需要显式传入 --machine 或 --offline")
        if not args.sql_only:
            for source in JSON_SOURCES:
                generate_json(source, locale, converter, args.machine, args.offline, args.write, args.merge)
        generate_sql(locale, converter, args.machine, args.offline, args.write, sql_directory, args.merge)
    if args.migration_version and args.write:
        readme = sql_directory / "README.md"
        if not readme.exists():
            readme.write_text(render_i18n_migration_description(locales), encoding="utf-8")
    action = "已生成" if args.write else "可生成"
    artifact = "迁移数据" if args.sql_only else "语言包和迁移数据"
    print(f"{action} {', '.join(locales)} {artifact}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
