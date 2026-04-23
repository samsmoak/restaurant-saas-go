package jwtutil

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func secret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

type Claims struct {
	Subject      string
	Email        string
	RestaurantID string // optional; admin tokens carry it
	Role         string // optional; owner | admin | staff
	Expires      time.Time
}

type SignOptions struct {
	UserID       string
	Email        string
	RestaurantID string
	Role         string
	TTL          time.Duration
}

func defaultTTL() time.Duration { return 7 * 24 * time.Hour }

func Sign(userID, email string) (string, error) {
	return SignWithOptions(SignOptions{UserID: userID, Email: email})
}

func SignWithOptions(opts SignOptions) (string, error) {
	s := secret()
	if len(s) == 0 {
		return "", errors.New("JWT_SECRET not set")
	}
	if opts.TTL == 0 {
		opts.TTL = defaultTTL()
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":   opts.UserID,
		"email": opts.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(opts.TTL).Unix(),
	}
	if opts.RestaurantID != "" {
		claims["restaurant_id"] = opts.RestaurantID
	}
	if opts.Role != "" {
		claims["role"] = opts.Role
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s)
	if err != nil {
		return "", fmt.Errorf("jwtutil.Sign: %w", err)
	}
	return signed, nil
}

func Parse(token string) (*Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwtutil.Parse: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	sub, _ := mc["sub"].(string)
	email, _ := mc["email"].(string)
	restaurantID, _ := mc["restaurant_id"].(string)
	role, _ := mc["role"].(string)
	c := &Claims{Subject: sub, Email: email, RestaurantID: restaurantID, Role: role}
	if expF, ok := mc["exp"].(float64); ok {
		c.Expires = time.Unix(int64(expF), 0)
	}
	return c, nil
}
