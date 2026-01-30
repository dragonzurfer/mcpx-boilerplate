package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/stores"
)

type EventsHandler struct {
	Store *stores.Store
}

type eventsBatchRequest struct {
	AnonID string              `json:"anon_id"`
	Events []eventBatchRequest `json:"events"`
}

type eventBatchRequest struct {
	Type       string                 `json:"type"`
	EntityType string                 `json:"entity_type"`
	EntityID   uint                   `json:"entity_id"`
	Meta       map[string]interface{} `json:"meta"`
}

func (h *EventsHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/events/batch", h.batch)
}

func (h *EventsHandler) batch(c *gin.Context) {
	var req eventsBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(req.Events) == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	user, _ := middleware.CurrentUser(c)
	var userID *uint
	if user != nil {
		userID = &user.ID
	}

	events := make([]stores.EventInput, 0, len(req.Events))
	for _, evt := range req.Events {
		entityID := evt.EntityID
		var entityPtr *uint
		if entityID > 0 {
			entityPtr = &entityID
		}
		events = append(events, stores.EventInput{
			EventType:  evt.Type,
			EntityType: evt.EntityType,
			EntityID:   entityPtr,
			Metadata:   evt.Meta,
		})
	}

	now := time.Now().UTC()
	if err := h.Store.InsertEvents(stores.EventBatchInput{UserID: userID, AnonID: req.AnonID, Events: events, Now: now}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
