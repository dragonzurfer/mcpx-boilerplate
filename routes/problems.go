package routes

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/stores"
)

type ProblemsHandler struct {
	Store *stores.Store
}

func (h *ProblemsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/problems", h.list)
	rg.GET("/problem-lists", h.listByLibrary)
	rg.GET("/problems/:slug", h.get)
}

func (h *ProblemsHandler) list(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	difficulty := strings.TrimSpace(c.Query("difficulty"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	problemListOutput, err := h.Store.ListProblems(stores.ProblemListInput{
		Status:     stores.ProblemStatusPublished,
		Difficulty: difficulty,
		Query:      query,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load problems"})
		return
	}

	items := make([]gin.H, 0, len(problemListOutput.Problems))
	for _, problemModel := range problemListOutput.Problems {
		items = append(items, publicProblemSummary(problemModel))
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": problemListOutput.Total})
}

func (h *ProblemsHandler) get(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	problemModel, err := h.Store.GetProblemBySlug(stores.ProblemLookupInput{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	if strings.ToUpper(strings.TrimSpace(problemModel.Status)) != stores.ProblemStatusPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	datasets, err := h.Store.ListDatasets(stores.DatasetListInput{ProblemID: problemModel.ID, Type: stores.DatasetTypePublic})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load datasets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"problem": publicProblemDetail(problemModel, datasets.Datasets)})
}

type practiceListView struct {
	ID          uint
	Name        string
	Slug        string
	Description string
	IsDefault   bool
	Problems    []stores.ProblemModel
}

func (h *ProblemsHandler) listByLibrary(c *gin.Context) {
	lists, err := h.loadPracticeLists()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load problem lists"})
		return
	}

	problemIDs := uniqueProblemIDsFromLists(lists)

	user, _ := middleware.CurrentUser(c)
	solvedSet, err := h.loadSolvedProblemSet(user, problemIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load solved stats"})
		return
	}

	items, problemIndex := buildPracticeListItems(lists, solvedSet)
	stats := buildPracticeStats(problemIndex, solvedSet)
	c.JSON(http.StatusOK, gin.H{"items": items, "stats": stats})
}

func (h *ProblemsHandler) loadPracticeLists() ([]practiceListView, error) {
	output, err := h.Store.ListProblemLists(stores.ProblemListListInput{})
	if err != nil {
		return nil, err
	}

	lists := make([]practiceListView, 0, len(output.Lists)+1)
	hasDefaultList := false

	for _, listModel := range output.Lists {
		problems, err := h.Store.ListProblemListProblems(stores.ProblemListProblemsInput{
			ProblemListID: listModel.ID,
			PublishedOnly: true,
		})
		if err != nil {
			return nil, err
		}

		lists = append(lists, practiceListView{
			ID:          listModel.ID,
			Name:        listModel.Name,
			Slug:        listModel.Slug,
			Description: listModel.Description,
			IsDefault:   listModel.IsDefault,
			Problems:    problems,
		})

		if isDefaultProblemList(listModel) {
			hasDefaultList = true
		}
	}

	if hasDefaultList {
		return lists, nil
	}

	defaultList, err := h.buildDefaultPracticeList()
	if err != nil {
		return nil, err
	}

	return append([]practiceListView{defaultList}, lists...), nil
}

func isDefaultProblemList(listModel stores.ProblemListModel) bool {
	if listModel.IsDefault {
		return true
	}

	if strings.EqualFold(strings.TrimSpace(listModel.Slug), "default-problems") {
		return true
	}

	return strings.EqualFold(strings.TrimSpace(listModel.Name), "default problems")
}

func (h *ProblemsHandler) buildDefaultPracticeList() (practiceListView, error) {
	problems, err := h.Store.ListPublishedProblems()
	if err != nil {
		return practiceListView{}, err
	}

	return practiceListView{
		Name:      "Default problems",
		Slug:      "default-problems",
		IsDefault: true,
		Problems:  problems,
	}, nil
}

func uniqueProblemIDsFromLists(lists []practiceListView) []uint {
	ids := []uint{}
	seen := map[uint]struct{}{}
	for _, list := range lists {
		for _, problem := range list.Problems {
			if _, ok := seen[problem.ID]; ok {
				continue
			}
			seen[problem.ID] = struct{}{}
			ids = append(ids, problem.ID)
		}
	}
	return ids
}

func (h *ProblemsHandler) loadSolvedProblemSet(user *stores.UserModel, problemIDs []uint) (map[uint]struct{}, error) {
	if user == nil || user.ID == 0 || len(problemIDs) == 0 {
		return map[uint]struct{}{}, nil
	}

	solvedIDs, err := h.Store.ListSolvedProblemIDs(stores.UserSolvedProblemIDsInput{
		UserID:     user.ID,
		ProblemIDs: problemIDs,
	})
	if err != nil {
		return nil, err
	}

	solvedSet := map[uint]struct{}{}
	for _, problemID := range solvedIDs {
		solvedSet[problemID] = struct{}{}
	}

	return solvedSet, nil
}

func buildPracticeListItems(lists []practiceListView, solvedSet map[uint]struct{}) ([]gin.H, map[uint]stores.ProblemModel) {
	items := make([]gin.H, 0, len(lists))
	problemIndex := map[uint]stores.ProblemModel{}

	for _, list := range lists {
		problems := make([]gin.H, 0, len(list.Problems))
		for _, problem := range list.Problems {
			_, solved := solvedSet[problem.ID]
			problems = append(problems, gin.H{
				"id":         problem.ID,
				"slug":       problem.Slug,
				"title":      problem.Title,
				"difficulty": problem.Difficulty,
				"tags":       stores.ParseProblemTags(problem.TagsJSON),
				"solved":     solved,
			})

			if _, exists := problemIndex[problem.ID]; !exists {
				problemIndex[problem.ID] = problem
			}
		}

		items = append(items, gin.H{
			"id":            list.ID,
			"name":          list.Name,
			"slug":          list.Slug,
			"description":   list.Description,
			"is_default":    list.IsDefault,
			"problem_count": len(problems),
			"problems":      problems,
		})
	}

	return items, problemIndex
}

func buildPracticeStats(problemIndex map[uint]stores.ProblemModel, solvedSet map[uint]struct{}) gin.H {
	difficultyStats := map[string]int{
		stores.ProblemDifficultyEasy:   0,
		stores.ProblemDifficultyMedium: 0,
		stores.ProblemDifficultyHard:   0,
	}
	totalByDifficulty := map[string]int{
		stores.ProblemDifficultyEasy:   0,
		stores.ProblemDifficultyMedium: 0,
		stores.ProblemDifficultyHard:   0,
	}

	totalSolved := 0
	tagCounts := map[string]int{}

	for problemID, problem := range problemIndex {
		difficulty := strings.ToUpper(strings.TrimSpace(problem.Difficulty))
		if _, ok := totalByDifficulty[difficulty]; ok {
			totalByDifficulty[difficulty]++
		}

		if _, solved := solvedSet[problemID]; !solved {
			continue
		}

		totalSolved++
		if _, ok := difficultyStats[difficulty]; ok {
			difficultyStats[difficulty]++
		}

		tags := stores.ParseProblemTags(problem.TagsJSON)
		seenTags := map[string]struct{}{}
		for _, tag := range tags {
			normalizedTag := strings.ToLower(strings.TrimSpace(tag))
			if normalizedTag == "" {
				continue
			}
			if _, exists := seenTags[normalizedTag]; exists {
				continue
			}
			seenTags[normalizedTag] = struct{}{}
			tagCounts[normalizedTag]++
		}
	}

	solvedByTag := make([]gin.H, 0, len(tagCounts))
	for tag, count := range tagCounts {
		solvedByTag = append(solvedByTag, gin.H{"tag": tag, "solved": count})
	}
	sort.Slice(solvedByTag, func(indexA int, indexB int) bool {
		left := solvedByTag[indexA]
		right := solvedByTag[indexB]
		leftCount := left["solved"].(int)
		rightCount := right["solved"].(int)
		if leftCount == rightCount {
			return left["tag"].(string) < right["tag"].(string)
		}
		return leftCount > rightCount
	})

	return gin.H{
		"total_problems":       len(problemIndex),
		"total_solved":         totalSolved,
		"solved_by_difficulty": difficultyStats,
		"total_by_difficulty":  totalByDifficulty,
		"solved_by_tag":        solvedByTag,
	}
}

func publicProblemSummary(problemModel stores.ProblemModel) gin.H {
	return gin.H{
		"id":         problemModel.ID,
		"slug":       problemModel.Slug,
		"title":      problemModel.Title,
		"difficulty": problemModel.Difficulty,
		"status":     problemModel.Status,
		"tags":       stores.ParseProblemTags(problemModel.TagsJSON),
	}
}

func publicProblemDetail(problemModel *stores.ProblemModel, datasets []stores.DatasetModel) gin.H {
	statement := stores.ParseProblemStatement(problemModel.StatementJSON)
	editorial := stores.ParseProblemEditorial(problemModel.EditorialJSON)
	ioSpec := rawProblemJSON(problemModel.IOSpecJSON, "{}")
	constraints := rawProblemJSON(problemModel.ConstraintsJSON, "{}")
	officialSolutions := rawProblemJSON(problemModel.SolutionsJSON, "[]")

	datasetItems := make([]gin.H, 0, len(datasets))
	for _, dataset := range datasets {
		datasetItems = append(datasetItems, gin.H{
			"id":   dataset.ID,
			"type": dataset.Type,
		})
	}

	return gin.H{
		"id":                 problemModel.ID,
		"slug":               problemModel.Slug,
		"title":              problemModel.Title,
		"difficulty":         problemModel.Difficulty,
		"status":             problemModel.Status,
		"statement":          statement,
		"editorial":          editorial,
		"io_spec":            ioSpec,
		"constraints":        constraints,
		"official_solutions": officialSolutions,
		"tags":               stores.ParseProblemTags(problemModel.TagsJSON),
		"datasets":           datasetItems,
	}
}
