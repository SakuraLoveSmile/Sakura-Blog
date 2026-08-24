package store

import (
	"path/filepath"
	"testing"
)

func TestAuthorizationTablesMigrateRolesAndPermissions(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "authz.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}

	admin := &User{Username: "admin", PasswordHash: "hash", AuthSecret: "secret", Role: RoleAdmin}
	author := &User{Username: "author", PasswordHash: "hash", AuthSecret: "secret", Role: RoleAuthor}
	reader := &User{Username: "reader", PasswordHash: "hash", AuthSecret: "secret", Role: RoleReader}
	for _, user := range []*User{admin, author, reader} {
		if err := s.CreateUser(user); err != nil {
			t.Fatalf("create %s: %v", user.Username, err)
		}
	}

	gotAuthor, err := s.GetUserByID(author.ID)
	if err != nil {
		t.Fatalf("get author: %v", err)
	}
	if len(gotAuthor.Roles) != 1 || gotAuthor.Roles[0] != RoleAuthor {
		t.Fatalf("author roles = %#v", gotAuthor.Roles)
	}
	if !HasPermission(gotAuthor.Permissions, PermissionContentWrite) || HasPermission(gotAuthor.Permissions, PermissionSystemManage) {
		t.Fatalf("author permissions = %#v", gotAuthor.Permissions)
	}

	gotAdmin, _ := s.GetUserByID(admin.ID)
	if !HasPermission(gotAdmin.Permissions, PermissionSystemManage) || !HasPermission(gotAdmin.Permissions, PermissionContentManageAny) {
		t.Fatalf("admin permissions = %#v", gotAdmin.Permissions)
	}
	gotReader, _ := s.GetUserByID(reader.ID)
	if HasPermission(gotReader.Permissions, PermissionContentWrite) || !HasPermission(gotReader.Permissions, PermissionProfileWrite) {
		t.Fatalf("reader permissions = %#v", gotReader.Permissions)
	}

	if err := s.SetUserRoles(reader.ID, []string{RoleReader, RoleAuthor}); err != nil {
		t.Fatalf("set multiple roles: %v", err)
	}
	roles, err := s.GetUserRoles(reader.ID)
	if err != nil || len(roles) != 2 {
		t.Fatalf("reader roles after update = %#v, err=%v", roles, err)
	}
}
