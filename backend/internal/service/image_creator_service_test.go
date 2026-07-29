package service

import "testing"

func TestNormalizeImageCreatorRequestControls(t *testing.T) {
	t.Parallel()

	if got := normalizeImageCreatorRequestedOutputFormat("png"); got != "png" {
		t.Fatalf("png format = %q, want png", got)
	}
	if got := normalizeImageCreatorRequestedOutputFormat("jpeg"); got != "jpeg" {
		t.Fatalf("jpeg format = %q, want jpeg", got)
	}
	if got := normalizeImageCreatorRequestedOutputFormat("webp"); got != "webp" {
		t.Fatalf("webp format = %q, want webp", got)
	}
	if got := normalizeImageCreatorQuality("standard"); got != "medium" {
		t.Fatalf("standard quality = %q, want medium", got)
	}
	if got := normalizeImageCreatorQuality("high"); got != "high" {
		t.Fatalf("high quality = %q, want high", got)
	}
}

func TestValidateImageCreatorSize(t *testing.T) {
	t.Parallel()

	for _, size := range []string{"", "auto", "1024x1024", "1536x1024", "1024x1536", "3072x1024"} {
		if err := validateImageCreatorSize(size); err != nil {
			t.Fatalf("size %q rejected: %v", size, err)
		}
	}
	for _, size := range []string{"1024x341", "4096x1024", "1024x1024x1"} {
		if err := validateImageCreatorSize(size); err == nil {
			t.Fatalf("size %q accepted", size)
		}
	}
}
