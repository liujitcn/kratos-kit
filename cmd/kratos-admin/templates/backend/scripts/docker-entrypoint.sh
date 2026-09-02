#!/bin/sh
set -eu

static_seed_directory="${KRATOS_STATIC_SEED_DIRECTORY:-/opt/kratos-admin/static}"
data_directory="${KRATOS_DATA_DIRECTORY:-/app/data}"

mkdir -p "$data_directory"
if [ -d "$static_seed_directory" ]; then
  cp -R "$static_seed_directory"/. "$data_directory"/
fi

exec "$@"
