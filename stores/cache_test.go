package stores

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListPostsCachesResults(t *testing.T) {
	store := newCacheTestStore(t)
	now := time.Now().UTC()

	first := PostModel{
		Slug:        "first",
		Title:       "First",
		AccessLevel: AccessLevelPublic,
		Status:      PostStatusPublished,
		PublishedAt: &now,
	}
	if err := store.db.Create(&first).Error; err != nil {
		t.Fatalf("create first post: %v", err)
	}

	output, err := store.ListPosts(PostListInput{Status: PostStatusPublished})
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if len(output.Posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(output.Posts))
	}

	second := PostModel{
		Slug:        "second",
		Title:       "Second",
		AccessLevel: AccessLevelPublic,
		Status:      PostStatusPublished,
		PublishedAt: &now,
	}
	if err := store.db.Create(&second).Error; err != nil {
		t.Fatalf("create second post: %v", err)
	}

	output, err = store.ListPosts(PostListInput{Status: PostStatusPublished})
	if err != nil {
		t.Fatalf("list posts cached: %v", err)
	}
	if len(output.Posts) != 1 {
		t.Fatalf("expected cached 1 post, got %d", len(output.Posts))
	}
}

func TestGetPostBySlugCachesResults(t *testing.T) {
	store := newCacheTestStore(t)
	now := time.Now().UTC()

	post := PostModel{
		Slug:        "cached-post",
		Title:       "Original",
		AccessLevel: AccessLevelPublic,
		Status:      PostStatusPublished,
		PublishedAt: &now,
	}
	if err := store.db.Create(&post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	output, _, err := store.GetPostBySlug(PostLookupInput{Slug: "cached-post"})
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	firstTitle := output.Title

	if err := store.db.Model(&PostModel{}).Where("id = ?", post.ID).Update("title", "Updated").Error; err != nil {
		t.Fatalf("update post: %v", err)
	}

	output, _, err = store.GetPostBySlug(PostLookupInput{Slug: "cached-post"})
	if err != nil {
		t.Fatalf("get post cached: %v", err)
	}
	if output.Title != firstTitle {
		t.Fatalf("expected cached title %q, got %q", firstTitle, output.Title)
	}
}

func newCacheTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&PostModel{}); err != nil {
		t.Fatalf("migrate posts: %v", err)
	}
	store := NewStoreWithDB(db)
	store.EnableCache(CacheConfig{TTL: time.Minute, MaxEntries: 100})
	return store
}
