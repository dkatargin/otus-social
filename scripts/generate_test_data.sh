#!/bin/bash

# Скрипт для генерации тестовых данных для социальной сети
# Создает дружбы между пользователями 1-5 и генерирует 100,000 постов

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SRC_DIR="$PROJECT_ROOT/src"

echo "🚀 Генератор тестовых данных для ленты постов"
echo "=============================================="

# Проверяем, что сервер запущен
echo "📡 Проверяем доступность API сервера..."
if ! curl -s http://localhost:8080/api/v1/feed > /dev/null 2>&1; then
    echo "❌ Сервер не доступен на http://localhost:8080"
    echo "   Запустите сервер командой: cd src && ./social-network"
    exit 1
fi

echo "✅ Сервер доступен"

# Проверяем, что Redis запущен
echo "📦 Проверяем доступность Redis..."
if command -v redis-cli >/dev/null 2>&1 && redis-cli ping >/dev/null 2>&1; then
    echo "✅ Redis доступен (локально)"
elif docker-compose exec -T redis redis-cli ping >/dev/null 2>&1; then
    echo "✅ Redis доступен (Docker)"
elif curl -s http://localhost:6379 >/dev/null 2>&1; then
    echo "✅ Redis доступен (порт 6379)"
else
    echo "❌ Redis не доступен"
    echo "   Проверьте статус: docker-compose ps redis"
    echo "   Или запустите: docker-compose up -d redis"
    echo "   Локально: redis-server"
    exit 1
fi

echo "✅ Redis доступен"

# Переходим в директорию с исходниками
cd "$SRC_DIR"

# Компилируем генератор
echo "🔨 Компилируем генератор данных..."
go build -o generate_test_data ./cmd/generate_test_data/

if [ ! -f "generate_test_data" ]; then
    echo "❌ Ошибка компиляции генератора"
    exit 1
fi

echo "✅ Генератор скомпилирован"

# Запускаем генерацию
echo ""
echo "🎯 Запускаем генерацию тестовых данных..."
echo "   - Дружбы между пользователями 1-5"
echo "   - 100,000 постов для пользователей 1-10"
echo ""

# Засекаем время
start_time=$(date +%s)

./generate_test_data

# Вычисляем время выполнения
end_time=$(date +%s)
duration=$((end_time - start_time))

echo ""
echo "🎉 Генерация завершена за $duration секунд!"
echo ""

# Показываем статистику
echo "📊 Статистика созданных данных:"
echo "   - Дружбы: 10 связей между пользователями 1-5"
echo "   - Посты: 100,000 постов от пользователей 1-10"
echo ""

# Проверяем статистику очереди
echo "⚙️ Статистика очереди обновления лент:"
curl -s http://localhost:8080/api/v1/admin/queue/stats | jq . 2>/dev/null || curl -s http://localhost:8080/api/v1/admin/queue/stats

echo ""
echo "💡 Полезные команды:"
echo "   - Просмотр ленты пользователя 1: curl 'http://localhost:8080/api/v1/feed?limit=10' -H 'X-User-ID: 1'"
echo "   - Статистика очереди: curl http://localhost:8080/api/v1/admin/queue/stats"
echo "   - Перестройка кеша: curl -X POST http://localhost:8080/api/v1/admin/feed/rebuild-all"

# Очистка
rm -f generate_test_data

echo ""
echo "✨ Готово! Теперь можно тестировать ленту постов."
