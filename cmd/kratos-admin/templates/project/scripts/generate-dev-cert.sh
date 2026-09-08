#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cert_dir="${repo_root}/certs"
lan_ip="${1:-}"

if [[ -z "${lan_ip}" ]] && command -v ipconfig >/dev/null 2>&1; then
  for interface in en0 en1; do
    lan_ip="$(ipconfig getifaddr "${interface}" 2>/dev/null || true)"
    [[ -n "${lan_ip}" ]] && break
  done
fi

if [[ -z "${lan_ip}" ]] && command -v hostname >/dev/null 2>&1; then
  lan_ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
fi

if [[ -z "${lan_ip}" ]]; then
  printf '未检测到局域网 IP，请手动传入，例如：%s 192.168.1.100\n' "$0" >&2
  exit 1
fi

command -v openssl >/dev/null 2>&1 || {
  printf '未找到 openssl，请先安装 OpenSSL。\n' >&2
  exit 1
}

mkdir -p "${cert_dir}"
openssl req \
  -x509 \
  -newkey rsa:2048 \
  -nodes \
  -sha256 \
  -days 825 \
  -keyout "${cert_dir}/dev-key.pem" \
  -out "${cert_dir}/dev-cert.pem" \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1,IP:${lan_ip}"

printf 'HTTPS 共享证书已生成：%s\n' "${cert_dir}"
printf 'Admin 地址：https://%s:8848\n' "${lan_ip}"
printf 'Taro 地址：https://%s:5002\n' "${lan_ip}"
printf 'uni-app 地址：https://%s:5004\n' "${lan_ip}"
