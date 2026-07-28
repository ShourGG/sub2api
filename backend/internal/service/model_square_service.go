package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

// ModelSquareEntry is one routable model in one active group. A model is kept
// separate for every group because its effective price and available accounts
// may differ even when the model name is the same.
type ModelSquareEntry struct {
	Name         string
	Platform     string
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
	now            func() time.Time
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
		now:            time.Now,
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

	now := s.now()
	entries := make([]ModelSquareEntry, 0)
	for i := range groups {
		group := groups[i]
		accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, group.ID)
		if err != nil {
			return nil, err
		}

		models := make(map[string]modelSquareModel)
		for key, channelModel := range channelModels {
			if key.groupID != group.ID || key.platform != group.Platform {
				continue
			}
			models[key.model] = modelSquareModel{
				name:    channelModel.name,
				pricing: channelModel.pricing,
			}
		}
		for _, account := range accounts {
			for model := range account.GetModelMapping() {
				name := strings.TrimSpace(model)
				if name == "" {
					continue
				}
				key := strings.ToLower(name)
				if _, exists := models[key]; !exists {
					models[key] = modelSquareModel{name: name}
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
				Group:        ref,
				AccountCount: accountCount,
				Pricing:      scaleModelSquarePricing(model.pricing, group.RateMultiplier*group.PeakMultiplierAt(now)),
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
		return entries[i].Group.Name < entries[j].Group.Name
	})
	return entries, nil
}

type modelSquarePricingKey struct {
	groupID  int64
	platform string
	model    string
}

type modelSquareChannelModel struct {
	name    string
	pricing *ChannelModelPricing
}

type modelSquareModel struct {
	name    string
	pricing *ChannelModelPricing
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
					groupID:  group.ID,
					platform: group.Platform,
					model:    strings.ToLower(strings.TrimSpace(model.Name)),
				}
				if _, exists := index[key]; exists {
					continue
				}
				entry := modelSquareChannelModel{name: model.Name}
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

func scaleModelSquarePricing(pricing *ChannelModelPricing, multiplier float64) *ChannelModelPricing {
	if pricing == nil {
		return nil
	}
	copy := pricing.Clone()
	copy.InputPrice = scaleModelSquarePrice(copy.InputPrice, multiplier)
	copy.OutputPrice = scaleModelSquarePrice(copy.OutputPrice, multiplier)
	copy.CacheWritePrice = scaleModelSquarePrice(copy.CacheWritePrice, multiplier)
	copy.CacheReadPrice = scaleModelSquarePrice(copy.CacheReadPrice, multiplier)
	copy.ImageInputPrice = scaleModelSquarePrice(copy.ImageInputPrice, multiplier)
	copy.ImageOutputPrice = scaleModelSquarePrice(copy.ImageOutputPrice, multiplier)
	copy.PerRequestPrice = scaleModelSquarePrice(copy.PerRequestPrice, multiplier)
	for i := range copy.Intervals {
		copy.Intervals[i].InputPrice = scaleModelSquarePrice(copy.Intervals[i].InputPrice, multiplier)
		copy.Intervals[i].OutputPrice = scaleModelSquarePrice(copy.Intervals[i].OutputPrice, multiplier)
		copy.Intervals[i].CacheWritePrice = scaleModelSquarePrice(copy.Intervals[i].CacheWritePrice, multiplier)
		copy.Intervals[i].CacheReadPrice = scaleModelSquarePrice(copy.Intervals[i].CacheReadPrice, multiplier)
		copy.Intervals[i].PerRequestPrice = scaleModelSquarePrice(copy.Intervals[i].PerRequestPrice, multiplier)
	}
	return &copy
}

func scaleModelSquarePrice(value *float64, multiplier float64) *float64 {
	if value == nil {
		return nil
	}
	result := *value * multiplier
	return &result
}
