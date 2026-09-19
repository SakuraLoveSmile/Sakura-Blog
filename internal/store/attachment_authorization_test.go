package store

import (
	"errors"
	"testing"
)

func createOwnedAttachment(t *testing.T, s *Store, ownerID, groupID int64, name string) int64 {
	t.Helper()
	id, err := s.CreateAttachment(&Attachment{Filename: name, Path: name, URL: "/uploads/" + name, Mime: "image/png", OwnerID: ownerID, GroupID: groupID})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	return id
}

func TestMoveAttachmentsAuthorizedRejectsWholeBatch(t *testing.T) {
	s := newAttachStore(t)
	group, err := s.CreateAttachmentGroup(&AttachmentGroup{Name: "target"})
	if err != nil {
		t.Fatal(err)
	}
	mine := createOwnedAttachment(t, s, 11, 0, "mine.png")
	other := createOwnedAttachment(t, s, 22, 0, "other.png")
	legacy := createOwnedAttachment(t, s, 0, 0, "legacy.png")

	checks := []struct {
		name string
		ids  []int64
		gid  int64
		any  bool
		want error
	}{
		{"mixed owners", []int64{mine, other}, group, false, ErrAttachmentForbidden},
		{"missing object", []int64{mine, 999999}, group, true, ErrAttachmentNotFound},
		{"missing group", []int64{mine}, 999999, true, ErrAttachmentGroupNotFound},
		{"legacy author", []int64{legacy}, group, false, ErrAttachmentForbidden},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if _, err := s.MoveAttachmentsAuthorized(check.ids, check.gid, 11, check.any); !errors.Is(err, check.want) {
				t.Fatalf("error = %v, want %v", err, check.want)
			}
			for _, id := range []int64{mine, other, legacy} {
				att, _ := s.GetAttachment(id)
				if att.GroupID != 0 {
					t.Fatalf("attachment %d changed after rejected batch: group=%d", id, att.GroupID)
				}
			}
		})
	}

	if _, err := s.MoveAttachmentsAuthorized([]int64{mine, mine}, group, 11, false); err != nil {
		t.Fatalf("duplicate owned ids: %v", err)
	}
	if att, _ := s.GetAttachment(mine); att.GroupID != group {
		t.Fatalf("duplicate id move group=%d, want %d", att.GroupID, group)
	}
}

func TestMoveAttachmentsAuthorizedRollsBackOnUpdateTrigger(t *testing.T) {
	s := newAttachStore(t)
	group, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "target"})
	a := createOwnedAttachment(t, s, 11, 0, "a.png")
	b := createOwnedAttachment(t, s, 11, 0, "b.png")
	if _, err := s.DB().Exec(`CREATE TRIGGER reject_attachment_move BEFORE UPDATE OF group_id ON attachments BEGIN SELECT RAISE(ABORT, 'forced attachment failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MoveAttachmentsAuthorized([]int64{a, b}, group, 11, false); err == nil {
		t.Fatal("triggered update should fail")
	}
	for _, id := range []int64{a, b} {
		att, _ := s.GetAttachment(id)
		if att.GroupID != 0 {
			t.Fatalf("attachment %d changed after trigger rollback: group=%d", id, att.GroupID)
		}
	}
}
