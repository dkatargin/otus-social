#!/bin/bash

set -e

# Устанавливаем секреты
echo "${POSTGRES_MASTER_HOST}:5432:${POSTGRES_DB}:${POSTGRES_USER}:${POSTGRES_PASSWORD}" > /var/lib/postgresql/.pgpass
chmod 600 /var/lib/postgresql/.pgpass

# Включаем режим репликации через pg_basebackup
su -c "mkdir -p /var/lib/postgresql/data && chown postgres:postgres /var/lib/postgresql/data && chmod 700 /var/lib/postgresql/data && pg_basebackup -h localhost -D /var/lib/postgresql/data -U app_user -d 'dbname=app_db host=postgres-master' -R -X stream" postgres