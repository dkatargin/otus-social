package db

import (
	"fmt"

	"gorm.io/gorm"
)

// Создаем enum для пола

// CreateShardedMessageTables создает N таблиц для сообщений (messages_0, messages_1, ...)
func CreateShardedMessageTables(db *gorm.DB, shards int) error {
	for i := 0; i < shards; i++ {
		tableName := fmt.Sprintf("messages_%d", i)
		createTableSQL := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id BIGSERIAL PRIMARY KEY,
				from_user_id BIGINT NOT NULL,
				to_user_id BIGINT NOT NULL,
				text TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT now(),
				is_read BOOLEAN NOT NULL DEFAULT false
			);
		`, tableName)
		if err := db.Exec(createTableSQL).Error; err != nil {
			return fmt.Errorf("failed to create table %s: %w", tableName, err)
		}

		//	создаем индекс для быстрого поиска по to_user_id и created_at
		indexName := fmt.Sprintf("idx_%s_from_to_user_id_created_at", tableName)
		createIndexSQL := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s (to_user_id, created_at);
		`, indexName, tableName)
		if err := db.Exec(createIndexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index %s: %w", indexName, err)
		}

		// создаем индекс для быстрого поиска по from_user_id и created_at
		indexName = fmt.Sprintf("idx_%s_from_user_id_created_at", tableName)
		createIndexSQL = fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s (from_user_id, created_at);
		`, indexName, tableName)
		if err := db.Exec(createIndexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index %s: %w", indexName, err)
		}
	}
	return nil
}

// CreateSexEnum создает тип ENUM sex, если он не существует
func CreateSexEnum(db *gorm.DB) error {
	createEnumSQL := `
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'sex') THEN
			CREATE TYPE sex AS ENUM ('male', 'female');
		END IF;
	END
	$$;
	`
	if err := db.Exec(createEnumSQL).Error; err != nil {
		return fmt.Errorf("failed to create enum sex: %w", err)
	}
	return nil
}
