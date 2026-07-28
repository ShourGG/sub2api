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
	Group        userAvailableGroup         `json:"group"`
	AccountCount int                        `json:"account_count"`
	Pricing      *userSupportedModelPricing `json:"pricing"`
}

// List handles GET /api/v1/model-square.
func (h *ModelSquareHandler) List(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	entries, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]modelSquareEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, modelSquareEntry{
			Name:         entry.Name,
			Platform:     entry.Platform,
			Group:        toUserAvailableGroup(entry.Group),
			AccountCount: entry.AccountCount,
			Pricing:      toUserPricing(entry.Pricing),
		})
	}
	response.Success(c, out)
}
