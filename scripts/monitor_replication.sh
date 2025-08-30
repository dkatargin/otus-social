#!/bin/bash

# Скрипт для мониторинга репликации PostgreSQL в реальном времени

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Функция для очистки экрана
clear_screen() {
    clear
    echo -e "${CYAN}=== Мониторинг репликации PostgreSQL ===${NC}"
    echo -e "${CYAN}Время: $(date)${NC}"
    echo
}

# Мониторинг репликации
monitor_replication() {
    while true; do
        clear_screen

        echo -e "${GREEN}=== Статус мастера ===${NC}"
        docker exec otus-social-postgres-master-1 psql -U app_user -d app_db -c "
            SELECT
                client_addr as replica_ip,
                application_name,
                state,
                sync_state,
                sync_priority,
                pg_wal_lsn_diff(pg_current_wal_lsn(), sent_lsn) as send_lag_bytes,
                pg_wal_lsn_diff(sent_lsn, flush_lsn) as flush_lag_bytes,
                pg_wal_lsn_diff(flush_lsn, replay_lsn) as replay_lag_bytes,
                EXTRACT(EPOCH FROM (now() - backend_start)) as connection_seconds
            FROM pg_stat_replication;
        " 2>/dev/null || echo -e "${RED}Мастер недоступен${NC}"

        echo
        echo -e "${BLUE}=== Статус слейвов ===${NC}"

        echo -e "${YELLOW}Слейв 1:${NC}"
        docker exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -c "
            SELECT
                CASE
                    WHEN pg_is_in_recovery() THEN 'REPLICA'
                    ELSE 'PRIMARY'
                END as status,
                pg_last_wal_receive_lsn() as received_lsn,
                pg_last_wal_replay_lsn() as replayed_lsn,
                pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()) as replay_lag_bytes,
                EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds;
        " 2>/dev/null || echo -e "${RED}Слейв 1 недоступен${NC}"

        echo -e "${YELLOW}Слейв 2:${NC}"
        docker exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -c "
            SELECT
                CASE
                    WHEN pg_is_in_recovery() THEN 'REPLICA'
                    ELSE 'PRIMARY'
                END as status,
                pg_last_wal_receive_lsn() as received_lsn,
                pg_last_wal_replay_lsn() as replayed_lsn,
                pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()) as replay_lag_bytes,
                EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds;
        " 2>/dev/null || echo -e "${RED}Слейв 2 недоступен${NC}"

        echo
        echo -e "${PURPLE}=== Статистика транзакций ===${NC}"

        echo "Количество пользователей на каждом узле:"
        echo -n "Мастер: "
        docker exec otus-social-postgres-master-1 psql -U app_user -d app_db -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ' || echo "N/A"

        echo -n "Слейв 1: "
        docker exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ' || echo "N/A"

        echo -n "Слейв 2: "
        docker exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ' || echo "N/A"

        echo
        echo "Транзакции записи за последние 5 минут:"
        docker exec otus-social-postgres-master-1 psql -U app_user -d app_db -c "
            SELECT
                COUNT(*) as write_transactions,
                COUNT(DISTINCT test_session) as test_sessions
            FROM write_transactions
            WHERE timestamp > NOW() - INTERVAL '5 minutes';
        " 2>/dev/null || echo "Таблица write_transactions не найдена"

        echo
        echo -e "${CYAN}Нажмите Ctrl+C для выхода${NC}"
        echo -e "${CYAN}Обновление каждые 3 секунды...${NC}"

        sleep 3
    done
}

# Функция для проверки подключения к контейнерам
check_containers() {
    echo "Проверка доступности контейнеров..."

    containers=("otus-social-postgres-master-1" "otus-social-postgres-slave-1-1" "otus-social-postgres-slave-2-1")

    for container in "${containers[@]}"; do
        if docker ps --format "table {{.Names}}" | grep -q "$container"; then
            echo -e "${GREEN}✓ $container запущен${NC}"
        else
            echo -e "${RED}✗ $container не найден или не запущен${NC}"
        fi
    done
    echo
}

# Главная функция
case "$1" in
    "monitor"|"")
        check_containers
        monitor_replication
        ;;
    "check")
        check_containers
        ;;
    *)
        echo "Использование: $0 [monitor|check]"
        echo ""
        echo "Команды:"
        echo "  monitor (по умолчанию) - Запустить мониторинг в реальном времени"
        echo "  check                  - Проверить состояние контейнеров"
        exit 1
        ;;
esac
