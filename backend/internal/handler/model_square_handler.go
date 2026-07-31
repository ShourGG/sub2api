package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelSquareHandler exposes the live catalogue of group-routable account models.
type ModelSquareHandler struct {
	service       *service.ModelSquareService
	apiKeyService *service.APIKeyService
}

func NewModelSquareHandler(
	service *service.ModelSquareService,
	apiKeyService *service.APIKeyService,
) *ModelSquareHandler {
	return &ModelSquareHandler{service: service, apiKeyService: apiKeyService}
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
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	allowedExclusive, err := h.apiKeyService.GetUserAllowedGroupIDSet(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	entries, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	entries = filterModelSquareVisibleEntries(entries, allowedExclusive)
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

// filterModelSquareVisibleEntries hides exclusive groups which are not
// explicitly granted to the current user. Public groups remain visible to all
// authenticated users, matching the API key group picker and Model Plaza.
func filterModelSquareVisibleEntries(
	entries []service.ModelSquareEntry,
	allowedExclusive map[int64]struct{},
) []service.ModelSquareEntry {
	visible := make([]service.ModelSquareEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Group.IsExclusive {
			if _, ok := allowedExclusive[entry.Group.ID]; !ok {
				continue
			}
		}
		visible = append(visible, entry)
	}
	return visible
}
