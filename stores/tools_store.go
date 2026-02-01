package stores

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ToolSeedInput struct {
	Name       string
	Slug       string
	Category   string
	IsActive   bool
	IsPaidTool bool
	Config     ToolConfig
}

func (s *Store) EnsureDefaultTools(inputs []ToolSeedInput) error {
	for _, input := range inputs {
		if strings.TrimSpace(input.Slug) == "" {
			continue
		}
		existing := ToolModel{}
		err := s.db.Where("slug = ?", input.Slug).First(&existing).Error
		if err == nil {
			updates := map[string]interface{}{
				"name":         input.Name,
				"category":     input.Category,
				"is_active":    input.IsActive,
				"is_paid_tool": input.IsPaidTool,
				"config_json":  SerializeToolConfig(input.Config),
				"updated_at":   time.Now().UTC(),
			}
			if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil && !errorsIsNotFound(err) {
			return err
		}

		row := ToolModel{
			Name:       input.Name,
			Slug:       input.Slug,
			Category:   input.Category,
			IsActive:   input.IsActive,
			IsPaidTool: input.IsPaidTool,
			ConfigJSON: SerializeToolConfig(input.Config),
		}
		if err := s.db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

type ToolListInput struct {
	ActiveOnly bool
}

func (s *Store) ListTools(input ToolListInput) ([]ToolModel, error) {
	rows := []ToolModel{}
	query := s.db.Model(&ToolModel{})
	if input.ActiveOnly {
		query = query.Where("is_active = ?", true)
	}
	if err := query.Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) GetToolBySlug(slug string) (*ToolModel, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	row := ToolModel{}
	if err := s.db.Where("slug = ?", slug).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

type ToolUpdateInput struct {
	ToolID    uint
	Name      string
	Category  string
	IsActive  bool
	IsPaid    bool
	Config    ToolConfig
	UpdatedAt time.Time
}

func (s *Store) UpdateTool(input ToolUpdateInput) (*ToolModel, error) {
	if input.ToolID == 0 {
		return nil, gorm.ErrInvalidData
	}
	row := ToolModel{}
	if err := s.db.Where("id = ?", input.ToolID).First(&row).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":         strings.TrimSpace(input.Name),
		"category":     strings.TrimSpace(input.Category),
		"is_active":    input.IsActive,
		"is_paid_tool": input.IsPaid,
		"config_json":  SerializeToolConfig(input.Config),
		"updated_at":   input.UpdatedAt,
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

type ToolUsageLookupInput struct {
	UserID    uint
	ToolID    uint
	Now       time.Time
	ForUpdate bool
	Tx        *gorm.DB
}

func (s *Store) GetOrCreateToolUsage(input ToolUsageLookupInput) (*ToolUsageModel, ToolUsageState, error) {
	if input.UserID == 0 || input.ToolID == 0 {
		return nil, ToolUsageState{}, gorm.ErrInvalidData
	}

	db := input.Tx
	if db == nil {
		db = s.db
	}

	query := db.Model(&ToolUsageModel{})
	if input.ForUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	usage := ToolUsageModel{}
	err := query.Where("user_id = ? AND tool_id = ?", input.UserID, input.ToolID).First(&usage).Error
	if err == nil {
		return &usage, ParseToolUsageState(usage.UsageStateJSON), nil
	}
	if err != nil && !errorsIsNotFound(err) {
		return nil, ToolUsageState{}, err
	}

	state := DefaultToolUsageState()
	usage = ToolUsageModel{
		UserID:         input.UserID,
		ToolID:         input.ToolID,
		UsageStateJSON: SerializeToolUsageState(state),
		FirstUsedAt:    input.Now,
		LastUsedAt:     input.Now,
	}
	if err := db.Create(&usage).Error; err != nil {
		return nil, ToolUsageState{}, err
	}
	return &usage, state, nil
}

type ToolUsageSaveInput struct {
	Tx    *gorm.DB
	Usage *ToolUsageModel
	State ToolUsageState
	Now   time.Time
}

func (s *Store) SaveToolUsage(input ToolUsageSaveInput) error {
	if input.Usage == nil {
		return gorm.ErrInvalidData
	}
	db := input.Tx
	if db == nil {
		db = s.db
	}
	input.Usage.UsageStateJSON = SerializeToolUsageState(input.State)
	input.Usage.LastUsedAt = input.Now
	input.Usage.UpdatedAt = input.Now
	return db.Save(input.Usage).Error
}

type ToolEventInput struct {
	ToolID    uint
	UserID    uint
	EventName string
	Metadata  map[string]interface{}
	CreatedAt time.Time
	Tx        *gorm.DB
}

func (s *Store) CreateToolEvent(input ToolEventInput) error {
	if input.ToolID == 0 || input.UserID == 0 || strings.TrimSpace(input.EventName) == "" {
		return gorm.ErrInvalidData
	}
	db := input.Tx
	if db == nil {
		db = s.db
	}
	payload := ""
	if len(input.Metadata) > 0 {
		raw, err := json.Marshal(input.Metadata)
		if err == nil {
			payload = string(raw)
		}
	}
	row := ToolEventModel{
		ToolID:    input.ToolID,
		UserID:    input.UserID,
		EventName: strings.TrimSpace(input.EventName),
		Metadata:  payload,
		CreatedAt: input.CreatedAt,
	}
	return db.Create(&row).Error
}
