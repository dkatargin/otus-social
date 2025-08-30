#!/bin/bash

# Скрипт для управления репликацией PostgreSQL
# Включает функции для проверки статуса, промоутинга слейвов и переключения

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Функция для логирования
log() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1"
}

error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1"
}

# Проверка доступности контейнера
is_container_running() {
    local container_name=$1
    docker ps --format "table {{.Names}}" | grep -q "^${container_name}$" 2>/dev/null
}

# Безопасное выполнение команды в контейнере
safe_docker_exec() {
    local container_name=$1
    shift

    if is_container_running "$container_name"; then
        docker exec "$container_name" "$@"
    else
        warn "Контейнер $container_name недоступен"
        return 1
    fi
}

# Проверка статуса репликации
check_replication_status() {
    log "Проверка статуса репликации..."

    echo "=== Статус мастера ==="
    safe_docker_exec otus-social-postgres-master-1 psql -U app_user -d app_db -c "
        SELECT
            client_addr as replica_ip,
            state,
            sync_state,
            sync_priority,
            replay_lag
        FROM pg_stat_replication;
    " || warn "Не удалось получить статус мастера"

    echo "=== Статус слейва 1 ==="
    safe_docker_exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -c "
        SELECT
            CASE
                WHEN pg_is_in_recovery() THEN 'REPLICA'
                ELSE 'PRIMARY'
            END as status,
            pg_last_wal_receive_lsn() as received_lsn,
            pg_last_wal_replay_lsn() as replayed_lsn,
            EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds;
    " || warn "Не удалось получить статус слейва 1"

    echo "=== Статус слейва 2 ==="
    safe_docker_exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -c "
        SELECT
            CASE
                WHEN pg_is_in_recovery() THEN 'REPLICA'
                ELSE 'PRIMARY'
            END as status,
            pg_last_wal_receive_lsn() as received_lsn,
            pg_last_wal_replay_lsn() as replayed_lsn,
            EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds;
    " || warn "Не удалось получить статус слейва 2"
}

# Промоутинг слейва до мастера
promote_slave() {
    local slave_name=$1
    if [ -z "$slave_name" ]; then
        error "Необходимо указать имя слейва (postgres-slave-1 или postgres-slave-2)"
        return 1
    fi

    log "Промоутинг $slave_name до мастера..."

    # Сначала проверяем, является ли узел еще репликой
    local container_name
    if [ "$slave_name" = "postgres-slave-1" ]; then
        container_name="otus-social-postgres-slave-1-1"
    elif [ "$slave_name" = "postgres-slave-2" ]; then
        container_name="otus-social-postgres-slave-2-1"
    else
        error "Неизвестное имя слейва: $slave_name"
        return 1
    fi

    # Проверяем статус восстановления
    local recovery_status
    recovery_status=$(safe_docker_exec "$container_name" psql -U app_user -d app_db -t -c "SELECT pg_is_in_recovery();" 2>/dev/null | tr -d ' ' || echo "f")

    if [ "$recovery_status" = "f" ]; then
        log "Узел $slave_name уже является мастером, промоутинг не требуется"
        return 0
    fi

    # Выполняем промоутинг через SQL команду pg_promote()
    if [ "$slave_name" = "postgres-slave-1" ]; then
        safe_docker_exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -c "SELECT pg_promote();"
        log "Промоутинг слейва 1 выполнен"
    elif [ "$slave_name" = "postgres-slave-2" ]; then
        safe_docker_exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -c "SELECT pg_promote();"
        log "Промоутинг слейва 2 выполнен"
    fi

    # Ждем завершения промоутинга
    sleep 10

    log "Проверка статуса после промоутинга..."
    safe_docker_exec "$container_name" psql -U app_user -d app_db -c "
        SELECT CASE WHEN pg_is_in_recovery() THEN 'Still REPLICA' ELSE 'Now PRIMARY' END as status;
    "
}

# Переключение слейва на новый мастер
switch_slave_to_master() {
    local slave_name=$1
    local new_master_host=$2

    if [ -z "$slave_name" ] || [ -z "$new_master_host" ]; then
        error "Использование: switch_slave_to_master <slave_name> <new_master_host>"
        return 1
    fi

    log "Переключение $slave_name на новый мастер $new_master_host..."

    # Останавливаем слейв
    safe_docker_exec otus-social-${slave_name}-1 pg_ctl stop -D /var/lib/postgresql/data -m fast || true

    # Обновляем конфигурацию recovery
    safe_docker_exec otus-social-${slave_name}-1 bash -c "
        echo 'standby_mode = on' > /var/lib/postgresql/data/recovery.conf
        echo 'primary_conninfo = '\''host=$new_master_host port=5432 user=replicator password=replicatorpass'\''' >> /var/lib/postgresql/data/recovery.conf
        echo 'trigger_file = '\''/tmp/promote_${slave_name##*-}'\''' >> /var/lib/postgresql/data/recovery.conf
    "

    # Запускаем слейв
    safe_docker_exec otus-social-${slave_name}-1 pg_ctl start -D /var/lib/postgresql/data

    log "Слейв $slave_name переключен на мастер $new_master_host"
}

# Проверка потерь транзакций
check_transaction_loss() {
    log "Проверка потерь транзакций..."

    echo "=== Сравнение LSN между узлами ==="

    echo "Мастер LSN:"
    master_lsn=$(safe_docker_exec otus-social-postgres-master-1 psql -U app_user -d app_db -t -c "
        SELECT pg_current_wal_lsn();
    " 2>/dev/null | tr -d ' ' | head -1 || echo "N/A")
    echo "LSN: $master_lsn"

    echo "Слейв 1 LSN:"
    if is_container_running "otus-social-postgres-slave-1-1"; then
        slave1_lsn=$(docker exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -t -c "
            SELECT pg_last_wal_replay_lsn();
        " 2>/dev/null | tr -d ' ' | head -1 || echo "N/A")
        echo "LSN: $slave1_lsn"
    else
        slave1_lsn="N/A"
        echo "LSN: N/A (контейнер недоступен)"
    fi

    echo "Слейв 2 LSN:"
    if is_container_running "otus-social-postgres-slave-2-1"; then
        slave2_lsn=$(docker exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -t -c "
            SELECT pg_last_wal_replay_lsn();
        " 2>/dev/null | tr -d ' ' | head -1 || echo "N/A")
        echo "LSN: $slave2_lsn"
    else
        slave2_lsn="N/A"
        echo "LSN: N/A (контейнер недоступен)"
    fi

    echo "=== Подсчет записей в таблице users ==="

    echo "Мастер:"
    if is_container_running "otus-social-postgres-master-1"; then
        master_count=$(docker exec otus-social-postgres-master-1 psql -U app_user -d app_db -t -c "
            SELECT COUNT(*) FROM users;
        " 2>/dev/null | tr -d ' ' | head -1 || echo "0")
        echo "Количество пользователей: $master_count"
    else
        master_count="0"
        echo "Количество пользователей: 0 (контейнер недоступен)"
    fi

    echo "Слейв 1:"
    if is_container_running "otus-social-postgres-slave-1-1"; then
        slave1_count=$(docker exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -t -c "
            SELECT COUNT(*) FROM users;
        " 2>/dev/null | tr -d ' ' | head -1 || echo "0")
        echo "Количество пользователей: $slave1_count"
    else
        slave1_count="0"
        echo "Количество пользователей: 0 (контейнер недоступен)"
    fi

    echo "Слейв 2:"
    if is_container_running "otus-social-postgres-slave-2-1"; then
        slave2_count=$(docker exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -t -c "
            SELECT COUNT(*) FROM users;
        " 2>/dev/null | tr -d ' ' | head -1 || echo "0")
        echo "Количество пользователей: $slave2_count"
    else
        slave2_count="0"
        echo "Количество пользователей: 0 (контейнер недоступен)"
    fi

    echo "=== Анализ потерь транзакций ==="

    # Определяем максимальное количество записей как базовое
    max_count=0
    if [[ "$master_count" =~ ^[0-9]+$ ]] && [ "$master_count" -gt "$max_count" ]; then
        max_count=$master_count
    fi
    if [[ "$slave1_count" =~ ^[0-9]+$ ]] && [ "$slave1_count" -gt "$max_count" ]; then
        max_count=$slave1_count
    fi
    if [[ "$slave2_count" =~ ^[0-9]+$ ]] && [ "$slave2_count" -gt "$max_count" ]; then
        max_count=$slave2_count
    fi

    echo "Максимальное количество записей: $max_count"

    # Вычисляем потери
    total_loss=0

    if [[ "$master_count" =~ ^[0-9]+$ ]]; then
        master_loss=$((max_count - master_count))
        echo "Потери на мастере: $master_loss записей"
        if [ "$master_loss" -gt 0 ]; then
            total_loss=$((total_loss + master_loss))
        fi
    else
        echo "Мастер недоступен - невозможно определить потери"
    fi

    if [[ "$slave1_count" =~ ^[0-9]+$ ]]; then
        slave1_loss=$((max_count - slave1_count))
        echo "Потери на слейве 1: $slave1_loss записей"
        if [ "$slave1_loss" -gt 0 ]; then
            total_loss=$((total_loss + slave1_loss))
        fi
    else
        echo "Слейв 1 недоступен - невозможно определить потери"
    fi

    if [[ "$slave2_count" =~ ^[0-9]+$ ]]; then
        slave2_loss=$((max_count - slave2_count))
        echo "Потери на слейве 2: $slave2_loss записей"
        if [ "$slave2_loss" -gt 0 ]; then
            total_loss=$((total_loss + slave2_loss))
        fi
    else
        echo "Слейв 2 недоступен - невозможно определить потери"
    fi

    echo "=== Итоговая оценка потерь ==="
    echo "Общее количество потерянных записей: $total_loss"

    if [ "$total_loss" -eq 0 ]; then
        echo "✅ Потери транзакций отсутствуют - данные синхронизированы"
    elif [ "$total_loss" -lt 10 ]; then
        echo "⚠️  Минимальные потери транзакций ($total_loss записей)"
    else
        echo "❌ Значительные потери транзакций ($total_loss записей)"
    fi
}

# Опредеkение самого свежего слейва
find_freshest_slave() {
    log "Определение самого свежего слейва..."

    # Проверяем доступность слейвов
    slave1_available=false
    slave2_available=false

    if is_container_running "otus-social-postgres-slave-1-1"; then
        slave1_available=true
        slave1_lsn=$(safe_docker_exec otus-social-postgres-slave-1-1 psql -U app_user -d app_db -t -c "
            SELECT pg_last_wal_replay_lsn();
        " 2>/dev/null | tr -d ' ' || echo "0/0")
    else
        slave1_lsn="0/0"
    fi

    if is_container_running "otus-social-postgres-slave-2-1"; then
        slave2_available=true
        slave2_lsn=$(safe_docker_exec otus-social-postgres-slave-2-1 psql -U app_user -d app_db -t -c "
            SELECT pg_last_wal_replay_lsn();
        " 2>/dev/null | tr -d ' ' || echo "0/0")
    else
        slave2_lsn="0/0"
    fi

    echo "Слейв 1 доступен: $slave1_available, LSN: $slave1_lsn"
    echo "Слейв 2 доступен: $slave2_available, LSN: $slave2_lsn"

    # Выбираем доступный слейв или самый свежий
    if [ "$slave1_available" = true ] && [ "$slave2_available" = false ]; then
        echo "Только слейв 1 доступен"
        echo "postgres-slave-1"
    elif [ "$slave2_available" = true ] && [ "$slave1_available" = false ]; then
        echo "Только слейв 2 доступен"
        echo "postgres-slave-2"
    elif [ "$slave1_available" = true ] && [ "$slave2_available" = true ]; then
        # Оба доступны, выбираем самый свежий
        if [[ "$slave1_lsn" > "$slave2_lsn" ]]; then
            echo "Самый свежий слейв: postgres-slave-1"
            echo "postgres-slave-1"
        else
            echo "Самый свежий слейв: postgres-slave-2"
            echo "postgres-slave-2"
        fi
    else
        error "Нет доступных слейвов для промоутинга!"
        return 1
    fi
}

# Главная функция
case "$1" in
    "status")
        check_replication_status
        ;;
    "promote")
        promote_slave "$2"
        ;;
    "switch")
        switch_slave_to_master "$2" "$3"
        ;;
    "check-loss")
        check_transaction_loss
        ;;
    "find-freshest")
        find_freshest_slave
        ;;
    "full-failover")
        log "Выполнение полного сценария failover..."

        log "1. Проверка статуса до failover"
        check_replication_status

        log "2. Определение самого свежего слейва"
        freshest=$(find_freshest_slave | tail -1)

        if [ -z "$freshest" ]; then
            error "Не удалось определить доступный слейв для промоутинга"
            exit 1
        fi

        log "3. Промоутинг самого свежего слейва ($freshest)"
        promote_slave "$freshest"

        log "4. Переключение другого слейва на новый мастер (если доступен)"
        if [ "$freshest" = "postgres-slave-1" ]; then
            if is_container_running "otus-social-postgres-slave-2-1"; then
                switch_slave_to_master "postgres-slave-2" "postgres-slave-1"
            else
                warn "Слейв 2 недоступен для переключения"
            fi
        else
            if is_container_running "otus-social-postgres-slave-1-1"; then
                switch_slave_to_master "postgres-slave-1" "postgres-slave-2"
            else
                warn "Слейв 1 недоступен для переключения"
            fi
        fi

        log "5. Проверка потерь транзакций"
        check_transaction_loss

        log "6. Финальная проверка статуса"
        check_replication_status
        ;;
    *)
        echo "Использование: $0 {status|promote|switch|check-loss|find-freshest|full-failover}"
        echo ""
        echo "Команды:"
        echo "  status              - Проверить статус репликации"
        echo "  promote <slave>     - Промоутить слейв до мастера"
        echo "  switch <slave> <master> - Переключить слейв на новый мастер"
        echo "  check-loss          - Проверить потери транзакций"
        echo "  find-freshest       - Найти самый свежий слейв"
        echo "  full-failover       - Выполнить полный сценарий failover"
        echo ""
        echo "Примеры:"
        echo "  $0 status"
        echo "  $0 promote postgres-slave-1"
        echo "  $0 switch postgres-slave-2 postgres-slave-1"
        echo "  $0 full-failover"
        exit 1
        ;;
esac
