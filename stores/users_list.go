package stores

import "strings"

type UserListInput struct {
	Query    string
	Page     int
	PageSize int
}

type UserListOutput struct {
	Users []UserModel
	Total int64
}

func (s *Store) ListUsers(input UserListInput) (UserListOutput, error) {
	key := userListKey(input)
	if cached, ok := s.getCachedUserList(key); ok {
		return cached, nil
	}

	query := s.db.Model(&UserModel{})
	if input.Query != "" {
		like := "%" + strings.TrimSpace(input.Query) + "%"
		query = query.Where("email LIKE ? OR name LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return UserListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize
	users := []UserModel{}
	if err := query.Order("created_at desc").Limit(pageSize).Offset(offset).Find(&users).Error; err != nil {
		return UserListOutput{}, err
	}
	output := UserListOutput{Users: users, Total: total}
	s.cacheSet(key, output)
	return output, nil
}

func (s *Store) ListUserIDs(page, pageSize int) ([]uint, error) {
	pageSize = clampPageSize(pageSize)
	offset := clampPage(page) * pageSize
	ids := []uint{}
	if err := s.db.Model(&UserModel{}).Select("id").Order("id asc").Limit(pageSize).Offset(offset).Find(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
