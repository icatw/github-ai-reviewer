# GitHub AI Reviewer

GitHub AI Reviewer 是一个基于 GitHub App 的 AI 代码评审服务。项目目标是让用户把机器人安装到自己的 GitHub 仓库后，在 Pull Request 创建或更新时自动触发评审流程，系统读取 PR 变更、构建上下文、调用 OpenAI-compatible 模型，并把结构化评审结果回写到 PR 页面。

这个项目不是一个简单的 `git diff + prompt` demo，而是一个面向真实研发流程的 GitHub 自动化服务。第一阶段先跑通 GitHub App 到 PR 评论的闭环，后续逐步增强 Check Run、Inline Review、准确性校验、仓库上下文检索和 AST 级代码理解能力。

## 项目定位

一句话定位：

```text
一个可安装到 GitHub 仓库的仓库级 AI Code Review Agent。
```

它要解决的问题：

- 普通 AI Review 只看 diff，上下文不足，容易误报或漏报。
- 人工 Review 重复劳动多，比如格式、测试缺失、明显逻辑风险、潜在安全问题。
- GitHub PR 流程里缺少一个能结合静态检查、项目上下文和 LLM 推理的自动评审层。
- 只调用大模型不能形成工程闭环，需要真正接入 Webhook、GitHub App 权限、PR 评论和 CI 状态。

## MVP 范围

第一版只追求一个完整、可演示、可验证的闭环：

```text
GitHub PR 事件
  -> GitHub App Webhook
  -> 签名校验
  -> Installation Token 鉴权
  -> 获取 PR changed files / patch
  -> 构建 Review Prompt
  -> 调用 LLM
  -> 回写 PR Comment
```

MVP 包含：

- GitHub App Webhook 接口
- `X-Hub-Signature-256` 签名校验
- GitHub App JWT 生成
- Installation Token 换取
- Pull Request 变更文件获取
- OpenAI-compatible LLM 调用
- 结构化 Review Summary 生成
- PR 普通评论回写
- Docker 部署配置
- 自动创建测试 PR 并验证评论回写

MVP 暂不包含：

- 复杂 Dashboard
- 多租户计费系统
- 自动修复代码
- 全语言 AST 分析
- 向量数据库
- 全仓库智能体

这些能力会在后续阶段逐步添加。

## 后续增强范围

MVP 跑通后，后续按优先级增加：

- GitHub Check Run：在 PR Checks 区显示 AI Review 状态。
- Inline Review Comment：对高置信度问题评论到具体代码行。
- Finding Verifier：对 LLM 生成的问题做二次证据校验。
- Severity Policy：按 blocker、warning、suggestion、question 分级。
- 仓库配置文件：支持 `.github/ai-review.yml`。
- 静态分析集成：接入 `go test`、`go vet`、`staticcheck`、`gosec`、`semgrep` 等。
- Repo Context：读取完整变更文件、相关测试、README、docs、配置文件。
- AST / Tree-sitter：构建符号索引、调用链和影响面分析。
- Review 历史记录：保存任务状态、发现的问题、模型调用成本和失败日志。

## 技术栈建议

当前项目优先采用 Go 技术栈，便于体现后端工程能力和 GitHub App 服务端能力。

```text
语言：Go
HTTP 服务：net/http 或 Gin
GitHub SDK：google/go-github
JWT：golang-jwt/jwt
存储：SQLite，后续可升级 PostgreSQL
队列：开发版内存队列，后续可升级 Redis + Asynq
LLM：OpenAI-compatible API，支持 DeepSeek / OpenAI / Qwen 等
部署：Docker / Docker Compose / Nginx / HTTPS
```

## 目录结构

```text
cmd/server/          HTTP 服务入口
internal/webhook/    GitHub Webhook 解析与签名校验
internal/githubapp/  GitHub App 鉴权、JWT、Installation Token、API Client
internal/review/     Review 主流程编排
internal/llm/        OpenAI-compatible 模型客户端
internal/comment/    GitHub 评论渲染与发布
internal/storage/    任务、安装实例、评审结果持久化
internal/worker/     异步 Review Worker
internal/config/     服务配置和仓库配置解析
deploy/              Docker、Nginx、部署配置
scripts/             本地调试和自动化测试脚本
docs/                设计文档、调研文档、路线图
```

## 配置说明

创建 GitHub App 后，复制 `.env.example` 为 `.env` 并填写配置：

```bash
cp .env.example .env
```

关键配置：

```text
GITHUB_APP_ID                 GitHub App ID
GITHUB_APP_PRIVATE_KEY_PATH   GitHub App private key 文件路径
GITHUB_WEBHOOK_SECRET         GitHub Webhook Secret
LLM_BASE_URL                  OpenAI-compatible API 地址
LLM_API_KEY                   模型 API Key
LLM_MODEL                     模型名称
DATABASE_PATH                 SQLite 数据库路径
```

## 开发目标

第一阶段完成后，应满足以下验证标准：

- GitHub App 可以安装到指定仓库。
- 仓库创建或更新 PR 后，服务能收到 webhook。
- 服务能通过 installation token 访问该仓库 PR 数据。
- 服务能读取 PR 变更文件和 patch。
- 服务能调用 LLM 生成评审摘要。
- PR 页面能真实出现 AI Review 评论。
- 服务日志能追踪一次完整 review job。

这才算项目真正跑通，而不是只完成代码骨架。