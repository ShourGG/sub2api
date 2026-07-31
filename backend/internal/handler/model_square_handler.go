package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelSquareHandler exposes the live catalogue of group-routable account models.
type ModelSquareHandler struct {
	service *service.ModelSquareService
}

func NewModelSquareHandler(service *service.ModelSquareService) *ModelSquareHandler {
	return &ModelSquareHandler{service: service}
}

type modelSquareEntry struct {
	Name         string                     `json:"name"`
	Platform     string                     `json:"platform"`
	ChannelID    int64                      `json:"channel_id"`
	ChannelName  string                     `json:"channel_name"`
	Group        userAvailableGroup         `json:"group"`
	AccountCount int                        `json:"account_count"`
	Pricing      *userSupportedModelPricing `json:"pricing"`
}

// List handles GET /api/v1/model-square.
func (h *ModelSquareHandler) List(c *gin.Context) {
	_, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	entries, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	entries = filterModelSquareVisibleEntries(entries)
	out := make([]modelSquareEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, modelSquareEntry{
			Name:         entry.Name,
			Platform:     entry.Platform,
			ChannelID:    entry.ChannelID,
			ChannelName:  entry.ChannelName,
			Group:        toUserAvailableGroup(entry.Group),
			AccountCount: entry.AccountCount,
			Pricing:      toUserPricing(entry.Pricing),
		})
	}
	response.Success(c, out)
}

// filterModelSquareVisibleEntries keeps the model square as a public catalogue.
// Exclusive groups are usable only by their assigned users and are deliberately
// omitted here, even for an assigned user.
func filterModelSquareVisibleEntries(entries []service.ModelSquareEntry) []service.ModelSquareEntry {
	visible := make([]service.ModelSquareEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Group.IsExclusive {
			continue
		}
		visible = append(visible, entry)
	}
	return visible
}
