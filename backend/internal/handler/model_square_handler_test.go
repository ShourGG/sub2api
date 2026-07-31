//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFilterModelSquareVisibleEntries_HidesAllExclusiveGroups(t *testing.T) {
	entries := []service.ModelSquareEntry{
		{Name: "public", Group: service.AvailableGroupRef{ID: 1, IsExclusive: false}},
		{Name: "granted", Group: service.AvailableGroupRef{ID: 2, IsExclusive: true}},
		{Name: "private", Group: service.AvailableGroupRef{ID: 3, IsExclusive: true}},
	}

	visible := filterModelSquareVisibleEntries(entries)
	require.Len(t, visible, 1)
	require.Equal(t, "public", visible[0].Name)
}
