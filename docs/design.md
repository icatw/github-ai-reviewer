# 设计文档

## 产品目标

GitHub AI Reviewer 的目标是构建一个 GitHub App 形态的 AI Code Review Agent。用户将 App 安装到自己的 GitHub 账号或组织后，可以选择授权部分仓库或全部仓库。被授权仓库在创建或更新 Pull Request 时，GitHub 会向服务端发送 webhook，服务端自动拉取 PR 变更、构建上下文、调用 LLM 进行分析，并将评审结果回写到 GitHub PR 页面。

核心目标不是替代人工 Review，而是提供一个低噪声、可追责、可验证的自动评审辅助层。

## 设计原则

- **先闭环，后智能**：先实现 GitHub App 安装、Webhook 接收、PR 数据读取、评论回写，再逐步增强上下文和准确性。
- **AI 不直接拥有最终裁决权**：LLM 输出必须经过结构化约束、证据校验和风险分级。
- **不只看 diff**：diff 只作为入口，后续需要扩展完整文件、相关测试、文档、配置和调用链上下文。
- **默认保守**：MVP 只发 summary comment，不自动阻断合并；只有引入静态检查和 finding verifier 后才允许 fail check。
- **低权限原则**：GitHub App 只申请实现功能所需的最小权限。
- **可观测**：每次 review job 都应该能追踪状态、输入范围、模型输出、错误原因和回写结果。

## MVP 事件流程

```text
pull_request webhook
  -> 读取请求体
  -> 校验 X-Hub-Signature-256
  -> 判断事件 action 是否为 opened / synchronize / reopened
  -> 提取 installation_id、owner、repo、pull_number、head_sha
  -> 创建 ReviewJob
  -> 后台 worker 处理任务
  -> 生成 GitHub App JWT
  -> 换取 Installation Token
  -> 调 GitHub API 获取 PR changed files 和 patch
  -> 构建紧凑 Review Prompt
  -> 调用 OpenAI-compatible LLM
  -> 渲染 Markdown Review Summary
  -> 调 GitHub API 发布 PR issue comment
```

Webhook 接口必须快速返回，不能同步等待 LLM。推荐返回：

```text
HTTP 202 Accepted
```

## 核心模块

### webhook 模块

职责：

- 接收 GitHub webhook HTTP 请求。
- 校验 `X-Hub-Signature-256`。
- 解析 `X-GitHub-Event`。
- 过滤不关心的事件和 action。
- 从 payload 中提取 review job 所需字段。

MVP 关注事件：

```text
pull_request.opened
pull_request.synchronize
pull_request.reopened
```

后续可增加：

```text
issue_comment.created，用于支持 /ai-review 命令
pull_request_review_comment.created，用于响应 review 线程
```

### githubapp 模块

职责：

- 读取 GitHub App ID 和 Private Key。
- 生成 GitHub App JWT。
- 使用 JWT 调用 GitHub API 换取 Installation Token。
- 基于 Installation Token 创建 GitHub API Client。
- 处理 GitHub API rate limit、错误和重试。

GitHub App 鉴权链路：

```text
App Private Key
  -> 生成 JWT
  -> POST /app/installations/{installation_id}/access_tokens
  -> 获取 Installation Token
  -> 使用 token 访问仓库 API
```

Installation Token 是短期 token，不建议长期落库。可以在内存中按 installation_id 缓存，到期前刷新。

### review 模块

职责：

- 编排一次 PR review 的完整流程。
- 获取 PR 信息和 changed files。
- 构建 review context。
- 调用 LLM。
- 解析结构化结果。
- 调用 comment 模块回写 GitHub。

MVP 的 context 可以包含：

```text
PR 标题
PR 描述
base branch
head branch
changed files 列表
每个文件的 patch
每个文件的 additions / deletions
```

后续扩展 context：

```text
完整变更文件内容
相关测试文件
README / docs
.github/ai-review.yml
静态检查结果
函数级上下文
调用链和影响面分析
```

### llm 模块

职责：

- 封装 OpenAI-compatible Chat Completions 或 Responses API。
- 支持 base_url、api_key、model 配置。
- 约束输出格式。
- 处理超时、重试、错误和 token 限制。

MVP 可以先输出 Markdown，但建议尽早切换到 JSON schema：

```json
{
  "summary": "本次 PR 的总体评价",
  "risk_score": 0,
  "findings": [
    {
      "severity": "warning",
      "category": "correctness",
      "file": "internal/example.go",
      "line": 42,
      "title": "问题标题",
      "evidence": "代码证据",
      "failure_scenario": "可能失败的场景",
      "suggestion": "修复建议",
      "confidence": 0.82
    }
  ],
  "missing_tests": []
}
```

### comment 模块

职责：

- 把结构化 review result 渲染成 GitHub Markdown。
- 发布 PR 普通评论。
- 通过稳定隐藏 marker 更新已有 AI Review summary comment，避免重复刷屏。

### reporter 模块

职责：

- 接收 review job started、completed、suppressed、failed 生命周期事件。
- 将同一份结构化 review result fan-out 到多个输出通道。
- 保留 PR summary comment 输出，并新增 `AI Review` Check Run 输出。
- Check Run 完成态使用 advisory/non-blocking policy：正常完成设置为 `neutral`，即使存在 finding；只有 GitHub API、LLM provider、reporter 或 job execution 等基础设施失败才可以设置为 `failure`。

MVP 使用：

```text
POST /repos/{owner}/{repo}/issues/{pull_number}/comments
```

M3 使用：

```text
POST /repos/{owner}/{repo}/check-runs
PATCH /repos/{owner}/{repo}/check-runs/{check_run_id}
```

Inline review comments 和 request-changes 行为不属于 M3。

## 权限设计

M3 GitHub App 最小权限：

```text
Metadata: Read-only
Contents: Read-only
Pull requests: Read and write
Issues: Read and write
Checks: Read and write
```

M1/M2 如果不启用 Check Run，可以暂不申请：

```text
Checks: Read and write
```

权限含义：

- Metadata：GitHub App 基础访问必需。
- Contents：读取 PR 中变更文件内容。
- Pull requests：读取 PR 信息、提交 PR review。
- Issues：PR conversation comment 使用 issue comment API，因此需要 issues 写权限。
- Checks：在 PR Checks 区展示 AI Review 状态。

## 准确性策略

AI Review 的准确性不能只靠 prompt，需要工程机制保证。

MVP 阶段：

- 不自动阻断合并。
- 不发布大量 inline comments。
- 评论中明确区分 summary、risk、suggestion。
- 要求模型基于给定 diff 输出，不允许臆测不存在的文件。

M3 Check Run 阶段：

- `AI Review` Check Run 用于展示 review lifecycle，不作为 merge gate。
- 完成的 AI review job 设置 `neutral`，即使存在 blocker、warning、suggestion 或 question finding。
- `failure` 只表示服务执行失败，例如 GitHub API、LLM provider、reporter 或 job execution failure；不能由 AI finding severity 推导。
- Check Run output 保持简短，不包含 raw prompt、raw model response、完整 webhook payload、installation token、API key、private key 或无边界 diff 内容。

增强阶段：

- 接入静态检查和测试结果。
- 要求每个 finding 包含 evidence、failure_scenario 和 confidence。
- 增加 finding verifier，校验问题是否被上下文支持。
- 只有高置信度且具备证据链的问题才升级为 blocker。
- 上下文不足时输出 question，而不是 blocker。

分级策略：

```text
blocker：明确 bug、安全风险或会导致 CI/运行失败的问题，可阻断合并。
warning：有较强风险，但依赖业务假设，不阻断。
suggestion：可读性、维护性、测试建议，不阻断。
question：上下文不足，需要人确认。
```

## 阶段计划

### M1：GitHub App 最小闭环

- 启动 HTTP 服务。
- 实现 webhook 签名校验。
- 接收 pull_request 事件。
- 生成 App JWT。
- 换取 Installation Token。
- 获取 PR changed files。
- 调 LLM 生成 review summary。
- 发布 PR comment。

### M2：结构化 Review

- 定义 ReviewResult JSON schema。
- 支持 severity、category、evidence、suggestion、confidence。
- 渲染稳定的 Markdown 评论。
- 控制评论长度和噪声。

### M3：Check Run Reporter

- 创建或更新 `AI Review` Check Run。
- review 开始时上报 `in_progress`。
- review 正常完成时上报 `completed` / `neutral`。
- 基础设施或 job execution failure 可上报 `completed` / `failure`，但不能基于 AI findings fail check。
- synchronize 后更新旧评论，避免刷屏。

### M4：仓库上下文

- 读取完整变更文件。
- 读取相关测试文件。
- 读取 README、docs、配置文件。
- 支持 `.github/ai-review.yml`。
- 实现 context size 控制。

### M5：Repo-aware Engine

- 引入 Go AST / Tree-sitter。
- 提取变更函数和结构体。
- 构建符号索引。
- 分析 caller / callee。
- 生成影响面报告。
- 引入 finding verifier 和静态检查结果融合。

## 验证标准

每个阶段都必须有真实验证，不以代码写完为准。

M1 验证标准：

- GitHub App 能安装到测试仓库。
- 创建测试 PR 后，服务收到 webhook。
- 服务日志记录 review job。
- GitHub API 成功获取 PR files。
- PR 页面出现 AI Review 评论。

M2 验证标准：

- LLM 输出能稳定解析为结构化结果。
- 无效 JSON 能重试或降级处理。
- 评论格式在 GitHub PR 页面可读。

M3 验证标准：

- PR Checks 区出现 AI Review 状态。
- AI findings 不会导致 Check Run failure 或 request changes。
- 重复 push 不产生大量重复 summary comments。

M4-M5 验证标准：

- 能展示本次 review 使用了哪些上下文。
- 能识别至少一类跨文件风险。
- 误报结果能被 verifier 降级或过滤。
