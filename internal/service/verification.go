package service

import (
	"fmt"
	"net/url"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// validateVerificationMedia is a defense-in-depth gate around the same
// rules the HTTP handlers enforce: 0 or at least 2 photo URLs, every URL
// http(s). Service callers go through here so that if a new caller ever
// skips the handler-side validation (CLI tool, future API, internal
// scheduler, etc.) we still refuse non-http(s) values that would render
// as <a href> in the admin tab.
func validateVerificationMedia(deviceType domain.DeviceType, photos []string, video string) *domain.AppError {
	required := domain.RequiredVerificationPhotos(deviceType)
	if len(photos) > 0 && len(photos) < required {
		return domain.ValidationFailed("verification photo count is insufficient", map[string]string{
			"photos": fmt.Sprintf("%d photo URL(s) are required for this device type", required),
		})
	}
	for _, p := range photos {
		if !isSafeHTTPVerificationURL(p) {
			return domain.ValidationFailed("verification photo URLs must be http(s)", map[string]string{
				"photos": "only http(s) URLs are accepted",
			})
		}
	}
	if video != "" && !isSafeHTTPVerificationURL(video) {
		return domain.ValidationFailed("verification video URL must be http(s)", map[string]string{
			"video": "only http(s) URLs are accepted",
		})
	}
	return nil
}

func isSafeHTTPVerificationURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
