# 公开项目与资料调研

本文档记录 GitHub AI Reviewer 项目前期调研得到的参考资料、开源项目形态和可借鉴设计。调研目标不是照搬某个项目，而是提炼出适合本项目的实现路线。

## 调研结论摘要

AI PR Review 工具大致分成三类：

```text
1. AI PR Reviewer
   代表：PR-Agent
   特点：围绕 PR 生成 summary、review、improve 建议，强调 LLM 能力。

2. Review Reporter / Annotation 工具
   代表：reviewdog
   特点：把 lint、static analysis、test 等确定性工具输出映射成 PR 评论、Check、Annotation。

3. GitHub App Framework
   代表：Probot
   特点：帮助开发者快速构建 GitHub App，处理 webhook 事件和 GitHub API 调用。
```

本项目应该结合三者：

```text
PR-Agent 的 AI review 产品形态
+ reviewdog 的 reporter 和降噪思路
+ GitHub App / Probot 的安装、权限、Webhook 模型
= GitHub App 形态的 Repo-aware AI Code Review Agent
```

## 参考项目：PR-Agent

仓库：`The-PR-Agent/pr-agent`

调研时观察到的信息：

- 项目定位为开源 AI PR Reviewer。
- GitHub stars 约 1.1 万以上。
- 主要语言是 Python。
- 支持 GitHub Action 使用方式。
- 支持 CLI 本地运行方式。
- 也支持 GitLab、Bitbucket、Azure DevOps 等平台。
- README 中推荐 GitHub Action 快速接入：在 `.github/workflows/pr-agent.yml` 中配置 action 和模型 key。

PR-Agent 的核心参考价值：

- AI PR Review 不一定一开始就做 GitHub App，也可以先支持 GitHub Action / CLI。
- 产品能力不应只有 review，还可以拆成 describe、review、improve 等命令。
- PR 评审结果需要结构化，否则很难稳定回写到 PR。
- 多平台支持不是第一阶段重点，但架构上可以把 review engine 和平台适配层分离。

对本项目的启发：

```text
review engine 应该独立于 GitHub App。
GitHub App 只是入口之一，未来可以复用到 CLI、GitHub Action、GitLab webhook。
```

建议本项目借鉴：

- PR summary 生成。
- Review findings 分级。
- 配置化模型提供商。
- 本地 CLI 入口，便于调试。

不建议第一阶段照搬：

- 多平台支持。
- 过多命令模式。
- 复杂配置系统。
- 大量 prompt 模板。

## 参考项目：reviewdog

仓库：`reviewdog/reviewdog`

调研时观察到的信息：

- 项目定位是自动化代码评审工具。
- GitHub stars 约 9k 以上。
- 主要语言是 Go。
- 它不是 AI Review 工具，而是把任意 linter / static analysis 工具的输出转换成代码托管平台上的 Review Comment、Check、Annotation。
- 支持 GitHub、GitLab、Bitbucket 等平台。
- 支持多种 reporter，例如 GitHub PR Check、GitHub Check、GitHub PR Review、GitHub Annotations。
- 关键能力是“只把落在 diff 范围内的问题发到 PR”，减少无关噪声。

reviewdog 的核心参考价值：

- 代码评审工具必须处理“问题定位”和“评论落点”。
- 不能把全量 lint 结果无脑发到 PR；应该按 diff 过滤。
- reporter 抽象非常重要，同一份诊断结果可以输出到不同平台或不同 GitHub 表现形式。
- 静态工具结果比 LLM 更确定，应该作为准确性兜底。

对本项目的启发：

```text
LLM 输出的 findings 可以被设计成类似 diagnostic format，之后由 reporter 统一发布。
```

建议本项目设计类似结构：

```text
Review Finding
  -> Markdown Summary Reporter
  -> GitHub PR Comment Reporter
  -> GitHub Inline Review Reporter
  -> GitHub Check Run Reporter
```

后续可以接入：

```text
go test
go vet
staticcheck
gosec
semgrep
eslint
tsc
```

然后将这些工具结果和 LLM finding 统一进一个 ReviewResult。

## 参考项目：Probot

仓库：`probot/probot`

调研时观察到的信息：

- 项目定位是构建 GitHub Apps 的框架。
- GitHub stars 约 9k 以上。
- 主要语言是 TypeScript。
- README 明确说明 GitHub Apps 可以安装到组织和用户账号，并授予特定仓库访问权限。
- GitHub App 通过 webhook 监听仓库或组织事件，再用 GitHub API 执行动作。

Probot 的核心参考价值：

- GitHub App 是 GitHub 平台的一等自动化能力。
- App 不是跟随个人主页自动生效，而是通过 installation 安装到账号或组织，并选择仓库授权。
- Webhook 事件驱动是 GitHub App 的核心模型。
- App 应该围绕事件处理器组织代码，例如 `pull_request.opened`、`issue_comment.created`。

对本项目的启发：

```text
即使用 Go 开发，也应该采用类似 Probot 的事件分发模型。
```

可以设计：

```text
webhook receiver
  -> event router
  -> pull_request handler
  -> issue_comment handler
  -> review job producer
```

这样后续扩展 `/ai-review`、`/ai-review deep`、`/ai-review explain` 会更自然。

## GitHub 官方能力

本项目需要重点参考以下 GitHub 官方能力。

### GitHub App

GitHub App 是推荐的集成方式，适合构建自动化机器人、CI 工具、代码审查工具和安全扫描工具。

关键概念：

```text
App：机器人产品本体，只创建一次。
Installation：用户或组织安装这个 App 的记录。
Repository selection：安装时选择授权哪些仓库。
Installation Token：App 针对某个 installation 换取的短期访问 token。
```

本项目使用流程：

```text
用户安装 App 到仓库
  -> PR 事件触发 webhook
  -> payload 中包含 installation_id
  -> 服务端用 App private key 生成 JWT
  -> 用 JWT 换取 installation token
  -> 用 installation token 访问仓库 PR API
```

### Webhook

Webhook 是 GitHub App 接收事件的入口。

本项目需要处理：

```text
X-GitHub-Event：事件类型
X-GitHub-Delivery：事件唯一 ID
X-Hub-Signature-256：签名
payload：事件内容
```

必须校验签名，不能直接信任 payload。

MVP 关注：

```text
pull_request.opened
pull_request.synchronize
pull_request.reopened
```

后续关注：

```text
issue_comment.created
pull_request_review_comment.created
```

### Pull Request Files API

用于读取 PR 中变更的文件。

典型用途：

```text
获取 changed files
获取每个文件 patch
获取 additions / deletions / status
判断文件是否过大
过滤 package-lock、dist、vendor 等低价值文件
```

MVP 主要依赖该 API 构建 Review Prompt。

### Pull Request Review API

用于提交正式 PR Review 和 inline comments。

MVP 可以先不用，先用普通 issue comment。后续接入后，可以实现：

```text
COMMENT
REQUEST_CHANGES
APPROVE
inline comments
```

本项目默认不自动 approve，也不轻易 request changes。更稳妥的策略是：

```text
blocker -> Check Run failure，可选 request changes
warning -> summary / inline comment
suggestion -> summary
question -> summary
```

### Checks API

用于在 PR Checks 区展示 AI Review 状态。

可实现：

```text
in_progress：review 正在运行
success：没有发现 blocker
neutral：只有 warning / suggestion
failure：存在高置信度 blocker
```

Checks API 能让 AI Review 更像 CI 的一部分，而不是只在 conversation 里发一条评论。

## 竞品和产品形态

公开市场中常见 AI PR Review 产品包括 CodeRabbit、Qodo、Greptile 等。它们通常具备这些能力：

- 安装到 GitHub 仓库或组织。
- PR 自动触发。
- 生成 PR 总结。
- 给出代码问题和修改建议。
- 支持与用户在 PR 评论区交互。
- 强调仓库上下文，而不只是 diff。
- 提供团队配置和管理页面。

本项目不需要第一阶段追平这些产品，但可以学习产品路径：

```text
先做 PR 自动 Review
再做上下文增强
再做准确性控制
最后做团队化和管理后台
```

## 本项目的差异化方向

为了避免变成普通 AI wrapper，本项目应该突出以下差异化：

### 1. Repo-aware，而不是 diff-only

Diff 只告诉系统“哪里变了”，不能单独代表完整语义。系统应该围绕变更点读取：

```text
完整函数
完整文件
调用方
被调用方
相关测试
项目文档
配置文件
```

### 2. Evidence-based Review

所有高等级 finding 必须有证据链：

```text
文件位置
代码证据
失败场景
修复建议
置信度
```

没有证据链的问题不能作为 blocker。

### 3. Tool + LLM 融合

确定性工具负责高可信检查：

```text
测试
lint
类型检查
安全扫描
secret scan
```

LLM 负责上下文推理：

```text
业务逻辑风险
测试缺口
跨文件影响
边界条件
可维护性建议
```

### 4. Reporter 抽象

同一份 ReviewResult 可以输出到不同渠道：

```text
PR Summary Comment
Inline Review Comment
Check Run
本地 CLI 输出
HTML 报告
```

### 5. 保守阻断策略

AI 不应该默认阻塞合并。只有满足严格条件的问题才升级为 blocker。

## 推荐实现路线

### 第一阶段：MVP

参考 PR-Agent 的快速接入思路，但采用 GitHub App 而不是 GitHub Action。

目标：

```text
安装 App -> 创建 PR -> 自动评论
```

### 第二阶段：Reporter 和 Check Run

参考 reviewdog 的 reporter 思路，把 ReviewResult 和 GitHub 输出解耦。

目标：

```text
ReviewResult -> comment / inline / check run
```

### 第三阶段：事件模型和命令交互

参考 Probot 的事件处理方式，支持：

```text
/ai-review
/ai-review deep
/ai-review explain
```

### 第四阶段：上下文增强

实现 repo-aware context，而不是只看 patch。

### 第五阶段：准确性评估

建立 eval cases，统计：

```text
precision
recall
false positive rate
blocking precision
latency
cost
```

对 AI Review Bot 来说，最重要的是 blocking precision。宁可少报，也不能乱挡 PR。