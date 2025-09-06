package models

// ShardMap представляет маппинг пользователей на шарды
type ShardMap struct {
	UserID  int64 `gorm:"primaryKey" json:"user_id"`
	ShardID int   `gorm:"not null" json:"shard_id"`
}

// TableName возвращает имя таблицы для модели ShardMap
func (ShardMap) TableName() string {
	return "shard_map"
}
