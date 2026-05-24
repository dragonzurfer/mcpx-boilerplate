package stores

import (
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateUserPhone(t *testing.T) {
	store := newUserStoreForTest(t)

	user := UserModel{
		Email:     "user@example.com",
		Name:      "Example User",
		Role:      UserRoleUser,
		Status:    UserStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	updatedUser, err := store.UpdateUserPhone(UserPhoneUpdateInput{
		UserID:              user.ID,
		PhoneCountryCode:    " in ",
		PhoneNationalNumber: "9876543210",
		PhoneE164:           "+919876543210",
	})
	if err != nil {
		t.Fatalf("update user phone: %v", err)
	}

	if updatedUser.PhoneCountryCode != "IN" {
		t.Fatalf("expected country IN, got %s", updatedUser.PhoneCountryCode)
	}
	if updatedUser.PhoneNationalNumber != "9876543210" {
		t.Fatalf("expected national number 9876543210, got %s", updatedUser.PhoneNationalNumber)
	}
	if updatedUser.PhoneE164 != "+919876543210" {
		t.Fatalf("expected E164 +919876543210, got %s", updatedUser.PhoneE164)
	}

	storedUser, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	if storedUser.PhoneE164 != "+919876543210" {
		t.Fatalf("expected persisted E164 +919876543210, got %s", storedUser.PhoneE164)
	}
}

func TestUpdateUserPhoneRequiresExistingUser(t *testing.T) {
	store := newUserStoreForTest(t)

	_, err := store.UpdateUserPhone(UserPhoneUpdateInput{
		UserID:              99999,
		PhoneCountryCode:    "IN",
		PhoneNationalNumber: "9876543210",
		PhoneE164:           "+919876543210",
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func newUserStoreForTest(t *testing.T) *Store {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&UserModel{}, &OAuthIdentityModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	store := NewStoreWithDB(db)
	return store
}
