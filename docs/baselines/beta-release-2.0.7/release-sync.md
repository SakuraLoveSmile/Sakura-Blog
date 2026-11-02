# release 历史同步到 main（v2.0.7-beta01 晋级前置）

- 背景：`release` 分支严格保护要求 PR head 包含 base 全部提交。`release` 独有的 4 个提交都是历史 main→release 合并记录（`400ada09`、`f8846101`、`b1b4e12e`、`8a66cdef`），使 main→release 的 PR #34 处于 `BEHIND`；GitHub 的 update-branch 被 main 保护拒绝（改 main 需走 PR），直接合并被 `not mergeable` 拒绝。
- 依据：沿用 `docs/stellar-200-full-port.md` 记录过的处理方式——不直接更新受保护分支、不使用管理员绕过；先通过正常 PR 把 release 历史合并进 main，再让 main→release PR 按新 main 重新验证。
- 本次操作：从 `origin/main` `9f37ac2c` 建同步分支，`git merge origin/release`（内容无差异，`git diff origin/main` 为空），经 PR #35 合入 main。未修改 `release` 分支、未放宽保护。
- 影响：main 增加 4 个历史 release 合并提交与 1 个同步合并提交，文件内容不变；随后的 main→release PR 与 tag `v2.0.7-beta01` 内容仍等价于 `77c4d99e`。
- 后续：`origin/main` 新增 release 历史后，dev 在下一次发布准备时同步 `origin/main`（本次发布开始后 dev 上另有并行任务提交，未纳入本 beta）。
