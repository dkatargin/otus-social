-- PostgreSQL initialization script for test database
-- This file is automatically executed when PostgreSQL container starts

-- Create enum type for sex
CREATE TYPE sex AS ENUM ('male', 'female');

-- Create shard map table
CREATE TABLE shard_map (
    user_id INTEGER PRIMARY KEY,
    shard_id INTEGER NOT NULL
);

-- Create sharded message tables
CREATE TABLE messages_0 (
    id SERIAL PRIMARY KEY,
    from_user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages_1 (
    id SERIAL PRIMARY KEY,
    from_user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages_2 (
    id SERIAL PRIMARY KEY,
    from_user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages_3 (
    id SERIAL PRIMARY KEY,
    from_user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX idx_messages_0_from_user ON messages_0(from_user_id);
CREATE INDEX idx_messages_0_to_user ON messages_0(to_user_id);
CREATE INDEX idx_messages_0_created_at ON messages_0(created_at);

CREATE INDEX idx_messages_1_from_user ON messages_1(from_user_id);
CREATE INDEX idx_messages_1_to_user ON messages_1(to_user_id);
CREATE INDEX idx_messages_1_created_at ON messages_1(created_at);

CREATE INDEX idx_messages_2_from_user ON messages_2(from_user_id);
CREATE INDEX idx_messages_2_to_user ON messages_2(to_user_id);
CREATE INDEX idx_messages_2_created_at ON messages_2(created_at);

CREATE INDEX idx_messages_3_from_user ON messages_3(from_user_id);
CREATE INDEX idx_messages_3_to_user ON messages_3(to_user_id);
CREATE INDEX idx_messages_3_created_at ON messages_3(created_at);