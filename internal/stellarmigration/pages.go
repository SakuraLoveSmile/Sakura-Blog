package stellarmigration

// Pages 消费者的冻结边界只有 friends 的原有显示/RSS配置。
// 已保留整段原值，不因此把无消费者的 Hero 高级字段标为已消费。
func (b *themeBuilder) validatePages() {
	value, exists := get(b.raw, "pages")
	if !exists {
		return
	}
	b.checkKeys(value, "pages", "friends")
	pages := object(value)
	if friends, exists := pages["friends"]; exists {
		b.checkKeys(friends, "pages.friends", "title", "hero", "show_posts", "refresh_interval", "posts_per_link")
		config := object(friends)
		if hero, exists := config["hero"]; exists {
			b.checkKeys(hero, "pages.friends.hero", "title", "description", "background")
		}
		if enabled, exists := config["show_posts"]; exists {
			b.requireBool(enabled, "pages.friends.show_posts")
		}
	}
}
