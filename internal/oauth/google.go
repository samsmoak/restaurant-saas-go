package oauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/idtoken"
)

type GooglePayload struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

func VerifyIDToken(ctx context.Context, token string) (*GooglePayload, error) {
	if token == "" {
		return nil, errors.New("id_token is required")
	}
	audienceEnv := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	if audienceEnv == "" {
		return nil, errors.New("GOOGLE_OAUTH_CLIENT_ID not configured")
	}
	var (
		payload *idtoken.Payload
		lastErr error
	)
	for _, aud := range strings.Split(audienceEnv, ",") {
		aud = strings.TrimSpace(aud)
		if aud == "" {
			continue
		}
		p, err := idtoken.Validate(ctx, token, aud)
		if err == nil {
			payload = p
			break
		}
		lastErr = err
	}
	if payload == nil {
		if lastErr == nil {
			lastErr = errors.New("no audiences configured")
		}
		return nil, fmt.Errorf("oauth.VerifyIDToken: %w", lastErr)
	}
	out := &GooglePayload{Sub: payload.Subject}
	if v, ok := payload.Claims["email"].(string); ok {
		out.Email = v
	}
	if v, ok := payload.Claims["email_verified"].(bool); ok {
		out.EmailVerified = v
	}
	if v, ok := payload.Claims["name"].(string); ok {
		out.Name = v
	}
	if v, ok := payload.Claims["picture"].(string); ok {
		out.Picture = v
	}
	return out, nil
}
