package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskLeaderboardIdentity(t *testing.T) {
	t.Parallel()

	require.Equal(t, "a***e", MaskLeaderboardIdentity("alice", "alice@example.com"))
	require.Equal(t, "甲***", MaskLeaderboardIdentity("甲", "alice@example.com"))
	require.Equal(t, "a***@***", MaskLeaderboardIdentity("", "alice@example.com"))
	require.Equal(t, "***", MaskLeaderboardIdentity("", ""))
}
