package routes

import (
	"testing"
	"time"

	tmpl "blog/modules/stellar/viewmodel"
)

func TestTopicCardsLatestAndDetailsSplit(t *testing.T) {
	for _, tc := range []struct {
		count, visible, hidden int
	}{
		{0, 0, 0},
		{1, 0, 0},
		{3, 2, 0},
		{4, 3, 0},
		{5, 3, 1},
		{8, 3, 4},
	} {
		cards := makeTopicCards(tc.count)
		latest, rest, visible, hidden := splitTopicCards(cards)
		if (tc.count == 0) != (latest == nil) {
			t.Errorf("count %d latest nil = %v", tc.count, latest == nil)
		}
		if len(rest) != maxInt(tc.count-1, 0) || len(visible) != tc.visible || len(hidden) != tc.hidden {
			t.Errorf("count %d split = latest=%v rest=%d visible=%d hidden=%d", tc.count, latest, len(rest), len(visible), len(hidden))
		}
	}
}

func TestTopicCardsSortByPublishedDateWithDeterministicFallback(t *testing.T) {
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	cards := []*tmpl.PostCard{
		{Slug: "pinned-old", Date: old, UpdatedAt: newer, Pinned: true},
		{Slug: "new", Date: newer, UpdatedAt: old},
		{Slug: "no-date", UpdatedAt: old},
	}
	sortTopicCards(cards)
	got := []string{cards[0].Slug, cards[1].Slug, cards[2].Slug}
	want := []string{"new", "pinned-old", "no-date"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted cards = %v, want %v", got, want)
		}
	}
	if gotDate := topicItemDate(&tmpl.TopicItem{LatestPost: cards[2], UpdatedAt: newer}); !gotDate.Equal(old) {
		t.Fatalf("topic date fallback = %v, want %v", gotDate, old)
	}
}

func makeTopicCards(n int) []*tmpl.PostCard {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cards := make([]*tmpl.PostCard, 0, n)
	for i := 0; i < n; i++ {
		cards = append(cards, &tmpl.PostCard{
			Slug:  string(rune('a' + i)),
			Title: string(rune('A' + i)),
			Date:  base.AddDate(0, 0, i),
		})
	}
	return cards
}
