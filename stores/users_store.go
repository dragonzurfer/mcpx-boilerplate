package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type UserIdentityInput struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

type IdentityLookupInput struct {
	Provider       string
	ProviderUserID string
}

func (s *Store) GetUserByIdentity(input IdentityLookupInput) (*UserModel, error) {
	provider := strings.TrimSpace(input.Provider)
	providerUserID := strings.TrimSpace(input.ProviderUserID)
	if providerUserID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	identity := OAuthIdentityModel{}
	query := s.db.Where("provider_user_id = ?", providerUserID)
	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if err := query.First(&identity).Error; err != nil {
		return nil, err
	}

	user := UserModel{}
	if err := s.db.Where("id = ?", identity.UserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByID(userID uint) (*UserModel, error) {
	if userID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	user := UserModel{}
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetOrCreateUserWithIdentity(input UserIdentityInput) (*UserModel, error) {
	provider := strings.TrimSpace(input.Provider)
	providerUserID := strings.TrimSpace(input.ProviderUserID)
	if provider == "" || providerUserID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	transaction := s.db.Begin()
	if transaction.Error != nil {
		return nil, transaction.Error
	}

	user, err := getOrCreateUserByIdentity(transaction, input)
	if err != nil {
		transaction.Rollback()
		return nil, err
	}

	if err := transaction.Commit().Error; err != nil {
		return nil, err
	}
	return user, nil
}

func getOrCreateUserByIdentity(db *gorm.DB, input UserIdentityInput) (*UserModel, error) {
	provider := strings.TrimSpace(input.Provider)
	providerUserID := strings.TrimSpace(input.ProviderUserID)

	identity := OAuthIdentityModel{}
	identityErr := db.Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&identity).Error
	if identityErr == nil {
		user := UserModel{}
		if err := db.Where("id = ?", identity.UserID).First(&user).Error; err != nil {
			return nil, err
		}
		updatedUser, err := applyUserUpdates(db, &user, input)
		if err != nil {
			return nil, err
		}
		return updatedUser, nil
	}
	if !errorsIsNotFound(identityErr) {
		return nil, identityErr
	}

	user, err := findOrCreateUser(db, input)
	if err != nil {
		return nil, err
	}

	identity = OAuthIdentityModel{
		UserID:         user.ID,
		Provider:       provider,
		ProviderUserID: providerUserID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := db.Create(&identity).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func findOrCreateUser(db *gorm.DB, input UserIdentityInput) (*UserModel, error) {
	email := strings.TrimSpace(input.Email)
	user := UserModel{}
	if email != "" {
		err := db.Where("email = ?", email).First(&user).Error
		if err == nil {
			updatedUser, err := applyUserUpdates(db, &user, input)
			if err != nil {
				return nil, err
			}
			return updatedUser, nil
		}
		if !errorsIsNotFound(err) {
			return nil, err
		}
	}

	user = UserModel{
		Email:     email,
		Name:      strings.TrimSpace(input.Name),
		AvatarURL: strings.TrimSpace(input.AvatarURL),
		Role:      UserRoleUser,
		Status:    UserStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func applyUserUpdates(db *gorm.DB, user *UserModel, input UserIdentityInput) (*UserModel, error) {
	updates := map[string]interface{}{}
	name := strings.TrimSpace(input.Name)
	if name != "" && name != user.Name {
		updates["name"] = name
	}
	avatar := strings.TrimSpace(input.AvatarURL)
	if avatar != "" && avatar != user.AvatarURL {
		updates["avatar_url"] = avatar
	}
	email := strings.TrimSpace(input.Email)
	if email != "" && email != user.Email {
		updates["email"] = email
	}
	if len(updates) == 0 {
		return user, nil
	}

	updates["updated_at"] = time.Now().UTC()
	if err := db.Model(user).Updates(updates).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
