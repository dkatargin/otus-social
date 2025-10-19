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
