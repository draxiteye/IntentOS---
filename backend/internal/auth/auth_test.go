package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHashing(t *testing.T) {
	password := "securePassword123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Fatalf("expected password match, got error: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte("wrongPassword")); err == nil {
		t.Fatalf("expected wrong password to fail")
	}
}

func TestJWTTokenSigningAndValidation(t *testing.T) {
	secret := "test-secret-key-12345"
	userID := "user-uuid-123"

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// Validate
	parsed, err := jwt.Parse(signed, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("failed to validate token: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || claims["sub"] != userID {
		t.Fatalf("sub claim mismatch: got %v, expected %s", claims["sub"], userID)
	}
}
