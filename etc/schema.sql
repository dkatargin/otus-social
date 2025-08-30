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

