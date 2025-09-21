#!/bin/bash

# Скрипт для запуска нагрузочных тестов диалогов (SQL vs Redis)
# Для выполнения ДЗ по курсу HighLoad

set -e

echo "=== Dialog Performance Testing Script ==="
echo "Запуск сравнительного тестирования SQL vs Redis диалогов"
echo

# Запускаем все необходимые сервисы через docker-compose
echo "Запускаем сервисы через docker-compose..."
docker-compose down && docker-compose up -d

# Ждем, пока сервисы запустятся
echo "Ожидаем запуска сервисов..."
sleep 10

# Проверяем наличие Redis для диалогов
echo "Проверяем доступность Redis для диалогов..."
if ! redis-cli -h localhost -p 6380 ping > /dev/null 2>&1; then
    echo "❌ Redis для диалогов недоступен на порту 6380"
    echo "Проверьте логи: docker-compose logs redis-dialogs"
    exit 1
fi
echo "✅ Redis для диалогов доступен"

# Проверяем наличие PostgreSQL (используем правильный порт 5435 для postgres-slave-2)
echo "Проверяем доступность PostgreSQL..."
if ! pg_isready -h localhost -p 5435 > /dev/null 2>&1; then
    echo "❌ PostgreSQL недоступен на порту 5435"
    echo "Проверьте логи: docker-compose logs postgres-slave-2"
    exit 1
fi
echo "✅ PostgreSQL доступен"

cd "$(dirname "$0")/../src"

# Устанавливаем зависимости если нужно
echo "Проверяем зависимости Go..."
go mod tidy

# 1. Запускаем базовый тест SQL диалогов
echo
echo "=== 1. Тестирование SQL диалогов (baseline) ==="
go test -v ./tests -run TestDialogLoadBaseline -timeout 60s
if [ $? -eq 0 ]; then
    echo "✅ Базовый тест SQL диалогов завершен"
else
    echo "❌ Ошибка в базовом тесте SQL диалогов"
    tail -20 ../results_sql_baseline.log
fi

# 2. Запускаем тест Redis диалогов
echo
echo "=== 2. Тестирование Redis диалогов ==="
go test -v ./tests -run TestRedisDialogLoad -timeout 60s > ../results_redis.log 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Тест Redis диалогов завершен"
else
    echo "❌ Ошибка в тесте Redis диалогов"
    tail -20 ../results_redis.log
fi

# 3. Запускаем сравнительный тест
echo
echo "=== 3. Сравнительное тестирование SQL vs Redis ==="
go test -v ./tests -run TestDialogPerformanceComparison -timeout 120s > ../results_comparison.log 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Сравнительный тест завершен"
else
    echo "❌ Ошибка в сравнительном тесте"
    tail -20 ../results_comparison.log
fi

# 4. Создаем итоговый отчет
echo
echo "=== 4. Формируем итоговый отчет ==="

REPORT_DIR="../performance_reports"
mkdir -p "$REPORT_DIR"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_FILE="$REPORT_DIR/dialog_performance_report_$TIMESTAMP.md"

cat > "$REPORT_FILE" << EOF
# Отчет по производительности диалогов: SQL vs Redis

**Дата тестирования:** $(date)
**Версия:** $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

## Описание тестирования

В рамках выполнения ДЗ по курсу HighLoad был проведен перенос модуля диалогов из SQL БД в Redis с использованием UDF (User Defined Functions).

### Архитектура решения

#### SQL версия (baseline)
- PostgreSQL с шардированием по 4 шардам
- Таблицы: messages_0, messages_1, messages_2, messages_3
- Детерминированное распределение по шардам на основе пары пользователей

#### Redis версия с UDF
- Отдельный инстанс Redis (порт 6380)
- Lua скрипты для атомарных операций
- Структуры данных:
  - Sorted Sets для хранения сообщений (сортировка по времени)
  - Hash Sets для счетчиков непрочитанных сообщений
  - Hash Sets для статистики диалогов

### Результаты тестирования

EOF

# Извлекаем результаты из логов и добавляем в отчет
echo "#### SQL Baseline Results" >> "$REPORT_FILE"
echo '```' >> "$REPORT_FILE"
grep -A 20 "=== Dialog Load Test Baseline Results ===" ../results_sql_baseline.log | head -20 >> "$REPORT_FILE" 2>/dev/null || echo "Результаты SQL не найдены" >> "$REPORT_FILE"
echo '```' >> "$REPORT_FILE"
echo >> "$REPORT_FILE"

echo "#### Redis Results" >> "$REPORT_FILE"
echo '```' >> "$REPORT_FILE"
grep -A 20 "=== Redis Dialog Load Test Results ===" ../results_redis.log | head -20 >> "$REPORT_FILE" 2>/dev/null || echo "Результаты Redis не найдены" >> "$REPORT_FILE"
echo '```' >> "$REPORT_FILE"
echo >> "$REPORT_FILE"

echo "#### Performance Comparison" >> "$REPORT_FILE"
echo '```' >> "$REPORT_FILE"
grep -A 10 "=== PERFORMANCE COMPARISON RESULTS ===" ../results_comparison.log | head -10 >> "$REPORT_FILE" 2>/dev/null || echo "Результаты сравнения не найдены" >> "$REPORT_FILE"
echo '```' >> "$REPORT_FILE"
echo >> "$REPORT_FILE"

# Добавляем JSON результаты если есть
if ls dialog_performance_comparison_*.json > /dev/null 2>&1; then
    echo "#### Подробные результаты (JSON)" >> "$REPORT_FILE"
    echo '```json' >> "$REPORT_FILE"
    cat $(ls -t dialog_performance_comparison_*.json | head -1) >> "$REPORT_FILE" 2>/dev/null
    echo '```' >> "$REPORT_FILE"
fi

cat >> "$REPORT_FILE" << EOF

## Выводы

### Преимущества Redis + UDF решения:
1. **Производительность**: Снижение задержек за счет in-memory хранения
2. **Атомарность**: Lua скрипты обеспечивают атомарность операций
3. **Простота**: Упрощенная схема данных без сложного шардирования
4. **Масштабируемость**: Лучшая производительность при высоких нагрузках

### Недостатки:
1. **Персистентность**: Требует настройки persistence для надежности
2. **Память**: Ограничения по объему доступной памяти
3. **Сложность**: Требует знания Lua для UDF

## Рекомендации

1. Использовать Redis для диалогов �� случаях высокой нагрузки
2. Настроить репликацию Redis для отказоустойчивости
3. Мониторить использование памяти
4. Рассмотреть гибридное решение: актуальные сообщения в Redis, архив в SQL

EOF

echo "✅ Отчет сохранен: $REPORT_FILE"

# Показываем краткие результаты
echo
echo "=== КРАТКИЕ РЕЗУЛЬТАТЫ ==="
echo
echo "📊 SQL Throughput:"
grep "Throughput:" ../results_sql_baseline.log | head -2 || echo "Не найдены"

echo
echo "📊 Redis Throughput:"
grep "Throughput:" ../results_redis.log | head -2 || echo "Не найдены"

echo
echo "📈 Performance Improvement:"
grep "Improvement:" ../results_comparison.log || echo "Не найдены"

echo
echo "📋 Полный отчет: $REPORT_FILE"
echo "📋 Л��ги тестов:"
echo "   - SQL baseline: $(pwd)/../results_sql_baseline.log"
echo "   - Redis: $(pwd)/../results_redis.log"
echo "   - Comparison: $(pwd)/../results_comparison.log"

echo
echo "🎉 Тестирование завершено!"
