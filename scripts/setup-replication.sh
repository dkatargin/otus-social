#!/bin/bash
set -e

# Создать директорию для архивов
mkdir -p /var/lib/postgresql/archives
chown postgres:postgres /var/lib/postgresql/archives
chmod 755 /var/lib/postgresql/archives

# Создать пользователя для репликации
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Создаем пользователя для репликации если он не существует
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'replicator') THEN
            CREATE USER replicator WITH REPLICATION PASSWORD 'replicatorpass';
        END IF;
    END
    \$\$;

    -- Также даем права репликации основному пользователю
    ALTER USER app_user REPLICATION;
EOSQL

echo "Пользователь replicator создан с правами репликации"
echo "Пользователь app_user получил права репликации"
echo "Директория архивов создана: /var/lib/postgresql/archives"