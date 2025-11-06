package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken is returned when the provided token is invalid.
	ErrInvalidToken = fmt.Errorf("invalid token")
)

type service struct {
	secret []byte
}

func NewAuthService(secret string) (*service, error) {
	return &service{
		secret: []byte(secret),
	}, nil
}

func (s *service) ValidateToken(_ context.Context, token string) (string, error) {
	t, err := jwt.Parse(strings.Split(token, " ")[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return "", errors.Join(ErrInvalidToken, err)
	}

	// read claims from payload and extract the user ID.
	if claims, ok := t.Claims.(jwt.MapClaims); ok && t.Valid {
		id, ok := claims["sub"].(string)
		if !ok {
			return "", fmt.Errorf("%w: failed to extract id from claims", ErrInvalidToken)
		}

		return id, nil
	}

	return "", ErrInvalidToken
}
