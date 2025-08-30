#!/bin/bash

# Скрипт для нагрузочного тестирования API на чтение
# Использует wrk для создания нагрузки на endpoints /user/get/{id} и /user/search

BASE_URL="http://localhost:8080/api/v1"
DURATION="60s"
THREADS=4
CONNECTIONS=100

echo "=== Нагрузочное тестирование API на чтение ==="
echo "URL: $BASE_URL"
echo "Длительность: $DURATION"
echo "Потоки: $THREADS"
echo "Соединения: $CONNECTIONS"
echo

# Проверяем наличие wrk
if ! command -v wrk &> /dev/null; then
    echo "wrk не найден. Установите wrk для нагрузочного тестирования:"
    echo "brew install wrk  # на macOS"
    echo "sudo apt-get install wrk  # на Ubuntu"
    exit 1
fi

echo "=== Тест 1: /user/get/{id} ==="
echo "Тестируем получение пользователя по ID..."

# Создаем Lua скрипт для случайных ID
cat > /tmp/user_get_test.lua << 'EOF'
-- Lua скрипт для тестирования /user/get/{id}
math.randomseed(os.time())

request = function()
    local user_id = math.random(1, 1000000)  -- Случайный ID от 1 до 1M
    local path = "/api/v1/user/get/" .. user_id
    return wrk.format("GET", path)
end
EOF

wrk -t$THREADS -c$CONNECTIONS -d$DURATION -s /tmp/user_get_test.lua $BASE_URL > user_get_results.txt
echo "Результаты сохранены в user_get_results.txt"
cat user_get_results.txt

echo
echo "=== Тест 2: /user/search ==="
echo "Тестируем поиск пользователей..."

# Создаем Lua скрипт для поиска
cat > /tmp/user_search_test.lua << 'EOF'
-- Lua скрипт для тестирования /user/search
math.randomseed(os.time())

-- Популярные имена для тестирования (латиница для корректной работы URL)
local first_names = {
    "Alexander", "Dmitry", "Maxim", "Sergey", "Andrew",
    "Alexey", "Artem", "Ilya", "Kirill", "Michael",
    "Nikita", "Matthew", "Roman", "Egor", "Arseniy"
}

local last_names = {
    "Ivanov", "Petrov", "Sidorov", "Smirnov", "Kuznetsov",
    "Popov", "Vasiliev", "Sokolov", "Mikhailov", "Novikov",
    "Fedorov", "Morozov", "Volkov", "Alekseev", "Lebedev"
}

function request()
    local use_first_name = math.random() > 0.5
    local use_last_name = math.random() > 0.3

    local params = {}

    if use_first_name then
        local first_name = first_names[math.random(#first_names)]
        table.insert(params, "first_name=" .. first_name)
    end

    if use_last_name then
        local last_name = last_names[math.random(#last_names)]
        table.insert(params, "last_name=" .. last_name)
    end

    -- Если не выбрано ни одного параметра, используем first_name
    if #params == 0 then
        local first_name = first_names[math.random(#first_names)]
        table.insert(params, "first_name=" .. first_name)
    end

    -- Добавляем лимит и offset
    table.insert(params, "limit=" .. math.random(10, 50))
    table.insert(params, "offset=" .. math.random(0, 100))

    local query_string = table.concat(params, "&")
    local path = "/api/v1/user/search?" .. query_string

    return wrk.format("GET", path)
end
EOF

wrk -t$THREADS -c$CONNECTIONS -d$DURATION -s /tmp/user_search_test.lua $BASE_URL > user_search_results.txt
echo "Результаты сохранены в user_search_results.txt"
cat user_search_results.txt

echo
echo "=== Смешанный тест ==="
echo "Тестируем оба endpoint'а одновременно..."

cat > /tmp/mixed_test.lua << 'EOF'
-- Lua скрипт для смешанного тестирования
math.randomseed(os.time())

local first_names = {
    "Alexander", "Dmitry", "Maxim", "Sergey", "Andrew",
    "Alexey", "Artem", "Ilya", "Kirill", "Michael"
}

local last_names = {
    "Ivanov", "Petrov", "Sidorov", "Smirnov", "Kuznetsov",
    "Popov", "Vasiliev", "Sokolov", "Mikhailov", "Novikov"
}

function request()
    if math.random() > 0.5 then
        -- 50% запросов на /user/get/{id}
        local user_id = math.random(1, 1000000)
        local path = "/api/v1/user/get/" .. user_id
        return wrk.format("GET", path)
    else
        -- 50% запросов на /user/search
        local first_name = first_names[math.random(#first_names)]
        local last_name = last_names[math.random(#last_names)]
        local use_both = math.random() > 0.7

        local params = {}
        if use_both or math.random() > 0.5 then
            table.insert(params, "first_name=" .. first_name)
        end
        if use_both or math.random() > 0.5 then
            table.insert(params, "last_name=" .. last_name)
        end

        if #params == 0 then
            table.insert(params, "first_name=" .. first_name)
        end

        table.insert(params, "limit=" .. math.random(10, 50))

        local query_string = table.concat(params, "&")
        local path = "/api/v1/user/search?" .. query_string
        return wrk.format("GET", path)
    end
end
EOF

wrk -t$THREADS -c$CONNECTIONS -d$DURATION -s /tmp/mixed_test.lua $BASE_URL > mixed_results.txt
echo "Результаты сохранены в mixed_results.txt"
cat mixed_results.txt

# Очистка временных файлов
rm -f /tmp/user_get_test.lua /tmp/user_search_test.lua /tmp/mixed_test.lua

echo
echo "=== Тестирование завершено ==="
echo "Результаты сохранены в файлы:"
echo "- user_get_results.txt"
echo "- user_search_results.txt"
echo "- mixed_results.txt"
