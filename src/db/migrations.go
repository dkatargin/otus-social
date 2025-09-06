package db

import (
	"fmt"
	"gorm.io/gorm"
)

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
	}
	return nil
}
