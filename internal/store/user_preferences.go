package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UserPreference stores one user-scoped JSON preference group.
// user_id=0 is the compatibility bucket used when console authentication is disabled.
type UserPreference struct {
	UserID          int64
	PreferenceGroup string
	Value           string
	UpdatedAt       time.Time
}

// EnsureUserPreferencesTable creates the user preference table idempotently.
func (s *Store) EnsureUserPreferencesTable() error {
	return s.ensureTable("user_preferences")
}

// GetUserPreference returns nil when the user has not saved this group yet.
func (s *Store) GetUserPreference(userID int64, group string) (*UserPreference, error) {
	if userID < 0 {
		return nil, fmt.Errorf("invalid user id %d", userID)
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, fmt.Errorf("preference group is empty")
	}

	p := &UserPreference{UserID: userID, PreferenceGroup: group}
	err := s.db.QueryRow(`
		SELECT value, updated_at
		FROM user_preferences
		WHERE user_id = ? AND preference_group = ?`, userID, group).Scan(&p.Value, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user preference %d/%s: %w", userID, group, err)
	}
	return p, nil
}

// PutUserPreference upserts one user-scoped JSON preference group.
func (s *Store) PutUserPreference(userID int64, group, value string) error {
	if userID < 0 {
		return fmt.Errorf("invalid user id %d", userID)
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return fmt.Errorf("preference group is empty")
	}
	if _, err := s.db.Exec(`
		INSERT INTO user_preferences (user_id, preference_group, value, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, preference_group) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP`, userID, group, value); err != nil {
		return fmt.Errorf("put user preference %d/%s: %w", userID, group, err)
	}
	return nil
}
