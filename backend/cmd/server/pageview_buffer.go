// PV 埋点异步批量写入（Spec 61 阶段 5）：
// trackPageView 不再每请求同步写库（埋点写会串行化全部读），改为 buffered channel +
// 后台 flush goroutine——批量阈值（64 条）或定时（3s）触发单事务批量 INSERT；
// 服务优雅退出时 close() 排空缓冲并最终落库，失败仅记日志（埋点非关键路径）。
package main

import (
	"log"
	"sync"
	"time"

	"blog/internal/store"
)

const (
	pvBufferSize    = 1024             // 通道容量：突发流量不阻塞请求，超容直接丢弃
	pvBatchSize     = 64               // 批量 INSERT 阈值
	pvFlushInterval = 3 * time.Second  // 定时兜底
)

// pageViewBuffer PV 埋点缓冲器。
type pageViewBuffer struct {
	ch   chan store.PageView
	st   *store.Store
	done chan struct{}
	wg   sync.WaitGroup
}

func newPageViewBuffer(st *store.Store) *pageViewBuffer {
	b := &pageViewBuffer{
		ch:   make(chan store.PageView, pvBufferSize),
		st:   st,
		done: make(chan struct{}),
	}
	b.wg.Add(1)
	go b.flushLoop()
	return b
}

// push 非阻塞入队；缓冲满时静默丢弃（埋点失败不影响页面响应）。
func (b *pageViewBuffer) push(v store.PageView) {
	select {
	case b.ch <- v:
	default:
	}
}

// flushLoop 消费通道：批量阈值或定时触发 flush；done 后排空剩余再退出。
func (b *pageViewBuffer) flushLoop() {
	defer b.wg.Done()
	ticker := time.NewTicker(pvFlushInterval)
	defer ticker.Stop()
	var buf []store.PageView
	for {
		select {
		case v, ok := <-b.ch:
			if !ok {
				return
			}
			buf = append(buf, v)
			if len(buf) >= pvBatchSize {
				b.flush(buf)
				buf = nil
			}
		case <-ticker.C:
			if len(buf) > 0 {
				b.flush(buf)
				buf = nil
			}
		case <-b.done:
			// 排空通道剩余记录后最终落库
			for {
				select {
				case v := <-b.ch:
					buf = append(buf, v)
				default:
					b.flush(buf)
					return
				}
			}
		}
	}
}

func (b *pageViewBuffer) flush(records []store.PageView) {
	if len(records) == 0 || b.st == nil {
		return // nil store（测试空 webDeps 场景）：静默跳过
	}
	if err := b.st.BatchRecordViews(records); err != nil {
		log.Printf("pageview flush: %v", err)
	}
}

// close 停止 flush goroutine 并等待最终落库（服务优雅退出时调用，需在 DB 关闭之前）。
func (b *pageViewBuffer) close() {
	close(b.done)
	b.wg.Wait()
}
