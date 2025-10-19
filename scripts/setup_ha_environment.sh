#!/bin/bash

# Скрипт для настройки и запуска HA окружения

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo "=========================================="
echo "Настройка HA окружения"
echo "=========================================="

# Проверка наличия docker-compose
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}docker-compose не найден. Установите его для продолжения.${NC}"
    exit 1
fi

# Шаг 1: Подготовка конфигурации
echo ""
echo -e "${YELLOW}Шаг 1: Подготовка конфигурации${NC}"

if [ -f "app.yaml" ]; then
    echo "Создаем резервную копию текущей конфигурации..."
    cp app.yaml app.yaml.backup.$(date +%s)
fi

if [ -f "app.yaml.haproxy" ]; then
    echo "Копируем конфигурацию с HAProxy..."
    cp app.yaml.haproxy app.yaml
    echo -e "${GREEN}✓${NC} Конфигурация обновлена"
else
    echo -e "${RED}✗${NC} Файл app.yaml.haproxy не найден!"
    exit 1
fi

# Шаг 2: Остановка старых контейнеров
echo ""
echo -e "${YELLOW}Шаг 2: Остановка существующих контейнеров${NC}"
docker-compose down

# Шаг 3: Сборка образов
echo ""
echo -e "${YELLOW}Шаг 3: Сборка Docker образов${NC}"
docker-compose build

# Шаг 4: Запуск сервисов
echo ""
echo -e "${YELLOW}Шаг 4: Запуск сервисов${NC}"
docker-compose up -d

# Шаг 5: Ожидание готовности
echo ""
echo -e "${YELLOW}Шаг 5: Ожидание готовности сервисов${NC}"
echo "Это может занять 1-2 минуты..."

sleep 10

max_attempts=30
attempt=0

while [ $attempt -lt $max_attempts ]; do
    if docker-compose ps | grep -q "healthy"; then
        healthy_count=$(docker-compose ps | grep "healthy" | wc -l)
        total_count=$(docker-compose ps | grep "Up" | wc -l)
        echo "Готово сервисов: $healthy_count/$total_count"
    fi

    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Nginx отвечает"
        break
    fi

    attempt=$((attempt + 1))
    sleep 5
    echo -n "."
done

echo ""

# Шаг 6: Проверка статуса
echo ""
echo -e "${YELLOW}Шаг 6: Проверка статуса сервисов${NC}"
docker-compose ps

# Проверка доступности
echo ""
echo "Проверка доступности endpoints..."

if curl -s http://localhost:8080/health > /dev/null; then
    echo -e "${GREEN}✓${NC} API доступен: http://localhost:8080"
else
    echo -e "${RED}✗${NC} API недоступен"
fi

if curl -s http://localhost:8404/stats > /dev/null; then
    echo -e "${GREEN}✓${NC} HAProxy Stats: http://localhost:8404/stats"
else
    echo -e "${RED}✗${NC} HAProxy Stats недоступен"
fi

if curl -s http://localhost:8082/nginx_status > /dev/null; then
    echo -e "${GREEN}✓${NC} Nginx Status: http://localhost:8082/nginx_status"
else
    echo -e "${RED}✗${NC} Nginx Status недоступен"
fi

# Итоги
echo ""
echo "=========================================="
echo "Настройка завершена!"
echo "=========================================="
echo ""
echo "Доступные endpoints:"
echo "  API (через Nginx):     http://localhost:8080"
echo "  HAProxy Stats:         http://localhost:8404/stats"
echo "  Nginx Status:          http://localhost:8082/nginx_status"
echo "  RabbitMQ Management:   http://localhost:15672"
echo ""
echo "Для просмотра логов:"
echo "  docker-compose logs -f [service_name]"
echo ""
echo "Для запуска тестов отказоустойчивости:"
echo "  ./scripts/test_ha_availability.sh"
echo ""
echo "Для запуска нагрузочного тестирования:"
echo "  ./scripts/benchmark_ha.sh"

