package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelSquareGroupRepoStub struct {
	groups []Group
}

func (s modelSquareGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	return s.groups, nil
}

type modelSquareAccountRepoStub struct {
	accounts map[int64][]Account
}

func (s modelSquareAccountRepoStub) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	return s.accounts[groupID], nil
}

type modelSquareChannelServiceStub struct {
	channels []AvailableChannel
}

func (s modelSquareChannelServiceStub) ListAvailable(context.Context) ([]AvailableChannel, error) {
	return s.channels, nil
}

func TestModelSquareList_AccountModelsAppearWithoutChannel(t *testing.T) {
	svc := NewModelSquareService(
		modelSquareGroupRepoStub{groups: []Group{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1}}},
		modelSquareAccountRepoStub{accounts: map[int64][]Account{
			1: []Account{{ID: 101, Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5"}}}}}},
		nil,
	)

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "gpt-5", entries[0].Name)
	require.Equal(t, int64(1), entries[0].Group.ID)
	require.Equal(t, 1, entries[0].AccountCount)
	require.Nil(t, entries[0].Pricing)
}

func TestModelSquareList_KeepsDuplicateModelsPerGroupAndScalesPrice(t *testing.T) {
	inputPrice := 0.000002
	groups := []Group{
		{ID: 1, Name: "Standard", Platform: PlatformAnthropic, Status: StatusActive, RateMultiplier: 1},
		{ID: 2, Name: "Premium", Platform: PlatformAnthropic, Status: StatusActive, RateMultiplier: 2},
	}
	accounts := map[int64][]Account{
		1: []Account{{ID: 101, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"claude-sonnet": "claude-sonnet"}}}},
		2: []Account{{ID: 201, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"claude-sonnet": "claude-sonnet"}}}},
	}
	channels := []AvailableChannel{{
		Status: StatusActive,
		Groups: []AvailableGroupRef{
			{ID: 1, Name: "Standard", Platform: PlatformAnthropic},
			{ID: 2, Name: "Premium", Platform: PlatformAnthropic},
		},
		SupportedModels: []SupportedModel{{
			Name:     "claude-sonnet",
			Platform: PlatformAnthropic,
			Pricing:  &ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: &inputPrice},
		}},
	}}
	svc := NewModelSquareService(
		modelSquareGroupRepoStub{groups: groups},
		modelSquareAccountRepoStub{accounts: accounts},
		modelSquareChannelServiceStub{channels: channels},
	)

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, int64(2), entries[0].Group.ID)
	require.Equal(t, int64(1), entries[1].Group.ID)
	require.NotNil(t, entries[0].Pricing)
	require.NotNil(t, entries[1].Pricing)
	require.InDelta(t, inputPrice*2, *entries[0].Pricing.InputPrice, 1e-12)
	require.InDelta(t, inputPrice, *entries[1].Pricing.InputPrice, 1e-12)
}

func TestModelSquareList_ChannelModelsSyncWithoutAccountMapping(t *testing.T) {
	inputPrice := 0.000003
	svc := NewModelSquareService(
		modelSquareGroupRepoStub{groups: []Group{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1.5}}},
		modelSquareAccountRepoStub{accounts: map[int64][]Account{
			// An empty mapping is intentionally unrestricted and therefore supports
			// the model configured at channel level.
			1: {{ID: 101, Platform: PlatformOpenAI, Credentials: map[string]any{}}},
		}},
		modelSquareChannelServiceStub{channels: []AvailableChannel{{
			Status: StatusActive,
			Groups: []AvailableGroupRef{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
			SupportedModels: []SupportedModel{{
				Name:     "gpt-5.4",
				Platform: PlatformOpenAI,
				Pricing:  &ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: &inputPrice},
			}},
		}}},
	)

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "gpt-5.4", entries[0].Name)
	require.Equal(t, 1, entries[0].AccountCount)
	require.NotNil(t, entries[0].Pricing)
	require.InDelta(t, inputPrice*1.5, *entries[0].Pricing.InputPrice, 1e-12)
}

func TestModelSquareList_HidesChannelModelWithoutSupportingAccount(t *testing.T) {
	svc := NewModelSquareService(
		modelSquareGroupRepoStub{groups: []Group{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1}}},
		modelSquareAccountRepoStub{accounts: map[int64][]Account{
			1: {{ID: 101, Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-4.1": "gpt-4.1"}}}},
		}},
		modelSquareChannelServiceStub{channels: []AvailableChannel{{
			Status:          StatusActive,
			Groups:          []AvailableGroupRef{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
			SupportedModels: []SupportedModel{{Name: "gpt-5.4", Platform: PlatformOpenAI}},
		}}},
	)

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "gpt-4.1", entries[0].Name)
}
