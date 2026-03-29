// Package auth provides user authentication with argon2id password hashing,
// HMAC-SHA256 JWT tokens, and bbolt-backed user storage for the Hive web console.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden: insufficient permissions")
	ErrRateLimited     = errors.New("too many login attempts, try again later")
	ErrSetupComplete   = errors.New("initial setup already completed")

	bucketUsers = []byte("auth_users")
	bucketMeta  = []byte("auth_meta")
)

// Role defines access levels.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// User is the stored user record.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login,omitempty"`
	Disabled     bool      `json:"disabled,omitempty"`
}

// UserInfo is the public user representation (no password hash).
type UserInfo struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin time.Time `json:"last_login,omitempty"`
	Disabled  bool      `json:"disabled,omitempty"`
}

func (u *User) Info() UserInfo {
	return UserInfo{
		ID:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		LastLogin: u.LastLogin,
		Disabled:  u.Disabled,
	}
}

// loginAttempt tracks failed login attempts for rate limiting.
type loginAttempt struct {
	count   int
	resetAt time.Time
}

// Service manages user authentication.
type Service struct {
	db        *bbolt.DB
	jwtSecret []byte

	mu       sync.RWMutex
	attempts map[string]*loginAttempt // keyed by username
}

const (
	maxLoginAttempts  = 5
	loginLockDuration = 5 * time.Minute
	tokenTTL          = 24 * time.Hour
	refreshTTL        = 7 * 24 * time.Hour
)

// New creates an auth service backed by the given bbolt database.
// If jwtSecret is empty, a random 256-bit secret is generated and persisted.
func New(db *bbolt.DB, jwtSecret string) (*Service, error) {
	// Ensure buckets exist
	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketUsers); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketMeta); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("auth: init buckets: %w", err)
	}

	secret := []byte(jwtSecret)
	if len(secret) == 0 {
		// Try to load persisted secret
		if err := db.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketMeta)
			if v := b.Get([]byte("jwt_secret")); v != nil {
				secret = make([]byte, len(v))
				copy(secret, v)
			}
			return nil
		}); err != nil {
			return nil, err
		}
		// Generate if still empty
		if len(secret) == 0 {
			secret = make([]byte, 32)
			if _, err := rand.Read(secret); err != nil {
				return nil, fmt.Errorf("auth: generate jwt secret: %w", err)
			}
			if err := db.Update(func(tx *bbolt.Tx) error {
				return tx.Bucket(bucketMeta).Put([]byte("jwt_secret"), secret)
			}); err != nil {
				return nil, fmt.Errorf("auth: persist jwt secret: %w", err)
			}
			slog.Info("auth: generated new JWT signing key")
		}
	}

	return &Service{
		db:        db,
		jwtSecret: secret,
		attempts:  make(map[string]*loginAttempt),
	}, nil
}

// NeedsSetup returns true if no users exist (first-time setup required).
func (s *Service) NeedsSetup() (bool, error) {
	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		count = b.Stats().KeyN
		return nil
	})
	return count == 0, err
}

// Setup creates the first admin user. Fails if any user already exists.
func (s *Service) Setup(username, password string) (*User, error) {
	needs, err := s.NeedsSetup()
	if err != nil {
		return nil, err
	}
	if !needs {
		return nil, ErrSetupComplete
	}
	return s.CreateUser(username, password, RoleAdmin)
}

// CreateUser creates a new user with the given role.
func (s *Service) CreateUser(username, password string, role Role) (*User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return nil, errors.New("username is required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	if role != RoleAdmin && role != RoleOperator && role != RoleViewer {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		// Check uniqueness
		if existing := b.Get([]byte(username)); existing != nil {
			return ErrUserExists
		}
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(username), data)
	})
	if err != nil {
		return nil, err
	}

	slog.Info("auth: user created", "username", username, "role", role)
	return user, nil
}

// Login validates credentials and returns a JWT token pair.
func (s *Service) Login(username, password string) (accessToken string, refreshToken string, err error) {
	username = strings.TrimSpace(strings.ToLower(username))

	// Rate limiting
	if err := s.checkRateLimit(username); err != nil {
		return "", "", err
	}

	var user User
	err = s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := b.Get([]byte(username))
		if data == nil {
			return ErrUserNotFound
		}
		return json.Unmarshal(data, &user)
	})
	if err != nil {
		s.recordFailedLogin(username)
		if errors.Is(err, ErrUserNotFound) {
			return "", "", ErrInvalidPassword // don't reveal whether user exists
		}
		return "", "", err
	}

	if user.Disabled {
		return "", "", ErrUnauthorized
	}

	if !VerifyPassword(password, user.PasswordHash) {
		s.recordFailedLogin(username)
		return "", "", ErrInvalidPassword
	}

	// Clear rate limit on success
	s.clearRateLimit(username)

	// Update last login
	user.LastLogin = time.Now().UTC()
	if err := s.putUser(&user); err != nil {
		slog.Warn("auth: failed to update last_login", "error", err)
	}

	accessToken, err = GenerateToken(s.jwtSecret, user.ID, user.Username, string(user.Role), tokenTTL)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = GenerateToken(s.jwtSecret, user.ID, user.Username, string(user.Role), refreshTTL)
	if err != nil {
		return "", "", err
	}

	slog.Info("auth: login successful", "username", username)
	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT and returns the claims.
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	return ValidateToken(s.jwtSecret, tokenStr)
}

// RefreshAccessToken issues a new access token from a valid refresh token.
func (s *Service) RefreshAccessToken(refreshTokenStr string) (string, error) {
	claims, err := s.ValidateToken(refreshTokenStr)
	if err != nil {
		return "", err
	}

	// Verify user still exists and is not disabled
	user, err := s.GetUser(claims.Username)
	if err != nil {
		return "", err
	}
	if user.Disabled {
		return "", ErrUnauthorized
	}

	return GenerateToken(s.jwtSecret, user.ID, user.Username, string(user.Role), tokenTTL)
}

// GetUser retrieves a user by username.
func (s *Service) GetUser(username string) (*User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	var user User
	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketUsers).Get([]byte(username))
		if data == nil {
			return ErrUserNotFound
		}
		return json.Unmarshal(data, &user)
	})
	return &user, err
}

// ListUsers returns all user info (no password hashes).
func (s *Service) ListUsers() ([]UserInfo, error) {
	var users []UserInfo
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketUsers).ForEach(func(k, v []byte) error {
			var u User
			if err := json.Unmarshal(v, &u); err != nil {
				return err
			}
			users = append(users, u.Info())
			return nil
		})
	})
	return users, err
}

// ChangePassword updates a user's password.
func (s *Service) ChangePassword(username, oldPassword, newPassword string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	user, err := s.GetUser(username)
	if err != nil {
		return err
	}

	if !VerifyPassword(oldPassword, user.PasswordHash) {
		return ErrInvalidPassword
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	return s.putUser(user)
}

// SetRole changes a user's role (admin only).
func (s *Service) SetRole(username string, role Role) error {
	username = strings.TrimSpace(strings.ToLower(username))
	user, err := s.GetUser(username)
	if err != nil {
		return err
	}
	user.Role = role
	return s.putUser(user)
}

// DeleteUser removes a user.
func (s *Service) DeleteUser(username string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b.Get([]byte(username)) == nil {
			return ErrUserNotFound
		}
		return b.Delete([]byte(username))
	})
}

// DisableUser toggles a user's disabled state.
func (s *Service) DisableUser(username string, disabled bool) error {
	user, err := s.GetUser(username)
	if err != nil {
		return err
	}
	user.Disabled = disabled
	return s.putUser(user)
}

// putUser persists a user record.
func (s *Service) putUser(user *User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketUsers).Put([]byte(user.Username), data)
	})
}

// Rate limiting helpers
func (s *Service) checkRateLimit(username string) error {
	s.mu.RLock()
	a, ok := s.attempts[username]
	s.mu.RUnlock()

	if !ok {
		return nil
	}
	if time.Now().After(a.resetAt) {
		s.clearRateLimit(username)
		return nil
	}
	if a.count >= maxLoginAttempts {
		return ErrRateLimited
	}
	return nil
}

func (s *Service) recordFailedLogin(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.attempts[username]
	if !ok || time.Now().After(a.resetAt) {
		s.attempts[username] = &loginAttempt{
			count:   1,
			resetAt: time.Now().Add(loginLockDuration),
		}
		return
	}
	a.count++
}

func (s *Service) clearRateLimit(username string) {
	s.mu.Lock()
	delete(s.attempts, username)
	s.mu.Unlock()
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
