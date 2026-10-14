package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

func recoveryTestUser(t *testing.T, s *Store) *User {
	t.Helper()
	u := &User{Username: "recovery-reader", PasswordHash: "verified-password-hash", AuthSecret: GenerateAuthSecret(), Role: RoleReader}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTOTPSecret(u.ID, "JBSWY3DPEHPK3PXP", []string{"12ab34cd", "98ef76ab"}); err != nil {
		t.Fatal(err)
	}
	u, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func recoveryTestCodes(t *testing.T, s *Store, id int64) []string {
	t.Helper()
	u, err := s.GetUserByID(id)
	if err != nil || u == nil {
		t.Fatalf("get recovery user: %v", err)
	}
	var codes []string
	if err := json.Unmarshal([]byte(u.TOTPRecoveryCodes), &codes); err != nil {
		t.Fatal(err)
	}
	return codes
}

func TestReaderRecoveryCodeOneTimeAndStrict(t *testing.T) {
	s := newUserStore(t)
	u := recoveryTestUser(t, s)
	for _, code := range []string{"", "wrong123", "12AB34CD", " 12ab34cd", "12ab34cd ", "12ab34c"} {
		if ok, err := s.ConsumeReaderRecoveryCode(u, code); err != nil || ok {
			t.Fatalf("wrong code accepted: %v %v", ok, err)
		}
	}
	if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 2 {
		t.Fatal("wrong code consumed recovery list")
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || !ok {
		t.Fatalf("recovery = %v %v", ok, err)
	}
	if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 1 || codes[0] != "98ef76ab" {
		t.Fatalf("remaining codes = %v", codes)
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || ok {
		t.Fatalf("reused code = %v %v", ok, err)
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "98ef76ab"); err != nil || !ok {
		t.Fatalf("last recovery = %v %v", ok, err)
	}
	if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 0 {
		t.Fatalf("final codes = %v", codes)
	}
}

func TestReaderRecoveryCodeBindsCurrentAccount(t *testing.T) {
	for _, change := range []string{"password", "auth-secret", "totp-secret", "disabled", "banned", "role", "role-association", "codes"} {
		t.Run(change, func(t *testing.T) {
			s := newUserStore(t)
			u := recoveryTestUser(t, s)
			var err error
			switch change {
			case "password":
				err = s.UpdatePassword(u.ID, "new-password-hash", u.AuthSecret)
			case "auth-secret":
				err = s.UpdatePassword(u.ID, u.PasswordHash, GenerateAuthSecret())
			case "totp-secret":
				err = s.UpdateTOTPSecret(u.ID, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", []string{"12ab34cd", "98ef76ab"})
			case "disabled":
				err = s.UpdateTOTPSecret(u.ID, "", []string{"12ab34cd", "98ef76ab"})
			case "banned":
				err = s.UpdateUserStatus(u.ID, UserStatusBanned)
			case "role":
				err = s.SetUserRoles(u.ID, []string{RoleAdmin})
			case "role-association":
				_, err = s.db.Exec(`DELETE FROM user_roles WHERE user_id = ?`, u.ID)
			case "codes":
				err = s.UpdateTOTPSecret(u.ID, u.TOTPSecret, []string{"newcode1", "98ef76ab"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || ok {
				t.Fatalf("changed account accepted recovery: %v %v", ok, err)
			}
			if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 2 {
				t.Fatalf("changed account consumed codes: %v", codes)
			}
		})
	}
}

func TestReaderRecoveryCodeWriteFailureDoesNotConsume(t *testing.T) {
	s := newUserStore(t)
	u := recoveryTestUser(t, s)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_recovery BEFORE UPDATE OF totp_recovery_codes ON users BEGIN SELECT RAISE(ABORT, 'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err == nil || ok {
		t.Fatalf("write failure = %v %v", ok, err)
	}
	if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 2 {
		t.Fatal("write failure consumed recovery code")
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_recovery`); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || !ok {
		t.Fatalf("retry = %v %v", ok, err)
	}
}

func TestReaderRecoveryCodeConcurrentAcrossConnections(t *testing.T) {
	s := newUserStore(t)
	u := recoveryTestUser(t, s)
	other, err := New(s.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	start := make(chan struct{})
	results := make(chan bool, 12)
	errors := make(chan error, 12)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			st := s
			if i%2 == 1 {
				st = other
			}
			ok, err := st.ConsumeReaderRecoveryCode(u, "12ab34cd")
			results <- ok
			errors <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)
	successes := 0
	for ok := range results {
		if ok {
			successes++
		}
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful recovery logins = %d", successes)
	}
	if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 1 || codes[0] != "98ef76ab" {
		t.Fatalf("remaining codes = %v", codes)
	}
}

func TestReaderRecoveryCodeMalformedListFailsClosed(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `"12ab34cd"`, `[123]`, `broken`} {
		t.Run(fmt.Sprintf("json=%s", raw), func(t *testing.T) {
			s := newUserStore(t)
			u := recoveryTestUser(t, s)
			if _, err := s.db.Exec(`UPDATE users SET totp_recovery_codes = ? WHERE id = ?`, raw, u.ID); err != nil {
				t.Fatal(err)
			}
			if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || ok {
				t.Fatalf("malformed list accepted: %v %v", ok, err)
			}
			var retained string
			if err := s.db.QueryRow(`SELECT totp_recovery_codes FROM users WHERE id = ?`, u.ID).Scan(&retained); err != nil || retained != raw {
				t.Fatalf("invalid list overwritten: %v", err)
			}
		})
	}
}

func TestReaderRecoveryCodeDuplicateCannotBeReused(t *testing.T) {
	s := newUserStore(t)
	u := recoveryTestUser(t, s)
	if err := s.UpdateTOTPSecret(u.ID, u.TOTPSecret, []string{"12ab34cd", "98ef76ab", "12ab34cd"}); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || !ok {
		t.Fatalf("duplicate-list consume = %v %v", ok, err)
	}
	if ok, err := s.ConsumeReaderRecoveryCode(u, "12ab34cd"); err != nil || ok {
		t.Fatalf("duplicate-list reuse = %v %v", ok, err)
	}
	if codes := recoveryTestCodes(t, s, u.ID); len(codes) != 1 || codes[0] != "98ef76ab" {
		t.Fatalf("duplicate-list remaining = %v", codes)
	}
}
