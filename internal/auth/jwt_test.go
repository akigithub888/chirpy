package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

const testSecret = "test-secret"

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()

	token, err := MakeJWT(userID, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	gotID, err := ValidateJWT(token, testSecret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if gotID != userID {
		t.Errorf("expected userID %v, got %v", userID, gotID)
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()

	token, err := MakeJWT(userID, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	_, err = ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Fatal("expected error when validating token with wrong secret")
	}
}

func TestValidateJWTExpired(t *testing.T) {
	userID := uuid.New()

	// Token already expired
	token, err := MakeJWT(userID, testSecret, -time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	_, err = ValidateJWT(token, testSecret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
