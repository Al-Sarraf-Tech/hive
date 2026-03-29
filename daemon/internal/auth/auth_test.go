package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

func testDB(t *testing.T) *bbolt.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(path)
	})
	return db
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("testpassword123")
	if err != nil {
		t.Fatal(err)
	}

	if !VerifyPassword("testpassword123", hash) {
		t.Error("valid password should verify")
	}
	if VerifyPassword("wrongpassword", hash) {
		t.Error("invalid password should not verify")
	}
	if VerifyPassword("", hash) {
		t.Error("empty password should not verify")
	}
}

func TestJWT(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")

	token, err := GenerateToken(secret, "user-1", "alice", "admin", 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatal(err)
	}

	if claims.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Errorf("expected alice, got %s", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("expected admin, got %s", claims.Role)
	}

	// Wrong secret should fail
	_, err = ValidateToken([]byte("wrong-secret"), token)
	if err != ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}

	// Expired token
	expired, _ := GenerateToken(secret, "user-1", "alice", "admin", -1*time.Hour)
	_, err = ValidateToken(secret, expired)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestSetupAndLogin(t *testing.T) {
	db := testDB(t)
	svc, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}

	// Should need setup
	needs, err := svc.NeedsSetup()
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Error("expected NeedsSetup=true")
	}

	// Setup admin
	user, err := svc.Setup("admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != RoleAdmin {
		t.Errorf("expected admin role, got %s", user.Role)
	}
	if user.Username != "admin" {
		t.Errorf("expected admin username, got %s", user.Username)
	}

	// Setup should fail now
	_, err = svc.Setup("admin2", "password456")
	if err != ErrSetupComplete {
		t.Errorf("expected ErrSetupComplete, got %v", err)
	}

	// Login
	access, refresh, err := svc.Login("admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if access == "" || refresh == "" {
		t.Error("expected non-empty tokens")
	}

	// Validate access token
	claims, err := svc.ValidateToken(access)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Username != "admin" {
		t.Errorf("expected admin, got %s", claims.Username)
	}

	// Wrong password
	_, _, err = svc.Login("admin", "wrongpassword")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// Nonexistent user (should also return ErrInvalidPassword)
	_, _, err = svc.Login("nobody", "whatever")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestUserManagement(t *testing.T) {
	db := testDB(t)
	svc, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}

	// Create users
	_, err = svc.CreateUser("alice", "password123", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateUser("bob", "password456", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}

	// Duplicate
	_, err = svc.CreateUser("alice", "password789", RoleViewer)
	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got %v", err)
	}

	// List
	users, err := svc.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}

	// Change password
	err = svc.ChangePassword("bob", "password456", "newpassword789")
	if err != nil {
		t.Fatal(err)
	}

	// Old password should fail
	_, _, err = svc.Login("bob", "password456")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword after password change")
	}

	// New password should work
	_, _, err = svc.Login("bob", "newpassword789")
	if err != nil {
		t.Fatal(err)
	}

	// Change role
	err = svc.SetRole("bob", RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	bob, err := svc.GetUser("bob")
	if err != nil {
		t.Fatal(err)
	}
	if bob.Role != RoleViewer {
		t.Errorf("expected viewer, got %s", bob.Role)
	}

	// Delete
	err = svc.DeleteUser("bob")
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.GetUser("bob")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after delete, got %v", err)
	}
}

func TestRateLimiting(t *testing.T) {
	db := testDB(t)
	svc, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.CreateUser("target", "password123", RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	// Exhaust attempts
	for i := 0; i < maxLoginAttempts; i++ {
		_, _, _ = svc.Login("target", "wrongpassword")
	}

	// Next attempt should be rate limited
	_, _, err = svc.Login("target", "password123")
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestShortPassword(t *testing.T) {
	db := testDB(t)
	svc, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.CreateUser("weak", "short", RoleViewer)
	if err == nil {
		t.Error("expected error for short password")
	}
}
