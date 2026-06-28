package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid access token")
	ErrExpiredToken = errors.New("expired access token")
)

type AccessTokenClaims struct {
	UserID    string `json:"uid"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration, now func() time.Time) *TokenService {
	if now == nil {
		now = time.Now
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}

	return &TokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        now,
	}
}

func (s *TokenService) IssueAccessToken(userID, sessionID uuid.UUID) (string, error) {
	issuedAt := s.now().UTC()
	claims := AccessTokenClaims{
		UserID:    userID.String(),
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        sessionID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(s.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *TokenService) ValidateAccessToken(tokenString string) (AccessTokenClaims, error) {
	claims := AccessTokenClaims{}
	parsedToken, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}
			return s.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return AccessTokenClaims{}, ErrExpiredToken
		}
		return AccessTokenClaims{}, ErrInvalidToken
	}
	if !parsedToken.Valid || claims.UserID == "" || claims.SessionID == "" {
		return AccessTokenClaims{}, ErrInvalidToken
	}

	return claims, nil
}

func (s *TokenService) NewRefreshToken() (plain string, hashed string, expiresAt time.Time, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", time.Time{}, err
	}

	plain = base64.RawURLEncoding.EncodeToString(raw)
	hashed = HashToken(plain)
	expiresAt = s.now().UTC().Add(s.refreshTTL)
	return plain, hashed, expiresAt, nil
}

func (s *TokenService) AccessTTLSeconds() int32 {
	return int32(s.accessTTL / time.Second)
}

func (s *TokenService) RefreshTTL() time.Duration {
	return s.refreshTTL
}

func HashToken(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
