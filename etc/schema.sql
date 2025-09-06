-- Создание типа для пола
CREATE TYPE sex AS ENUM ('male', 'female');

-- Создание таблицы пользователей (структура должна точно соответствовать Go модели)
CREATE TABLE "user" (
    id BIGSERIAL PRIMARY KEY,
    nickname VARCHAR(60) NOT NULL,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    password VARCHAR(255) NOT NULL,
    birthday TIMESTAMPTZ NOT NULL,
    sex sex,
    city VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Создание уникального индекса для nickname (GORM будет управлять этим)
CREATE UNIQUE INDEX uni_users_nickname ON "user" (nickname);

-- Создание индексов для поиска по имени и фамилии
CREATE INDEX idx_users_first_name ON "user" (first_name);
CREATE INDEX idx_users_last_name ON "user" (last_name);
CREATE INDEX idx_users_name_search ON "user" (first_name, last_name);

-- Создание таблицы интересов
CREATE TABLE interest (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(60) NOT NULL
);
CREATE UNIQUE INDEX uni_interest_name ON "interest" (name);



-- Создание таблицы связи пользователей и интересов
CREATE TABLE user_interest (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    interest_id BIGINT NOT NULL,
    UNIQUE(user_id, interest_id)
);

-- Создание индексов для связи пользователей и интересов
CREATE INDEX idx_user_interests_user_id ON user_interest (user_id);
CREATE INDEX idx_user_interests_interest_id ON user_interest (interest_id);

-- Создание таблицы токенов пользователей
CREATE TABLE user_token (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    token VARCHAR(255) NOT NULL
);

-- Создание индексов для токенов
CREATE INDEX idx_user_tokens_user_id ON user_token (user_id);
CREATE INDEX idx_user_tokens_token ON user_token (token);

-- Создание таблицы для отслеживания транзакций записи
CREATE TABLE write_transaction (
    id BIGSERIAL PRIMARY KEY,
    table_name VARCHAR(100) NOT NULL,
    operation VARCHAR(20) NOT NULL,
    record_id BIGINT,
    timestamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    test_session VARCHAR(100)
);

-- Создание индексов для отслеживания транзакций
CREATE INDEX idx_write_transactions_timestamp ON write_transaction (timestamp);
CREATE INDEX idx_write_transactions_test_session ON write_transaction (test_session);

-- Схема базы данных для социальной сети с поддержкой шардированного хранения диалогов

-- Таблица пользователей (основная)
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(255) NOT NULL,
    second_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    age INTEGER,
    biography TEXT,
    city VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица маппинга пользователей на шарды
CREATE TABLE IF NOT EXISTS shard_map (
    user_id BIGINT PRIMARY KEY,
    shard_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Создание индексов для shard_map
CREATE INDEX IF NOT EXISTS idx_shard_map_shard_id ON shard_map(shard_id);

-- Шаблон для создания шардированных таблиц сообщений
-- Для каждого шарда создается отдельная таблица messages_N

-- Таблица сообщений для шарда 0
CREATE TABLE IF NOT EXISTS messages_0 (
    id BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE
);

-- Индексы для эффективного поиска диалогов
CREATE INDEX IF NOT EXISTS idx_messages_0_dialog ON messages_0(from_user_id, to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_0_to_user ON messages_0(to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_0_from_user ON messages_0(from_user_id, created_at);

-- Таблица сообщений для шарда 1
CREATE TABLE IF NOT EXISTS messages_1 (
    id BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_messages_1_dialog ON messages_1(from_user_id, to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_1_to_user ON messages_1(to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_1_from_user ON messages_1(from_user_id, created_at);

-- Таблица сообщений для шарда 2
CREATE TABLE IF NOT EXISTS messages_2 (
    id BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_messages_2_dialog ON messages_2(from_user_id, to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_2_to_user ON messages_2(to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_2_from_user ON messages_2(from_user_id, created_at);

-- Таблица сообщений для шарда 3
CREATE TABLE IF NOT EXISTS messages_3 (
    id BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_messages_3_dialog ON messages_3(from_user_id, to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_3_to_user ON messages_3(to_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_3_from_user ON messages_3(from_user_id, created_at);

-- Функция для автоматического создания новых шардов
CREATE OR REPLACE FUNCTION create_messages_shard(shard_id INTEGER)
RETURNS VOID AS $$
DECLARE
    table_name TEXT;
BEGIN
    table_name := 'messages_' || shard_id;

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I (
            id BIGSERIAL PRIMARY KEY,
            from_user_id BIGINT NOT NULL,
            to_user_id BIGINT NOT NULL,
            text TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            is_read BOOLEAN DEFAULT FALSE
        )', table_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_dialog ON %I(from_user_id, to_user_id, created_at)', table_name, table_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_to_user ON %I(to_user_id, created_at)', table_name, table_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_from_user ON %I(from_user_id, created_at)', table_name, table_name);
END;
$$ LANGUAGE plpgsql;
