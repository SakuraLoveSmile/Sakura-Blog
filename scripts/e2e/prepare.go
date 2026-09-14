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

	// 非管理员隔离账号：User Center（/uc/）资料页、通知页与共享按钮/弹层验收使用，
	// 避免用管理员账号代替普通用户路径。
	author := &store.User{
		Username: "e2e-author", PasswordHash: hash, DisplayName: "E2E Author",
		AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAuthor,
	}
	if err := st.CreateUser(author); err != nil {
		panic(fmt.Errorf("create e2e author: %w", err))
	}
}
