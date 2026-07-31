//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanvasModelsForAPIKeyIncludesModelsFromGroupChannel(t *testing.T) {
	group := Group{ID: 42, Name: "image", Status: StatusActive}
	groupRepo := &stubGroupRepoForAvailable{activeGroups: []Group{group}}
	channelService := newAvailableChannelService([]Channel{{
		ID:       1,
		Name:     "image-channel",
		Status:   StatusActive,
		GroupIDs: []int64{group.ID},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI,
			Models:   []string{"gpt-image-2"},
		}},
	}}, groupRepo)

	service := &CanvasService{
		apiKeys:        &APIKeyService{groupRepo: groupRepo},
		channelService: channelService,
	}

	models := service.canvasModelsForAPIKey(context.Background(), &APIKey{Group: &group})
	require.Equal(t, []string{"gpt-image-2"}, models)
}
