---
title: SQLite WAL 模式：读写并发不再互相阻塞
date: 2026-07-22
excerpt: 把博客的存储从 rollback journal 切到 WAL 之后，写入延迟和读阻塞都明显改善，记录一下原理与迁移步骤。
categories: 技术
topic: 数据库
tags: [SQLite, 数据库, 性能]
cover: https://picsum.photos/seed/sqlite-wal/1280/640
banner: https://picsum.photos/seed/sqlite-banner/1600/640
poster_headline: WAL 是什么
poster_caption: write-ahead logging · checkpoint · -wal
---

## 传统模式的问题

默认的 rollback journal 模式下，**写事务要拿独占锁**，写期间所有读都要等待。对小博客够用，但评论、点赞这类高频小写入一旦出现，页面就会偶发抖动。

## WAL 做了什么

WAL（Write-Ahead Logging）把修改先追加到独立的 `-wal` 文件，**读依然可以直接读主库文件**，读写不再互斥：

| 模式 | 写期间能否读 | 读期间能否写 |
|------|:---:|:---:|
| Rollback journal | ❌ | ❌ |
| WAL | ✅ | ✅ |

## 怎么开启

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL; -- WAL 下推荐，兼顾安全与性能
```

迁移后目录里多出 `blog.db-wal` 和 `blog.db-shm` 两个文件，属正常现象，backup 时别忘了它们。

## 什么时候别用 WAL

- 应用跑在网络文件系统（NFS）上时，WAL 的共享内存机制不可靠。
- 数据库被多个**进程**同时高频写时，WAL 依然会有锁竞争——那是该换 PostgreSQL 的信号了。

> 一句话：单机、单进程、读写混合的场景，WAL 是 SQLite 的最优解。
