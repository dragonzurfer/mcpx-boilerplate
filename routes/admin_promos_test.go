package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpsertPromoPreservesCreatedAt(t *testing.T) {
	store := newPromoTestStore(t)

	created := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	promo := stores.PromoModel{
		Code:      "PROMO_ONE",
		Name:      "Promo One",
		Slot:      stores.PromoSlotInline,
		Status:    stores.PromoStatusActive,
		Priority:  10,
		CreatedAt: created,
		UpdatedAt: created,
	}
	if err := store.DB().Create(&promo).Error; err != nil {
		t.Fatalf("failed to create promo: %v", err)
	}

	handler := AdminPromosHandler{Store: store}
	req := promoRequest{
		Code:           "PROMO_ONE",
		Name:           "Promo Updated",
		Slot:           stores.PromoSlotInline,
		Status:         stores.PromoStatusActive,
		Priority:       20,
		EligibleStages: []string{"HOT"},
		Variants: []promoVariantRequest{
			{Headline: "H", Body: "B", CTAText: "CTA", CTAAction: "OPEN_PRICING", Weight: 1},
		},
	}
	updated, err := handler.upsertPromo(promo.ID, req)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated promo")
	}

	var loaded stores.PromoModel
	if err := store.DB().First(&loaded, promo.ID).Error; err != nil {
		t.Fatalf("failed to load promo: %v", err)
	}
	if loaded.CreatedAt.IsZero() {
		t.Fatal("expected created_at to remain set")
	}
	if !loaded.CreatedAt.Equal(created) {
		t.Fatalf("expected created_at %v, got %v", created, loaded.CreatedAt)
	}
}

func TestGetPromoReturnsVariants(t *testing.T) {
	store := newPromoTestStore(t)

	promo := stores.PromoModel{
		Code:     "PROMO_DETAIL",
		Name:     "Promo Detail",
		Slot:     stores.PromoSlotInline,
		Status:   stores.PromoStatusActive,
		Priority: 10,
	}
	if err := store.DB().Create(&promo).Error; err != nil {
		t.Fatalf("failed to create promo: %v", err)
	}
	variant := stores.PromoVariantModel{
		PromoID:   promo.ID,
		Headline:  "Headline",
		Body:      "Body",
		CTAText:   "CTA",
		CTAAction: "OPEN_PRICING",
		Weight:    1,
	}
	if err := store.DB().Create(&variant).Error; err != nil {
		t.Fatalf("failed to create variant: %v", err)
	}

	handler := AdminPromosHandler{Store: store}
	router := gin.New()
	api := router.Group("/api/admin")
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/promos/%d", promo.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Promo    stores.PromoModel          `json:"promo"`
		Variants []stores.PromoVariantModel `json:"variants"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Promo.ID != promo.ID {
		t.Fatalf("expected promo id %d, got %d", promo.ID, payload.Promo.ID)
	}
	if len(payload.Variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(payload.Variants))
	}
	if payload.Variants[0].Headline != variant.Headline {
		t.Fatalf("expected variant headline %q, got %q", variant.Headline, payload.Variants[0].Headline)
	}
}

func newPromoTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stores.PromoModel{}, &stores.PromoVariantModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return stores.NewStoreWithDB(db)
}
