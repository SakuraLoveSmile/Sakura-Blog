---
title: 深入理解 Go 的 goroutine 调度模型
date: 2026-08-01
excerpt: 从 GMP 模型到抢占式调度，聊聊 Go 运行时是如何用 M 个线程跑 N 个 goroutine 的。
categories: 技术
topic: Go 并发
tags: [Go, 并发, 调度]
cover: https://picsum.photos/seed/go-goroutine/1280/640
banner: https://picsum.photos/seed/go-banner/1600/640
poster_headline: GMP 模型拆解
poster_caption: goroutine · channel · scheduler
---

## 为什么需要调度器

Go 程序里的 `go func()` 可以瞬间创建成千上万个 goroutine，而操作系统线程是昂贵资源。调度器的职责就是**用少量的线程（M）高效运行海量的 goroutine（G）**。

## GMP 三件套

- **G（goroutine）**：一个待执行的任务，包含栈、指令指针、状态。
- **M（machine）**：一个操作系统线程，负责真正执行 G。
- **P（processor）**：本地可运行队列，持有 G 的等待队列，数量默认等于 CPU 核数。

```go
runtime.GOMAXPROCS(runtime.NumCPU()) // 默认就是核数
```

## 调度策略一句话

> P 像是一个"售票窗口"，M 是"服务员"，G 是"顾客"。没有 P 的 M 只能闲着等待，持有 P 的 M 才能领 G 干活。

本地队列空了就去全局队列"偷"任务，还会从别的 P 上**偷一半**任务过来（work stealing），保证所有核心忙起来。

## 抢占：不协作也能被中断

Go 1.14 之后实现了**异步抢占**：运行超过 10ms 的 G 会被监控线程发信号强制让出，避免一个死循环 G 卡死整个程序。

```go
func main() {
    ch := make(chan int, 10)
    for i := 0; i < 10; i++ {
        go func(n int) { ch <- n }(i)
    }
    for i := 0; i < 10; i++ {
        fmt.Println(<-ch)
    }
}
```

## 小结

理解了 GMP，就理解了为什么 Go 并发"简单"：你不用自己管线程池，只要关心业务并发；剩下的调度、偷取、抢占都交给运行时。
