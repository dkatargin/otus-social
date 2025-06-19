CREATE TYPE sex AS ENUM ('male', 'female');

CREATE TABLE users
(
    id            SERIAL PRIMARY KEY,
    nickname      VARCHAR(60)  NOT NULL UNIQUE,
    first_name    VARCHAR(255),
    last_name     VARCHAR(255),
    sex           sex,
    password_hash VARCHAR(255) NOT NULL,
    birthday      DATE         NOT NULL,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE interests
(
    id   SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE
);

CREATE TABLE user_interests
(
    user_id     INT NOT NULL,
    interest_id INT NOT NULL,
    PRIMARY KEY (user_id, interest_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (interest_id) REFERENCES interests (id) ON DELETE CASCADE
);

CREATE TABLE user_tokens
(
    id         SERIAL PRIMARY KEY,
    user_id    INT          NOT NULL,
    token      VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP DEFAULT (CURRENT_TIMESTAMP + INTERVAL '7 days'),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION trigger_set_timestamp()
    RETURNS trigger
	LANGUAGE plpgsql
AS $function$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$function$;

CREATE TRIGGER set_timestamp
    before
        update
    on
        users
    for each row execute function trigger_set_timestamp();
