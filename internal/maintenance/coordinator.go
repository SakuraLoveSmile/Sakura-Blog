// Package maintenance 协调维护窗口与已进入的请求、后台作业。
package maintenance

import (
	"context"
	"errors"
	"sync"
)

var ErrBusy = errors.New("maintenance operation in progress")

// Coordinator 不持有数据库锁；普通工作取得短期许可，维护先阻断新工作再排空。
type Coordinator struct {
	mu         sync.Mutex
	active     int
	generation uint64
	operation  string
	reserved   string
	blocked    bool
	changed    chan struct{}
}

func New() *Coordinator { return &Coordinator{changed: make(chan struct{})} }

func (c *Coordinator) Active() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.operation != ""
}

// Acquire 不等待维护结束，调用方可以拒绝请求或跳过本轮后台任务。
func (c *Coordinator) Acquire() (func(), error) {
	if c == nil {
		return func() {}, nil
	}
	c.mu.Lock()
	if c.operation != "" {
		c.mu.Unlock()
		return nil, ErrBusy
	}
	c.active++
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.active--
			close(c.changed)
			c.changed = make(chan struct{})
		})
	}, nil
}

// Window 只有完整提交或完整补偿后才能 Close；Hold 使故障保持维护状态。
type Window struct {
	coordinator *Coordinator
	once        sync.Once
	held        bool
}

func (c *Coordinator) Begin(ctx context.Context, operation string) (*Window, error) {
	if c == nil {
		return &Window{}, nil
	}
	c.mu.Lock()
	if c.operation != "" || c.reserved != "" {
		c.mu.Unlock()
		return nil, ErrBusy
	}
	c.operation = operation
	c.generation++
	for c.active > 0 {
		changed := c.changed
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.operation = ""
			c.mu.Unlock()
			return nil, ctx.Err()
		case <-changed:
		}
		c.mu.Lock()
	}
	c.mu.Unlock()
	return &Window{coordinator: c}, nil
}

func (w *Window) Hold() {
	if w != nil {
		w.held = true
	}
}
func (w *Window) Close() {
	if w == nil || w.coordinator == nil {
		return
	}
	w.once.Do(func() {
		if !w.held {
			w.coordinator.mu.Lock()
			if !w.coordinator.blocked {
				w.coordinator.operation = ""
			}
			w.coordinator.mu.Unlock()
		}
	})
}

// Block 用于启动补偿失败，只允许健康探针与公共维护状态响应。
func (c *Coordinator) Block() {
	c.mu.Lock()
	c.operation = "recovery_required"
	c.blocked = true
	c.mu.Unlock()
}

type requestLeaseKey struct{}

func WithRequestLease(ctx context.Context, release func()) context.Context {
	return context.WithValue(ctx, requestLeaseKey{}, release)
}

// ReleaseRequest 让旧同步恢复接口在进入维护排空前归还自身许可，防止等待自己。
func ReleaseRequest(ctx context.Context) {
	if release, ok := ctx.Value(requestLeaseKey{}).(func()); ok {
		release()
	}
}

// Generation 标识当前数据世代，延迟缓冲不得跨维护窗口写回旧数据。
func (c *Coordinator) Generation() uint64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}

// Reserve 仅保留更新/恢复互斥权，不阻断更新过程中的健康检查与普通请求。
func (c *Coordinator) Reserve(operation string) (func(), error) {
	c.mu.Lock()
	if c.operation != "" || c.reserved != "" {
		c.mu.Unlock()
		return nil, ErrBusy
	}
	c.reserved = operation
	c.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { c.mu.Lock(); c.reserved = ""; c.mu.Unlock() }) }, nil
}

// Busy 同时包含恢复维护和更新保留权，用于提交前快速返回冲突。
func (c *Coordinator) Busy() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.operation != "" || c.reserved != ""
}
