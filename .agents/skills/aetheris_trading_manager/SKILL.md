---
name: aetheris-trading-manager
description: Read-only operations for the AETHERIS trading system. Check containers, balances, decision logs, and run preflight. Do not edit live prompts or start live trading unless the user explicitly asks.
---

# AETHERIS Trading Manager

Default posture: **read only**. This skill inspects the system. It does not place orders, restart live traders, or edit `prompts/adaptive.md` unless the user explicitly requests that action.

## Invariants

- Do not start Docker traders or call `POST /api/traders/:id/start` unless the user asked to go live.
- Do not edit `prompts/adaptive.md` or other threshold files as a "fix" for wait/loss.
- Do not `docker compose restart` to reload a prompt unless the user asked and no live positions are at risk.
- Process boot only auto-starts traders with `is_running=1`. Stale flags should stay at 0.
- Live trading is last. Climb `docs/guides/TEST_AND_DEBUG.md`: unit → inspect → replay → preflight → snapshot → dryrun. Never skip to live.

## Layout

- Workspace: `/Users/huangjunyou/aetheris`
- Config DB: `config.db`
- Active template: `prompts/adaptive.md` (core). Playbooks in `prompts/skills/` are injected by Go.
- Production UI: `web/` (Docker :3434). `web-beta/` is experimental.
- Decision logs: `decision_logs/<trader_id>/`
- API port: `3636`

## Read-only checks

Container health:

```bash
docker ps -a --filter name=aetheris
```

Traders (flag vs process):

```bash
sqlite3 config.db "SELECT id, name, is_running, initial_balance FROM traders;"
```

Read-only debug ladder (does **not** place orders):

```bash
go test ./trader ./decision
go run ./cmd/debug inspect -n 50
go run ./cmd/debug replay -n 50
go run ./cmd/preflight
go run ./cmd/debug snapshot
go run ./cmd/debug dryrun
```

Optional LLM ping (consumes tokens): `go run ./cmd/preflight -ai`

Live positions only if the backend is already running:

```bash
docker logs aetheris-trading --tail 50
```

## MCP (external operator tools)

`aetheris_mcp_server.py` is for an external agent to inspect status. The trading LLM does not use it.

| Tool | Safe default | Description |
|------|----------------|-------------|
| `get_trader_status` | read | Query current trader status |
| `get_active_positions` | read | Query live exchange positions |
| `read_trading_rules` | read | Read current strategy prompts |
| `get_recent_decision_logs` | read | Inspect recent cycle logs |
| `get_operator_directive` | read | Read active operator pause/note directive |
| `list_operator_events` | read | Audit log of external interventions |
| `set_operator_directive` | operator write | Safely pause/resume opens or leave a note (does not place orders) |
| `update_trading_rules` | **do not call** unless the user explicitly wants a prompt edit | Hot-reloads prompts via `/api/reload-prompts` |

## If asked to go live

1. `go test ./trader ./decision`
2. `go run ./cmd/debug inspect -n 20` and `replay -n 20`
3. `go run ./cmd/preflight` and `go run ./cmd/debug snapshot`
4. Confirm `is_running=0` until the user says start
5. Start only the named trader; do not `StartAll` blindly
