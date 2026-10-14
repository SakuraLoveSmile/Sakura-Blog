package store

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newInviteStore(t *testing.T) (*Store, *User) {
	t.Helper()
	s := newUserStore(t)
	if err := s.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	admin := &User{Username: "invite-admin", PasswordHash: "hash", Role: RoleAdmin}
	if err := s.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	return s, admin
}

func testInvite(t *testing.T, s *Store, admin *User) string {
	t.Helper()
	code, err := s.CreateRegistrationInvite(admin.ID, time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func inviteReader(name string) *User {
	return &User{Username: name, PasswordHash: "hash", AuthSecret: GenerateAuthSecret(), Role: RoleAdmin, Roles: []string{RoleAdmin}, Permissions: []string{"*"}}
}

func TestRegistrationInviteHashAndMetadata(t *testing.T) {
	s, admin := newInviteStore(t)
	code := testInvite(t, s, admin)
	decoded, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil || len(decoded) < 24 {
		t.Fatalf("invite entropy = %d, err=%v", len(decoded), err)
	}
	var hash string
	var createdBy, createdAt, expiresAt int64
	var usedAt, usedBy sql.NullInt64
	if err := s.db.QueryRow(`SELECT code_hash, created_by, created_at, expires_at, used_at, used_by FROM registration_invites`).Scan(&hash, &createdBy, &createdAt, &expiresAt, &usedAt, &usedBy); err != nil {
		t.Fatal(err)
	}
	if hash != registrationInviteHash(code) || hash == code || createdBy != admin.ID || usedAt.Valid || usedBy.Valid {
		t.Fatalf("invalid invite metadata")
	}
	if expiresAt-createdAt < (7*24*time.Hour - time.Second).Milliseconds() {
		t.Fatalf("wrong expiry interval")
	}
	rows, err := s.db.Query(`PRAGMA table_info(registration_invites)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "code" || name == "token" {
			t.Fatalf("plaintext column: %s", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrationInviteAtomicReaderAndReuse(t *testing.T) {
	s, admin := newInviteStore(t)
	code := testInvite(t, s, admin)
	u := inviteReader("invited")
	if err := s.CreateUserWithRegistrationInvite(u, " "+code+" "); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetUserByID(u.ID)
	if err != nil || stored == nil || stored.Role != RoleReader || len(stored.Roles) != 1 || stored.Roles[0] != RoleReader || HasPermission(stored.Permissions, PermissionUsersManage) {
		t.Fatalf("registration role: %+v / %v", stored, err)
	}
	var usedBy, usedAt int64
	if err := s.db.QueryRow(`SELECT used_by, used_at FROM registration_invites WHERE code_hash = ?`, registrationInviteHash(code)).Scan(&usedBy, &usedAt); err != nil || usedBy != u.ID || usedAt == 0 {
		t.Fatalf("usage metadata: %d %d %v", usedBy, usedAt, err)
	}
	if err := s.CreateUserWithRegistrationInvite(inviteReader("reuse"), code); !errors.Is(err, ErrInvalidRegistrationInvite) {
		t.Fatalf("reuse = %v", err)
	}
}

func TestRegistrationInviteInvalidAndExpiry(t *testing.T) {
	s, admin := newInviteStore(t)
	for _, code := range []string{"", "wrong"} {
		if err := s.CreateUserWithRegistrationInvite(inviteReader("invalid"), code); !errors.Is(err, ErrInvalidRegistrationInvite) {
			t.Fatalf("invalid invite = %v", err)
		}
	}
	code := testInvite(t, s, admin)
	if _, err := s.db.Exec(`UPDATE registration_invites SET expires_at = ?`, time.Now().Add(-time.Second).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateUserWithRegistrationInvite(inviteReader("expired"), code); !errors.Is(err, ErrInvalidRegistrationInvite) {
		t.Fatalf("expired = %v", err)
	}
	if code, err := s.CreateRegistrationInvite(admin.ID, time.Now().Add(-time.Second)); err == nil || code != "" {
		t.Fatalf("past expiry accepted")
	}
	reader := inviteReader("creator-reader")
	reader.Role = RoleReader
	if err := s.CreateUser(reader); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{0, reader.ID} {
		if code, err := s.CreateRegistrationInvite(id, time.Now().Add(time.Hour)); err == nil || code != "" {
			t.Fatalf("non-admin creator accepted: %d", id)
		}
	}
	if err := s.UpdateUserStatus(admin.ID, UserStatusBanned); err != nil {
		t.Fatal(err)
	}
	if code, err := s.CreateRegistrationInvite(admin.ID, time.Now().Add(time.Hour)); err == nil || code != "" {
		t.Fatalf("banned creator accepted")
	}
}

func TestRegistrationInviteRollback(t *testing.T) {
	for _, failure := range []string{"duplicate", "role", "metadata"} {
		t.Run(failure, func(t *testing.T) {
			s, admin := newInviteStore(t)
			code := testInvite(t, s, admin)
			if failure == "duplicate" {
				if err := s.CreateUser(inviteReader("taken")); err != nil {
					t.Fatal(err)
				}
			} else {
				statement := `CREATE TRIGGER fail_registration BEFORE INSERT ON user_roles WHEN NEW.role = 'reader' BEGIN SELECT RAISE(ABORT, 'test role failure'); END;`
				if failure == "metadata" {
					statement = `CREATE TRIGGER fail_registration BEFORE UPDATE OF used_by ON registration_invites BEGIN SELECT RAISE(ABORT, 'test metadata failure'); END;`
				}
				if _, err := s.db.Exec(statement); err != nil {
					t.Fatal(err)
				}
			}
			u := inviteReader("taken")
			if err := s.CreateUserWithRegistrationInvite(u, code); err == nil {
				t.Fatal("expected failure")
			}
			if u.ID != 0 {
				t.Fatal("failed registration mutated user id")
			}
			var usedAt sql.NullInt64
			if err := s.db.QueryRow(`SELECT used_at FROM registration_invites WHERE code_hash = ?`, registrationInviteHash(code)).Scan(&usedAt); err != nil || usedAt.Valid {
				t.Fatalf("failure consumed invite: %v %v", usedAt, err)
			}
			if failure != "duplicate" {
				if user, err := s.GetUserByUsername("taken"); err != nil || user != nil {
					t.Fatalf("partial user: %v %v", user, err)
				}
				if _, err := s.db.Exec(`DROP TRIGGER fail_registration`); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.CreateUserWithRegistrationInvite(inviteReader("retry"), code); err != nil {
				t.Fatalf("retry = %v", err)
			}
		})
	}
}

func TestRegistrationInviteConcurrentAcrossConnections(t *testing.T) {
	s, admin := newInviteStore(t)
	code := testInvite(t, s, admin)
	var path string
	var seq int
	var name string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	other, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	const count = 12
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			st := s
			if i%2 == 1 {
				st = other
			}
			results <- st.CreateUserWithRegistrationInvite(inviteReader(fmt.Sprintf("race-%d", i)), code)
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrInvalidRegistrationInvite) {
			t.Fatalf("unexpected concurrency error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful registrations = %d", successes)
	}
	var users, roles int
	if err := s.db.QueryRow(`SELECT count(*) FROM users WHERE username LIKE 'race-%'`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM user_roles WHERE role = 'reader'`).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if users != 1 || roles != 1 {
		t.Fatalf("users=%d roles=%d", users, roles)
	}
}

func TestRegistrationInviteSchemaExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	admin := &User{Username: "existing-admin", PasswordHash: "keep-hash", Role: RoleAdmin}
	if err := s.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRegistrationInvite(admin.ID, time.Now().Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "registration_invites") {
		t.Fatalf("store unexpectedly migrated schema: %v", err)
	}
	for range 2 {
		if err := s.EnsureSchema(); err != nil {
			t.Fatal(err)
		}
	}
	code := testInvite(t, s, admin)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateUserWithRegistrationInvite(inviteReader("after-restart"), code); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetUserByID(admin.ID)
	if err != nil || stored == nil || stored.PasswordHash != "keep-hash" || stored.Role != RoleAdmin {
		t.Fatalf("existing user changed: %+v %v", stored, err)
	}
}

func TestRegistrationInviteExpiresWhileWaitingForWriter(t *testing.T) {
	s, admin := newInviteStore(t)
	code := testInvite(t, s, admin)
	var seq int
	var name, path string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	other, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	expires := time.Now().Add(200 * time.Millisecond)
	if _, err := tx.Exec(`UPDATE registration_invites SET expires_at = ? WHERE code_hash = ?`, expires.UnixMilli(), registrationInviteHash(code)); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- other.CreateUserWithRegistrationInvite(inviteReader("wait-expired"), code) }()
	time.Sleep(time.Until(expires) + 50*time.Millisecond)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, ErrInvalidRegistrationInvite) {
		t.Fatalf("invite expired behind writer lock: %v", err)
	}
}

func TestRegistrationInviteAuditSurvivesUserDeletion(t *testing.T) {
	s, admin := newInviteStore(t)
	code := testInvite(t, s, admin)
	reader := inviteReader("deleted-reader")
	if err := s.CreateUserWithRegistrationInvite(reader, code); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(reader.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(admin.ID); err != nil {
		t.Fatal(err)
	}
	var createdBy, usedBy, usedAt int64
	if err := s.db.QueryRow(`SELECT created_by, used_by, used_at FROM registration_invites WHERE code_hash = ?`, registrationInviteHash(code)).Scan(&createdBy, &usedBy, &usedAt); err != nil || createdBy != admin.ID || usedBy != reader.ID || usedAt == 0 {
		t.Fatalf("audit lost: %d %d %d %v", createdBy, usedBy, usedAt, err)
	}
	if err := s.CreateUserWithRegistrationInvite(inviteReader("cannot-reuse"), code); !errors.Is(err, ErrInvalidRegistrationInvite) {
		t.Fatalf("deleted user revived invite: %v", err)
	}
}
