package backup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	RestorePreparing        = "preparing"
	RestoreWaiting          = "waiting"
	RestoreApplying         = "applying"
	RestoreRebuilding       = "rebuilding"
	RestoreSucceeded        = "succeeded"
	RestoreRolledBack       = "rolled_back"
	RestoreRecoveryRequired = "recovery_required"
)

var (
	ErrRestoreConflict  = errors.New("restore conflict")
	ErrRestoreNotFound  = errors.New("restore task not found")
	ErrRecoveryRequired = errors.New("restore recovery required")
)

// RestoreTask 只包含管理员界面所需的任务状态，文件路径不进入响应。
type RestoreTask struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	HasPreimage bool      `json:"has_preimage"`
}

type RestoreInput struct {
	Data                    []byte
	Snapshot                string
	Strict                  bool
	IdempotencyKey          string
	ContentImport           bool
	ContentMergeDefinitions bool
}

// restoreJournal 的前像路径均由任务 ID 派生，不信任磁盘 JSON 提供任意目标路径。
type restoreJournal struct {
	Version int `json:"version"`
	RestoreTask
	IdempotencyKey          string `json:"idempotency_key,omitempty"`
	ContentHash             string `json:"content_hash"`
	Strict                  bool   `json:"strict"`
	DatabaseOnly            bool   `json:"database_only"`
	ContentImport           bool   `json:"content_import,omitempty"`
	ContentMergeDefinitions bool   `json:"content_merge_definitions,omitempty"`
	RestoreSite             bool   `json:"restore_site"`
	UploadsExisted          bool   `json:"uploads_existed"`
	SiteExisted             bool   `json:"site_existed"`
	SiteMode                uint32 `json:"site_mode"`
	PreimageReady           bool   `json:"preimage_ready"`
	Step                    string `json:"step"`
}

func (s *Service) SetRestoreHook(fn func())                   { s.restoreHook = fn }
func (s *Service) SetRestoreRebuild(fn func() error)          { s.restoreRebuild = fn }
func (s *Service) SetOperationLock(fn func() (func(), error)) { s.operationLock = fn }

func validRestoreID(id string) bool {
	if len(id) != 40 || !strings.HasPrefix(id, "restore-") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(id, "restore-"))
	return err == nil
}
func newRestoreID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate restore ID: %w", err)
	}
	return "restore-" + hex.EncodeToString(raw), nil
}
func (s *Service) taskRoot(id string) string { return filepath.Join(s.backupDir, "restore-tasks", id) }
func (s *Service) journalPath(id string) string {
	return filepath.Join(s.backupDir, ".restore-"+id+".json")
}
func (s *Service) beforePath(id, name string) string {
	return filepath.Join(s.taskRoot(id), "preimage", name)
}
func (s *Service) inputPath(id, name string) string {
	return filepath.Join(s.taskRoot(id), "input", name)
}
func (s *Service) uploadStage(id, suffix string) string {
	return filepath.Join(filepath.Dir(s.uploadDir), ".blog-"+id+"-"+suffix)
}

func (s *Service) readJournal(id string) (restoreJournal, error) {
	var journal restoreJournal
	if !validRestoreID(id) {
		return journal, ErrRestoreNotFound
	}
	data, err := readRegularFile(s.journalPath(id), 1<<20)
	if os.IsNotExist(err) {
		return journal, ErrRestoreNotFound
	}
	if err != nil {
		return journal, err
	}
	if err := json.Unmarshal(data, &journal); err != nil {
		return journal, fmt.Errorf("decode restore journal: %w", err)
	}
	if journal.Version != 1 || journal.ID != id {
		return journal, fmt.Errorf("unsupported restore journal version or ID")
	}
	switch journal.Status {
	case RestorePreparing, RestoreWaiting, RestoreApplying, RestoreRebuilding, RestoreSucceeded, RestoreRolledBack, RestoreRecoveryRequired:
	default:
		return journal, fmt.Errorf("invalid restore journal status")
	}
	return journal, nil
}
func (s *Service) writeJournal(journal *restoreJournal) error {
	journal.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if s.restoreJournalWrite != nil {
		return s.restoreJournalWrite(s.journalPath(journal.ID), data)
	}
	return writeAtomicFile(s.journalPath(journal.ID), data, 0600)
}
func (s *Service) journals() ([]restoreJournal, error) {
	entries, err := os.ReadDir(s.backupDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var journals []restoreJournal
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".restore-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), ".restore-"), ".json")
		journal, err := s.readJournal(id)
		if err != nil {
			return nil, err
		}
		journals = append(journals, journal)
	}
	sort.Slice(journals, func(i, j int) bool { return journals[i].CreatedAt.After(journals[j].CreatedAt) })
	return journals, nil
}
func (s *Service) GetRestoreTask(id string) (RestoreTask, error) {
	j, err := s.readJournal(id)
	return s.visibleTask(j.RestoreTask), err
}
func (s *Service) ListRestoreTasks() ([]RestoreTask, error) {
	journals, err := s.journals()
	if err != nil {
		return nil, err
	}
	tasks := make([]RestoreTask, 0, len(journals))
	for _, j := range journals {
		tasks = append(tasks, s.visibleTask(j.RestoreTask))
	}
	return tasks, nil
}

// StartRestore 完成源预检和持久化后才返回；任务提交不继承 HTTP 断连取消信号。
func (s *Service) StartRestore(ctx context.Context, input RestoreInput) (RestoreTask, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if (len(input.Data) == 0) == (input.Snapshot == "") {
		return RestoreTask{}, fmt.Errorf("exactly one restore source is required")
	}
	if len(input.IdempotencyKey) > 128 {
		return RestoreTask{}, fmt.Errorf("idempotency key too long")
	}
	data, dbOnly, err := s.restoreSource(input)
	if err != nil {
		return RestoreTask{}, err
	}
	if input.ContentImport {
		if input.Snapshot != "" || !hasStellarContentManifest(data) {
			return RestoreTask{}, fmt.Errorf("content import requires a portable content archive")
		}
		dbOnly = true
	}
	if input.ContentMergeDefinitions && !input.ContentImport {
		return RestoreTask{}, fmt.Errorf("content merge requires a content import")
	}
	hash := sha256.New()
	_, _ = hash.Write(data)
	_, _ = fmt.Fprintf(hash, "\x00%t\x00%t", input.Strict, dbOnly)
	if input.ContentImport {
		_, _ = hash.Write([]byte("\x00content-import-v1"))
		if input.ContentMergeDefinitions {
			_, _ = hash.Write([]byte("\x00preserve-definitions"))
		}
	}
	fingerprint := hex.EncodeToString(hash.Sum(nil))
	keyHash := ""
	if input.IdempotencyKey != "" {
		key := sha256.Sum256([]byte(input.IdempotencyKey))
		keyHash = hex.EncodeToString(key[:])
	}
	s.restoreMu.Lock()
	journals, err := s.journals()
	if err != nil {
		s.restoreMu.Unlock()
		return RestoreTask{}, err
	}
	for _, j := range journals {
		if keyHash != "" && j.IdempotencyKey == keyHash {
			s.restoreMu.Unlock()
			if j.ContentHash != fingerprint {
				return RestoreTask{}, ErrRestoreConflict
			}
			return j.RestoreTask, nil
		}
	}
	for _, j := range journals {
		if j.Status != RestoreSucceeded && j.Status != RestoreRolledBack {
			s.restoreMu.Unlock()
			return RestoreTask{}, ErrRestoreConflict
		}
	}
	if s.activeRestore != "" || s.store.Maintenance().Busy() {
		s.restoreMu.Unlock()
		return RestoreTask{}, ErrRestoreConflict
	}
	id, err := newRestoreID()
	if err != nil {
		s.restoreMu.Unlock()
		return RestoreTask{}, err
	}
	s.activeRestore = id
	s.restoreMu.Unlock()
	finishActive := func() { s.restoreMu.Lock(); s.activeRestore = ""; s.restoreMu.Unlock() }
	release, err := s.store.Maintenance().Acquire()
	if err != nil {
		finishActive()
		return RestoreTask{}, ErrRestoreConflict
	}
	journal := restoreJournal{Version: 1, RestoreTask: RestoreTask{ID: id, Status: RestorePreparing, CreatedAt: time.Now().UTC()}, IdempotencyKey: keyHash, ContentHash: fingerprint, Strict: input.Strict, DatabaseOnly: dbOnly, ContentImport: input.ContentImport, ContentMergeDefinitions: input.ContentMergeDefinitions}
	if err := os.MkdirAll(filepath.Join(s.taskRoot(id), "input"), 0700); err != nil {
		release()
		finishActive()
		return RestoreTask{}, err
	}
	if err := s.writeJournal(&journal); err != nil {
		release()
		finishActive()
		return RestoreTask{}, err
	}
	err = s.prepareRestore(ctx, &journal, data)
	if err != nil {
		journal.Status = RestoreRolledBack
		journal.Error = "预检失败，原数据未改变"
		writeErr := s.writeJournal(&journal)
		removeErr := os.RemoveAll(s.taskRoot(id))
		release()
		finishActive()
		return RestoreTask{}, errors.Join(err, writeErr, removeErr)
	}
	journal.Status = RestoreWaiting
	if err := s.writeJournal(&journal); err != nil {
		release()
		finishActive()
		return RestoreTask{}, err
	}
	release()
	task := journal.RestoreTask
	go func() { defer finishActive(); s.executeRestore(journal) }()
	return task, nil
}

func (s *Service) Restore(data []byte, strict bool) error {
	task, err := s.StartRestore(context.Background(), RestoreInput{Data: data, Strict: strict})
	if err != nil {
		return err
	}
	return s.WaitRestore(context.Background(), task.ID)
}
func (s *Service) RestoreSnapshot(name string) error {
	task, err := s.StartRestore(context.Background(), RestoreInput{Snapshot: name})
	if err != nil {
		return err
	}
	return s.WaitRestore(context.Background(), task.ID)
}
func (s *Service) WaitRestore(ctx context.Context, id string) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		task, err := s.GetRestoreTask(id)
		if err != nil {
			return err
		}
		s.restoreMu.Lock()
		running := s.activeRestore == id
		uncertain := s.uncertainTask == id
		s.restoreMu.Unlock()
		if !running {
			if uncertain {
				return ErrRecoveryRequired
			}
			switch task.Status {
			case RestoreSucceeded:
				return nil
			case RestoreRolledBack:
				return fmt.Errorf("restore rolled back: %s", task.Error)
			case RestoreRecoveryRequired:
				return fmt.Errorf("%w: %s", ErrRecoveryRequired, task.Error)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) visibleTask(task RestoreTask) RestoreTask {
	s.restoreMu.Lock()
	uncertain := s.uncertainTask == task.ID
	s.restoreMu.Unlock()
	if uncertain {
		task.Status = RestoreRecoveryRequired
		task.Error = "任务终态落盘未确认，站点保持维护，请重启后检查恢复结果"
	}
	return task
}
func (s *Service) markUncertain(id string) {
	s.restoreMu.Lock()
	s.uncertainTask = id
	s.restoreMu.Unlock()
}
