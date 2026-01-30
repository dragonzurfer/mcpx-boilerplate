package stores

import "time"

type EventModel struct {
	ID         uint64    `gorm:"primaryKey"`
	UserID     *uint     `gorm:"index"`
	AnonID     *string   `gorm:"type:varchar(191);index"`
	EventType  string    `gorm:"type:varchar(64);index"`
	EntityType string    `gorm:"type:varchar(32);index"`
	EntityID   *uint     `gorm:"index"`
	Metadata   string    `gorm:"type:longtext"`
	CreatedAt  time.Time `gorm:"index"`
	UpdatedAt  time.Time
}

func (EventModel) TableName() string {
	return "events"
}
