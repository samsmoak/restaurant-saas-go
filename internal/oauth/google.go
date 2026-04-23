package oauth

import (
	"context"
	"errors"
	"fmt"
	"os"

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
	audience := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	if audience == "" {
		return nil, errors.New("GOOGLE_OAUTH_CLIENT_ID not configured")
	}
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return nil, fmt.Errorf("oauth.VerifyIDToken: %w", err)
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
