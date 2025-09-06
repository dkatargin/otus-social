package models

// ShardMap - маппинг user_id -> shard_id для поддержки решардинга
type ShardMap struct {
	UserID  int64 `gorm:"primaryKey;uniqueIndex" json:"user_id"`
	ShardID int   `gorm:"index" json:"shard_id"`
}
