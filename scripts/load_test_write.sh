#!/bin/bash

# Скрипт для нагрузочного тестирования API на запись
# Создает нагрузку на endpoint для создания пользователей

BASE_URL="http://localhost:8080"
DURATION="60s"
THREADS=2
CONNECTIONS=20
COUNTER_FILE="/tmp/write_counter.txt"

echo "=== Нагрузочное тестирование API на запись ==="
echo "URL: $BASE_URL"
echo "Длительность: $DURATION"
echo "Потоки: $THREADS"
echo "Соединения: $CONNECTIONS"
echo

# Проверяем наличие wrk
if ! command -v wrk &> /dev/null; then
    echo "wrk не найден. Установите wrk для нагрузочного тестирования"
    exit 1
fi

# Инициализируем счетчик успешных записей
echo "0" > $COUNTER_FILE

echo "=== Тест записи: /user/register ==="
echo "Тестируем создание пользователей..."

# Создаем Lua скрипт для регистрации пользователей
cat > /tmp/user_write_test.lua << 'EOF'
-- Lua скрипт для тестирования /user/register
math.randomseed(os.time())

local first_names = {
    "Александр", "Дмитрий", "Максим", "Сергей", "Андрей",
    "Алексей", "Артем", "Илья", "Кирилл", "Михаил",
    "Никита", "Матвей", "Роман", "Егор", "Арсений",
    "Владимир", "Павел", "Николай", "Данил", "Тимур"
}

local last_names = {
    "Иванов", "Петров", "Сидоров", "Смирнов", "Кузнецов",
    "Попов", "Васильев", "Соколов", "Михайлов", "Новиков",
    "Федоров", "Морозов", "Волков", "Алексеев", "Лебедев",
    "Семенов", "Егоров", "Павлов", "Козлов", "Степанов"
}

local cities = {
    "Москва", "Санкт-Петербург", "Новосибирск", "Екатеринбург", "Казань",
    "Нижний Новгород", "Челябинск", "Самара", "Омск", "Ростов-на-Дону"
}

local sexes = {"male", "female"}

-- Счетчик для уникальных никнеймов
local counter = 0

function init(args)
    counter = math.random(1, 1000000)
end

function request()
    counter = counter + 1

    local first_name = first_names[math.random(#first_names)]
    local last_name = last_names[math.random(#last_names)]
    local city = cities[math.random(#cities)]
    local sex = sexes[math.random(#sexes)]

    -- Генерируем уникальный никнейм
    local nickname = "user_" .. counter .. "_" .. math.random(1000, 9999)
    local password = "password123"

    -- Генерируем случайную дату рождения
    local year = math.random(1970, 2000)
    local month = math.random(1, 12)
    local day = math.random(1, 28)
    local birthday = string.format("%04d-%02d-%02dT00:00:00Z", year, month, day)

    local body = string.format([[{
        "nickname": "%s",
        "password": "%s",
        "first_name": "%s",
        "last_name": "%s",
        "birthday": "%s",
        "sex": "%s",
        "city": "%s"
    }]], nickname, password, first_name, last_name, birthday, sex, city)

    local headers = {}
    headers["Content-Type"] = "application/json"

    return wrk.format("POST", "/api/v1/auth/register", headers, body)
end

function response(status, headers, body)
    if status == 200 or status == 201 then
        -- Успешная запись
        local f = io.open("/tmp/write_counter.txt", "r")
        local count = 0
        if f then
            count = tonumber(f:read("*a")) or 0
            f:close()
        end

        local f = io.open("/tmp/write_counter.txt", "w")
        if f then
            f:write(tostring(count + 1))
            f:close()
        end
    end
end
EOF

echo "Запуск теста записи..."
wrk -t$THREADS -c$CONNECTIONS -d$DURATION -s /tmp/user_write_test.lua $BASE_URL > write_results.txt

echo "Результаты нагрузочного теста:"
cat write_results.txt

# Показываем количество успешных записей
if [ -f $COUNTER_FILE ]; then
    successful_writes=$(cat $COUNTER_FILE)
    echo
    echo "=== Статистика записи ==="
    echo "Успешно запи��ано пользователей: $successful_writes"
else
    echo "Не удалось получить статистику записи"
fi

# Очистка временных файлов
rm -f /tmp/user_write_test.lua $COUNTER_FILE

echo
echo "=== Тест записи завершен ==="
echo "Результаты сохранены в write_results.txt"
