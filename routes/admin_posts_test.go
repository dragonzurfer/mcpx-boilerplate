package routes

import (
	"testing"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminPostDetailIncludesFields(t *testing.T) {
	post := &stores.PostModel{
		ID:              42,
		Slug:            "hello-world",
		Title:           "Hello",
		Excerpt:         "Excerpt",
		BodyMarkdown:    "# Body",
		AccessLevel:     stores.AccessLevelTrial,
		Status:          stores.PostStatusDraft,
		MetaTitle:       "Meta",
		MetaDescription: "Meta desc",
		MetaImageURL:    "https://example.com/og.png",
		CanonicalURL:    "https://example.com/post/hello",
		NoIndex:         true,
	}
	tags := []stores.TagModel{
		{Name: "AI", TagType: "TOPIC"},
		{Name: "Career", TagType: "INTEREST"},
	}

	payload := adminPostDetail(post, tags)

	if payload["body_markdown"] != "# Body" {
		t.Fatalf("expected body_markdown")
	}
	if payload["noindex"].(bool) != true {
		t.Fatalf("expected noindex true")
	}

	topicTags := payload["topic_tags"].([]string)
	if len(topicTags) != 1 || topicTags[0] != "AI" {
		t.Fatalf("expected topic tag AI")
	}
	interestTags := payload["interest_tags"].([]string)
	if len(interestTags) != 1 || interestTags[0] != "Career" {
		t.Fatalf("expected interest tag Career")
	}
}

func TestResolveTagIDsDeduplicates(t *testing.T) {
	store := newPostsTestStore(t)
	handler := AdminPostsHandler{Store: store}

	ids, err := handler.resolveTagIDs(
		[]string{"AI", "AI", "Systems"},
		[]string{"Career", "Career"},
	)
	if err != nil {
		t.Fatalf("expected resolveTagIDs to succeed, got %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 unique tag IDs, got %d", len(ids))
	}
	seen := map[uint]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("expected unique tag IDs, saw duplicate %d", id)
		}
		seen[id] = true
	}
}

func newPostsTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stores.TagModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return stores.NewStoreWithDB(db)
}
