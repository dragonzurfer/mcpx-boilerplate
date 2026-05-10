package stores

import (
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

type ProblemListListInput struct {
	Query               string
	IncludeProblemCount bool
}

type ProblemListListOutput struct {
	Lists              []ProblemListModel
	ProblemCountByList map[uint]int64
}

func (s *Store) ListProblemLists(input ProblemListListInput) (ProblemListListOutput, error) {
	query := s.db.Model(&ProblemListModel{})
	query = applyProblemListQueryFilter(query, input.Query)

	lists := []ProblemListModel{}
	if err := query.Order("is_default desc, name asc, id asc").Find(&lists).Error; err != nil {
		return ProblemListListOutput{}, err
	}

	countByList := map[uint]int64{}
	if input.IncludeProblemCount && len(lists) > 0 {
		counts, err := s.listProblemCountsByList(lists)
		if err != nil {
			return ProblemListListOutput{}, err
		}
		countByList = counts
	}

	return ProblemListListOutput{
		Lists:              lists,
		ProblemCountByList: countByList,
	}, nil
}

func applyProblemListQueryFilter(query *gorm.DB, rawQuery string) *gorm.DB {
	searchQuery := strings.TrimSpace(rawQuery)
	if searchQuery == "" {
		return query
	}

	likePattern := "%" + searchQuery + "%"
	return query.Where("name LIKE ? OR slug LIKE ?", likePattern, likePattern)
}

func (s *Store) listProblemCountsByList(lists []ProblemListModel) (map[uint]int64, error) {
	listIDs := extractProblemListIDs(lists)

	type countRow struct {
		ProblemListID uint
		Total         int64
	}

	rows := []countRow{}
	err := s.db.Model(&ProblemListProblemModel{}).
		Select("problem_list_id, COUNT(*) as total").
		Where("problem_list_id IN ?", listIDs).
		Group("problem_list_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	countByList := map[uint]int64{}
	for _, row := range rows {
		countByList[row.ProblemListID] = row.Total
	}

	return countByList, nil
}

func extractProblemListIDs(lists []ProblemListModel) []uint {
	listIDs := make([]uint, 0, len(lists))
	for _, listModel := range lists {
		listIDs = append(listIDs, listModel.ID)
	}
	return listIDs
}

type ProblemListLookupInput struct {
	ProblemListID uint
}

func (s *Store) GetProblemListByID(input ProblemListLookupInput) (*ProblemListModel, error) {
	if input.ProblemListID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	listModel := ProblemListModel{}
	if err := s.db.Where("id = ?", input.ProblemListID).First(&listModel).Error; err != nil {
		return nil, err
	}

	return &listModel, nil
}

type ProblemListCreateInput struct {
	Name        string
	Slug        string
	Description string
	ProblemIDs  []uint
	CreatedBy   uint
	IsDefault   bool
}

func (s *Store) CreateProblemList(input ProblemListCreateInput) (*ProblemListModel, error) {
	normalizedInput := normalizeProblemListCreateInput(input)
	if normalizedInput.Name == "" {
		return nil, gorm.ErrInvalidData
	}

	validProblemIDs, err := s.ensureProblemIDsExist(normalizedInput.ProblemIDs)
	if err != nil {
		return nil, err
	}

	listModel := buildProblemListModel(normalizedInput)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&listModel).Error; err != nil {
			return err
		}
		return replaceProblemListProblemsTx(tx, ProblemListProblemReplaceInput{
			ProblemListID: listModel.ID,
			ProblemIDs:    validProblemIDs,
		})
	})
	if err != nil {
		return nil, err
	}

	return &listModel, nil
}

type ProblemListUpdateInput struct {
	ProblemListID uint
	Name          string
	Slug          string
	Description   string
	ProblemIDs    []uint
	IsDefault     bool
}

func (s *Store) UpdateProblemList(input ProblemListUpdateInput) (*ProblemListModel, error) {
	if input.ProblemListID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	listModel, err := s.GetProblemListByID(ProblemListLookupInput{ProblemListID: input.ProblemListID})
	if err != nil {
		return nil, err
	}

	normalizedInput := normalizeProblemListUpdateInput(input, listModel)
	if normalizedInput.Name == "" {
		return nil, gorm.ErrInvalidData
	}

	validProblemIDs, err := s.ensureProblemIDsExist(normalizedInput.ProblemIDs)
	if err != nil {
		return nil, err
	}

	updateMap := map[string]interface{}{
		"name":        normalizedInput.Name,
		"slug":        normalizedInput.Slug,
		"description": normalizedInput.Description,
		"is_default":  normalizedInput.IsDefault,
		"updated_at":  time.Now().UTC(),
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(listModel).Updates(updateMap).Error; err != nil {
			return err
		}

		return replaceProblemListProblemsTx(tx, ProblemListProblemReplaceInput{
			ProblemListID: listModel.ID,
			ProblemIDs:    validProblemIDs,
		})
	})
	if err != nil {
		return nil, err
	}

	return s.GetProblemListByID(ProblemListLookupInput{ProblemListID: input.ProblemListID})
}

func normalizeProblemListCreateInput(input ProblemListCreateInput) ProblemListCreateInput {
	name := strings.TrimSpace(input.Name)
	return ProblemListCreateInput{
		Name:        name,
		Slug:        normalizeProblemListSlug(input.Slug, name),
		Description: strings.TrimSpace(input.Description),
		ProblemIDs:  uniqueProblemIDs(input.ProblemIDs),
		CreatedBy:   input.CreatedBy,
		IsDefault:   input.IsDefault,
	}
}

func normalizeProblemListUpdateInput(input ProblemListUpdateInput, existing *ProblemListModel) ProblemListUpdateInput {
	name := strings.TrimSpace(input.Name)
	if name == "" && existing != nil {
		name = existing.Name
	}

	slugSource := input.Slug
	if strings.TrimSpace(slugSource) == "" && existing != nil {
		slugSource = existing.Slug
	}

	return ProblemListUpdateInput{
		ProblemListID: input.ProblemListID,
		Name:          name,
		Slug:          normalizeProblemListSlug(slugSource, name),
		Description:   strings.TrimSpace(input.Description),
		ProblemIDs:    uniqueProblemIDs(input.ProblemIDs),
		IsDefault:     input.IsDefault,
	}
}

func buildProblemListModel(input ProblemListCreateInput) ProblemListModel {
	now := time.Now().UTC()
	return ProblemListModel{
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
		IsDefault:   input.IsDefault,
		CreatedBy:   input.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func normalizeProblemListSlug(rawSlug string, fallbackName string) string {
	candidate := strings.TrimSpace(rawSlug)
	if candidate == "" {
		candidate = strings.TrimSpace(fallbackName)
	}

	candidate = strings.ToLower(candidate)
	if candidate == "" {
		return ""
	}

	var builder strings.Builder
	previousDash := false
	for _, char := range candidate {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			previousDash = false
			continue
		}
		if previousDash {
			continue
		}
		builder.WriteRune('-')
		previousDash = true
	}

	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "list"
	}

	return normalized
}

func uniqueProblemIDs(problemIDs []uint) []uint {
	if len(problemIDs) == 0 {
		return []uint{}
	}

	unique := make([]uint, 0, len(problemIDs))
	seen := map[uint]struct{}{}
	for _, problemID := range problemIDs {
		if problemID == 0 {
			continue
		}
		if _, ok := seen[problemID]; ok {
			continue
		}
		seen[problemID] = struct{}{}
		unique = append(unique, problemID)
	}

	return unique
}

func (s *Store) ensureProblemIDsExist(problemIDs []uint) ([]uint, error) {
	cleanedIDs := uniqueProblemIDs(problemIDs)
	if len(cleanedIDs) == 0 {
		return cleanedIDs, nil
	}

	var existingCount int64
	err := s.db.Model(&ProblemModel{}).Where("id IN ?", cleanedIDs).Count(&existingCount).Error
	if err != nil {
		return nil, err
	}

	if existingCount != int64(len(cleanedIDs)) {
		return nil, gorm.ErrInvalidData
	}

	return cleanedIDs, nil
}

type ProblemListDeleteInput struct {
	ProblemListID uint
}

func (s *Store) DeleteProblemList(input ProblemListDeleteInput) error {
	if input.ProblemListID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("problem_list_id = ?", input.ProblemListID).Delete(&ProblemListProblemModel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", input.ProblemListID).Delete(&ProblemListModel{}).Error
	})
}

type ProblemListProblemReplaceInput struct {
	ProblemListID uint
	ProblemIDs    []uint
}

func (s *Store) ReplaceProblemListProblems(input ProblemListProblemReplaceInput) error {
	if input.ProblemListID == 0 {
		return gorm.ErrRecordNotFound
	}

	validProblemIDs, err := s.ensureProblemIDsExist(input.ProblemIDs)
	if err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		return replaceProblemListProblemsTx(tx, ProblemListProblemReplaceInput{
			ProblemListID: input.ProblemListID,
			ProblemIDs:    validProblemIDs,
		})
	})
}

func replaceProblemListProblemsTx(tx *gorm.DB, input ProblemListProblemReplaceInput) error {
	if err := tx.Where("problem_list_id = ?", input.ProblemListID).
		Delete(&ProblemListProblemModel{}).Error; err != nil {
		return err
	}

	if len(input.ProblemIDs) == 0 {
		return nil
	}

	rows := buildProblemListProblemRows(input)
	return tx.Create(&rows).Error
}

func buildProblemListProblemRows(input ProblemListProblemReplaceInput) []ProblemListProblemModel {
	rows := make([]ProblemListProblemModel, 0, len(input.ProblemIDs))
	now := time.Now().UTC()

	for index, problemID := range input.ProblemIDs {
		rows = append(rows, ProblemListProblemModel{
			ProblemListID: input.ProblemListID,
			ProblemID:     problemID,
			Position:      index + 1,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	return rows
}

type ProblemListProblemsInput struct {
	ProblemListID uint
	PublishedOnly bool
}

func (s *Store) ListProblemListProblems(input ProblemListProblemsInput) ([]ProblemModel, error) {
	if input.ProblemListID == 0 {
		return []ProblemModel{}, nil
	}

	query := s.db.Table("problem_list_problems AS plp").
		Select("p.*").
		Joins("JOIN problems AS p ON p.id = plp.problem_id").
		Where("plp.problem_list_id = ?", input.ProblemListID)

	if input.PublishedOnly {
		query = query.Where("p.status = ?", ProblemStatusPublished)
	}

	problems := []ProblemModel{}
	err := query.Order("plp.position asc, plp.id asc").Find(&problems).Error
	if err != nil {
		return nil, err
	}

	return problems, nil
}

type ProblemListProblemIDsInput struct {
	ProblemListID uint
}

func (s *Store) ListProblemListProblemIDs(input ProblemListProblemIDsInput) ([]uint, error) {
	if input.ProblemListID == 0 {
		return []uint{}, nil
	}

	rows := []ProblemListProblemModel{}
	err := s.db.Where("problem_list_id = ?", input.ProblemListID).
		Order("position asc, id asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	problemIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		problemIDs = append(problemIDs, row.ProblemID)
	}

	return problemIDs, nil
}

func (s *Store) ListPublishedProblems() ([]ProblemModel, error) {
	problems := []ProblemModel{}
	err := s.db.Model(&ProblemModel{}).
		Where("status = ?", ProblemStatusPublished).
		Order("published_at desc, id desc").
		Find(&problems).Error
	if err != nil {
		return nil, err
	}

	return problems, nil
}

type UserSolvedProblemIDsInput struct {
	UserID     uint
	ProblemIDs []uint
}

func (s *Store) ListSolvedProblemIDs(input UserSolvedProblemIDsInput) ([]uint, error) {
	if input.UserID == 0 {
		return []uint{}, nil
	}

	rows := []struct {
		ProblemID uint
	}{}

	acceptedVerdictEscapedPattern := `%\"verdict\":\"` + VerdictAccepted + `\"%`
	acceptedVerdictPattern := `%"verdict":"` + VerdictAccepted + `"%`

	query := s.db.Table("submissions").
		Select("DISTINCT submissions.problem_id AS problem_id").
		Joins("JOIN submission_results ON submission_results.submission_id = submissions.id").
		Where("submissions.user_id = ?", input.UserID).
		Where("submissions.mode = ?", SubmissionModeSubmit).
		Where(
			"(submission_results.overall_json LIKE ? OR submission_results.overall_json LIKE ?)",
			acceptedVerdictEscapedPattern,
			acceptedVerdictPattern,
		)

	filteredProblemIDs := uniqueProblemIDs(input.ProblemIDs)
	if len(filteredProblemIDs) > 0 {
		query = query.Where("submissions.problem_id IN ?", filteredProblemIDs)
	}

	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	problemIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		problemIDs = append(problemIDs, row.ProblemID)
	}

	return problemIDs, nil
}
