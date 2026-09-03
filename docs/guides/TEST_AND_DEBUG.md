# AETHERIS 测试与除错（谨慎路径）

原则：每一层都不能比上一层更容易碰到真钱。默认假设会输；工具的工作是证伪，不是证明策略能赚。

## 层

| 层 | 命令 | 碰交易所 | 碰 AI | 能下单 | 用途 |
|---|---|---|---|---|---|
| 0 单元 | `go test ./trader ./decision` | 否 | 否 | 否 | 风控数字、sanitize、dry-run wrapper |
| 1 日志 | `go run ./cmd/debug inspect` | 否 | 否 | 否 | 连续 wait、净值、动作分布 |
| 2 回放 | `go run ./cmd/debug replay` | 否 | 否 | 否 | 当前验证器对历史 JSON 会丢掉什么 |
| 3 预检 | `go run ./cmd/preflight` | 只读 | 可选 `-ai` | 否 | 密钥、余额、持仓、行情 |
| 4 快照 | `go run ./cmd/debug snapshot` | 只读 | 否 | 否 | 余额 + 回撤是否该 halt |
| 5 干跑 | `go run ./cmd/debug dryrun` | 只读 | 否 | 否 | 写操作被 DryRunTrader 吃掉 |
| 6 回测 | `go run ./cmd/run_backtest` | mock | mock/真 | 否 | 历史 K 线，默认 mock AI |
| 7 实盘 | API `POST /traders/:id/start` | 是 | 是 | 是 | 只有用户明确要求才做 |

正式 UI 是 `web/`（Docker 3434）。仪表板会显示 `risk_halted` / `consecutive_wait`。`web-beta` 仅本地试验。

Docker Desktop 更新会停掉容器。compose 使用 `restart: always`（不要 `unless-stopped`）。进程被 SIGTERM 时**不要**把 `is_running` 写成 0，否则容器回来也不会自动交易。登录后 `scripts/ensure-aetheris-up.sh`（LaunchAgent `com.huangjunyou.aetheris-compose`）会再 `compose up -d`。

交易策略：`prompts/adaptive.md` 为永远核心；`prompts/skills/*.md` 由 Go 按持仓注入。数字风控仍在 runtime。

不要跳层。改验证器或风控之后至少跑 0 + 2。改交易所客户端之后跑 3 + 5。

## 除错时先看什么

1. `inspect`：是不是又在连续 wait，净值有没有掉。
2. 单条 `decision_logs/.../decision_*.json` 的 `decisions`、`error_message`、`execution_log`。不要一上来改 `adaptive.md`。
3. `replay`：若历史 open 被当前 sanitize 丢掉，先分清是缺行情（回放限制）还是规则真的更严。
4. `snapshot`：账户是不是还在、有没有漏仓、halt 会不会误触发。
5. 容器日志只在进程已经在跑时才看。

## 禁止

- 用「跑一轮实盘」当单元测试。
- 为了看 AI 会不会下单而启动 trader。
- 在 live 上改 prompt 后不经 replay 就重启。
- 把 `cmd/run_backtest` 的 mock 通过当成实盘可开。

## 改动后的最小证明

- 风控 / sanitize：`go test ./trader ./decision`
- 日志行为：`go run ./cmd/debug inspect -n 20` 和 `replay -n 20`
- API：`go run ./cmd/preflight`（不要加 `-ai` 除非在测模型连通）
