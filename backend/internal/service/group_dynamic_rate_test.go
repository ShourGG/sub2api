package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateDynamicGroupRateRoundsHighestUpstreamRateUp(t *testing.T) {
	got, err := CalculateDynamicGroupRate(0.07, 1.25)
	require.NoError(t, err)
	require.InDelta(t, 0.09, got, 1e-12)
}

func TestCalculateDynamicGroupRateKeepsExactStep(t *testing.T) {
	got, err := CalculateDynamicGroupRate(0.05, 1.3)
	require.NoError(t, err)
	require.InDelta(t, 0.065, got, 1e-12)
}

func TestCalculateDynamicGroupRateRejectsInvalidMarkup(t *testing.T) {
	_, err := CalculateDynamicGroupRate(0.07, 0)
	require.Error(t, err)
}
