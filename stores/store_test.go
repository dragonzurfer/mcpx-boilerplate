package stores

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewStoreWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:store-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	store := NewStoreWithDB(db)
	if store == nil {
		t.Fatal("expected store")
	}
	if store.DB() != db {
		t.Fatal("expected store to use provided db")
	}
}
