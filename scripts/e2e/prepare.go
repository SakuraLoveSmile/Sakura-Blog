package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"blog/internal/store"
)

func main() {
	dbPath := flag.String("db", "", "temporary SQLite path")
	flag.Parse()
	if *dbPath == "" {
		panic("-db is required")
	}
	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		panic(err)
	}
	st, err := store.New(*dbPath)
	if err != nil {
		panic(err)
	}
	defer st.Close()
	if err := st.EnsureUsersTable(); err != nil {
		panic(err)
	}
	hash, err := store.HashPassword("e2e-password")
	if err != nil {
		panic(err)
	}
	user := &store.User{
		Username: "admin", PasswordHash: hash, DisplayName: "E2E Admin",
		AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAdmin,
	}
	if err := st.CreateUser(user); err != nil {
		panic(fmt.Errorf("create e2e admin: %w", err))
	}
}
