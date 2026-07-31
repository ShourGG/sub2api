package service

import (
	"context"
	"sort"
	"strings"
)

// ModelSquareEntry is one routable model in one active group. A model is kept
// separate for every group because its effective price and available accounts
// may differ even when the model name is the same.
type ModelSquareEntry struct {
	Name         string
	Platform     string
	ChannelID    int64
	ChannelName  string
	Group        AvailableGroupRef
	AccountCount int
	Pricing      *ChannelModelPricing
}

type modelSquareGroupRepository interface {
	ListActive(ctx context.Context) ([]Group, error)
}

type modelSquareAccountRepository interface {
	ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error)
}

type modelSquareChannelService interface {
	ListAvailable(ctx context.Context) ([]AvailableChannel, error)
}

// ModelSquareService builds the user-facing model square from active groups,
// channel configuration, and schedulable accounts. A channel model is a
// deliberate public catalogue entry even before every account has a restrictive
// model mapping, while account mappings add models not managed in channels.
type ModelSquareService struct {
	groupRepo      modelSquareGroupRepository
	accountRepo    modelSquareAccountRepository
	channelService modelSquareChannelService
}

func NewModelSquareService(
	groupRepo modelSquareGroupRepository,
	accountRepo modelSquareAccountRepository,
	channelService modelSquareChannelService,
) *ModelSquareService {
	return &ModelSquareService{
		groupRepo:      groupRepo,
		accountRepo:    accountRepo,
		channelService: channelService,
	}
}

// ProvideModelSquareService keeps Wire on the public repository contracts while
// NewModelSquareService retains narrow interfaces for focused unit tests.
func ProvideModelSquareService(
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	channelService *ChannelService,
) *ModelSquareService {
	return NewModelSquareService(groupRepo, accountRepo, channelService)
}

// List returns all active groups' mapped models for authenticated users. It is
// intentionally not scoped by API-key group permissions: the model square is a
// catalogue, while actual gateway access remains enforced by API key routing.
func (s *ModelSquareService) List(ctx context.Context) ([]ModelSquareEntry, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	channelModels := map[modelSquarePricingKey]modelSquareChannelModel{}
	if s.channelService != nil {
		channels, err := s.channelService.ListAvailable(ctx)
		if err != nil {
			return nil, err
		}
		channelModels = buildModelSquareChannelModels(channels)
	}

	entries := make([]ModelSquareEntry, 0)
	for i := range groups {
		group := groups[i]
		accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, group.ID)
		if err != nil {
			return nil, err
		}

		models := make(map[modelSquareEntryKey]modelSquareModel)
		configuredModels := make(map[string]struct{})
		for key, channelModel := range channelModels {
			if key.groupID != group.ID || key.platform != group.Platform {
				continue
			}
			models[modelSquareEntryKey{channelID: channelModel.channelID, model: key.model}] = modelSquareModel(channelModel)
			configuredModels[key.model] = struct{}{}
		}
		for _, account := range accounts {
			for model := range account.GetModelMapping() {
				name := strings.TrimSpace(model)
				if name == "" {
					continue
				}
				key := strings.ToLower(name)
				if _, exists := configuredModels[key]; !exists {
					models[modelSquareEntryKey{model: key}] = modelSquareModel{name: name}
					configuredModels[key] = struct{}{}
				}
			}
		}

		ref := availableGroupRefFromGroup(group)
		for _, model := range models {
			accountCount := 0
			for i := range accounts {
				if accounts[i].IsModelSupported(model.name) {
					accountCount++
				}
			}
			if accountCount == 0 {
				continue
			}
			entries = append(entries, ModelSquareEntry{
				Name:         model.name,
				Platform:     group.Platform,
				ChannelID:    model.channelID,
				ChannelName:  model.channelName,
				Group:        ref,
				AccountCount: accountCount,
				// The model square displays the channel base price. Rate and peak
				// multipliers remain visible in the group badge and are applied only
				// when the user makes a request through that group.
				Pricing: model.pricing,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Platform != entries[j].Platform {
			return entries[i].Platform < entries[j].Platform
		}
		if entries[i].Name != entries[j].Name {
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}
		if entries[i].Group.Name != entries[j].Group.Name {
			return entries[i].Group.Name < entries[j].Group.Name
		}
		if entries[i].ChannelName != entries[j].ChannelName {
			return entries[i].ChannelName < entries[j].ChannelName
		}
		return entries[i].ChannelID < entries[j].ChannelID
	})
	return entries, nil
}

type modelSquarePricingKey struct {
	groupID   int64
	channelID int64
	platform  string
	model     string
}

type modelSquareEntryKey struct {
	channelID int64
	model     string
}

type modelSquareChannelModel struct {
	name        string
	channelID   int64
	channelName string
	pricing     *ChannelModelPricing
}

type modelSquareModel struct {
	name        string
	channelID   int64
	channelName string
	pricing     *ChannelModelPricing
}

// buildModelSquareChannelModels preserves every configured supported model,
// including models whose price is intentionally left unset. That makes group,
// account, and channel-pricing changes visible through a single source of
// truth in the model square.
func buildModelSquareChannelModels(channels []AvailableChannel) map[modelSquarePricingKey]modelSquareChannelModel {
	index := make(map[modelSquarePricingKey]modelSquareChannelModel)
	for _, channel := range channels {
		if channel.Status != StatusActive {
			continue
		}
		for _, group := range channel.Groups {
			for _, model := range channel.SupportedModels {
				if model.Platform != group.Platform {
					continue
				}
				key := modelSquarePricingKey{
					groupID:   group.ID,
					channelID: channel.ID,
					platform:  group.Platform,
					model:     strings.ToLower(strings.TrimSpace(model.Name)),
				}
				if _, exists := index[key]; exists {
					continue
				}
				entry := modelSquareChannelModel{
					name:        model.Name,
					channelID:   channel.ID,
					channelName: channel.Name,
				}
				if model.Pricing != nil {
					pricing := model.Pricing.Clone()
					entry.pricing = &pricing
				}
				index[key] = entry
			}
		}
	}
	return index
}

func availableGroupRefFromGroup(group Group) AvailableGroupRef {
	return AvailableGroupRef{
		ID:                 group.ID,
		Name:               group.Name,
		Platform:           group.Platform,
		SubscriptionType:   group.SubscriptionType,
		RateMultiplier:     group.RateMultiplier,
		PeakRateEnabled:    group.PeakRateEnabled,
		PeakStart:          group.PeakStart,
		PeakEnd:            group.PeakEnd,
		PeakRateMultiplier: group.PeakRateMultiplier,
		IsExclusive:        group.IsExclusive,
	}
}
