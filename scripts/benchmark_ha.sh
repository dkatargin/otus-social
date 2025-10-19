#!/bin/bash

# Скрипт для бенчмарка производительности с HAProxy и Nginx

set -e

BASE_URL="http://localhost:8080"
DURATION=60
CONNECTIONS=50
THREADS=4

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "Benchmark с HAProxy и Nginx"
echo "=========================================="

# Проверка наличия wrk
if ! command -v wrk &> /dev/null; then
    echo "wrk не установлен. Установите его для запуска бенчмарков:"
    echo "  macOS: brew install wrk"
    echo "  Linux: apt-get install wrk / yum install wrk"
    exit 1
fi

# Функция для запуска wrk
run_benchmark() {
    local url=$1
    local name=$2

    echo ""
    echo -e "${YELLOW}=== $name ===${NC}"
    echo "URL: $url"
    echo "Продолжительность: ${DURATION}s"
    echo "Соединения: $CONNECTIONS"
    echo "Потоки: $THREADS"
    echo ""

    wrk -t$THREADS -c$CONNECTIONS -d${DURATION}s "$url"
}

# 1. Тест health endpoint
run_benchmark "$BASE_URL/health" "Health Endpoint"

# 2. Тест с отказом одного backend
echo ""
echo -e "${YELLOW}Останавливаем backend-1 для теста отказоустойчивости...${NC}"
docker-compose stop backend-1
sleep 5

run_benchmark "$BASE_URL/health" "Health Endpoint (backend-1 остановлен)"

echo ""
echo -e "${YELLOW}Восстанавливаем backend-1...${NC}"
docker-compose start backend-1
sleep 30

run_benchmark "$BASE_URL/health" "Health Endpoint (все backend восстановлены)"

echo ""
echo "=========================================="
echo "Benchmark завершен"
echo "=========================================="
#!/bin/bash

# Скрипт для тестирования отказоустойчивости и доступности
# с HAProxy и Nginx балансировкой

set -e

BASE_URL="http://localhost:8080"
HAPROXY_STATS="http://localhost:8404/stats"
NGINX_STATUS="http://localhost:8082/nginx_status"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Тест отказоустойчивости и доступности"
echo "=========================================="

# Функция для проверки доступности сервиса
check_service() {
    local url=$1
    local name=$2

    if curl -s -o /dev/null -w "%{http_code}" "$url" | grep -q "200"; then
        echo -e "${GREEN}✓${NC} $name доступен"
        return 0
    else
        echo -e "${RED}✗${NC} $name недоступен"
        return 1
    fi
}

# Функция для отправки запросов и подсчета успешных
load_test() {
    local url=$1
    local requests=$2
    local name=$3

    echo ""
    echo "Отправка $requests запросов к $name..."

    success=0
    failed=0
    total_time=0

    for i in $(seq 1 $requests); do
        start_time=$(date +%s%N)
        http_code=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
        end_time=$(date +%s%N)

        response_time=$(( (end_time - start_time) / 1000000 ))
        total_time=$((total_time + response_time))

        if [ "$http_code" = "200" ]; then
            success=$((success + 1))
        else
            failed=$((failed + 1))
        fi

        if [ $((i % 10)) -eq 0 ]; then
            echo -n "."
        fi
    done

    echo ""
    avg_time=$((total_time / requests))
    success_rate=$(awk "BEGIN {printf \"%.2f\", ($success / $requests) * 100}")

    echo "Результаты:"
    echo "  Успешных запросов: $success"
    echo "  Неудачных запросов: $failed"
    echo "  Процент успеха: $success_rate%"
    echo "  Средняя задержка: ${avg_time}ms"

    return $success
}

# Шаг 1: Проверка базовой доступности
echo ""
echo "=== Шаг 1: Проверка базовой доступности ==="
check_service "$BASE_URL/health" "Nginx"
check_service "$HAPROXY_STATS" "HAProxy Stats"

# Шаг 2: Базовый нагрузочный тест
echo ""
echo "=== Шаг 2: Базовый нагрузочный тест (все сервисы работают) ==="
load_test "$BASE_URL/health" 50 "Health endpoint"

# Шаг 3: Симуляция отказа одного backend инстанса
echo ""
echo "=== Шаг 3: Остановка одного backend инстанса ==="
echo -e "${YELLOW}Останавливаем backend-1...${NC}"
docker-compose stop backend-1

sleep 5

echo "Проверка доступности после отказа backend-1..."
load_test "$BASE_URL/health" 50 "Health endpoint"

# Шаг 4: Симуляция отказа второго backend инстанса
echo ""
echo "=== Шаг 4: Остановка второго backend инстанса ==="
echo -e "${YELLOW}Останавливаем backend-2...${NC}"
docker-compose stop backend-2

sleep 5

echo "Проверка доступности после отказа backend-2..."
load_test "$BASE_URL/health" 50 "Health endpoint"

# Шаг 5: Восстановление инстансов
echo ""
echo "=== Шаг 5: Восстановление backend инстансов ==="
echo -e "${YELLOW}Запускаем backend-1 и backend-2...${NC}"
docker-compose start backend-1
docker-compose start backend-2

echo "Ожидание готовности сервисов (30 секунд)..."
sleep 30

echo "Проверка доступности после восстановления..."
load_test "$BASE_URL/health" 50 "Health endpoint"

# Шаг 6: Симуляция отказа одной PostgreSQL реплики
echo ""
echo "=== Шаг 6: Остановка одной PostgreSQL реплики ==="
echo -e "${YELLOW}Останавливаем postgres-slave-1...${NC}"
docker-compose stop postgres-slave-1

sleep 5

echo "Проверка доступности после отказа реплики..."
load_test "$BASE_URL/health" 50 "Health endpoint"

# Шаг 7: Восстановление реплики
echo ""
echo "=== Шаг 7: Восстановление PostgreSQL реплики ==="
echo -e "${YELLOW}Запускаем postgres-slave-1...${NC}"
docker-compose start postgres-slave-1

echo "Ожидание готовности реплики (40 секунд)..."
sleep 40

echo "Проверка доступности после восстановления реплики..."
load_test "$BASE_URL/health" 50 "Health endpoint"

# Итоги
echo ""
echo "=========================================="
echo "Тестирование завершено"
echo "=========================================="
echo ""
echo "Проверьте статистику:"
echo "  HAProxy: $HAPROXY_STATS"
echo "  Nginx: $NGINX_STATUS"

