#!/usr/bin/env python3
import os
import json
import sqlite3
import urllib.request
from pathlib import Path
from mcp.server.fastmcp import FastMCP

# 初始化 FastMCP Server
mcp = FastMCP("aetheris-trading")

# 匯出 Starlette ASGI 應用供 Uvicorn 執行 (SSE 模式)
app = mcp.sse_app()

# 自動偵測工作目錄：容器內為 /app，本機為當前工作目錄
if Path("/app/config.db").exists():
    WORKSPACE_DIR = Path("/app")
else:
    WORKSPACE_DIR = Path(os.getcwd())

DB_PATH = WORKSPACE_DIR / "config.db"
RULES_PATH = WORKSPACE_DIR / "prompts" / "adaptive.md"
API_URL = os.environ.get("AETHERIS_API_URL", os.environ.get("NOFX_API_URL", "http://localhost:3636"))

def get_db_connection():
    return sqlite3.connect(str(DB_PATH))

def get_auth_token():
    password = os.environ.get("AETHERIS_ADMIN_PASSWORD", os.environ.get("NOFX_ADMIN_PASSWORD", "admin123"))
    try:
        req_data = json.dumps({"password": password}).encode('utf-8')
        req = urllib.request.Request(
            f"{API_URL}/api/admin-login",
            data=req_data,
            headers={"Content-Type": "application/json"},
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as response:
            res_data = json.loads(response.read().decode())
            return res_data.get("token")
    except Exception as e:
        import sys
        sys.stderr.write(f"⚠️ [MCP] 管理員登入認證失敗: {e}\n")
        return None

def get_default_trader_id():
    try:
        conn = get_db_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT id FROM traders ORDER BY is_running DESC LIMIT 1")
        row = cursor.fetchone()
        conn.close()
        return row[0] if row else None
    except Exception as e:
        import sys
        sys.stderr.write(f"⚠️ [MCP] 查詢交易員ID失敗: {e}\n")
        return None

@mcp.tool()
def get_trader_status() -> str:
    """獲取目前交易系統中所有 AI 交易員的運行狀態、當前餘額、所屬帳號以及策略模板名稱。"""
    try:
        conn = get_db_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT id, name, system_prompt_template, is_running, initial_balance FROM traders")
        rows = cursor.fetchall()
        traders = []
        for r in rows:
            traders.append({
                "id": r[0],
                "name": r[1],
                "template": r[2],
                "is_running": bool(r[3]),
                "initial_balance": r[4]
            })
        conn.close()
        return json.dumps(traders, ensure_ascii=False, indent=2)
    except Exception as e:
        return f"錯誤: 無法查詢交易員狀態: {e}"

@mcp.tool()
def get_active_positions() -> str:
    """向交易所 API 查詢目前帳戶的所有即時合約持倉（包括方向、槓桿、開倉價與未實現盈虧）。"""
    trader_id = get_default_trader_id()
    if not trader_id:
        return "錯誤: 數據庫中找不到任何交易員。"
    
    token = get_auth_token()
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    
    try:
        req = urllib.request.Request(
            f"{API_URL}/api/account?trader_id={trader_id}",
            headers=headers
        )
        with urllib.request.urlopen(req, timeout=10) as response:
            data = json.loads(response.read().decode())
            positions = data.get("positions", [])
            balance = data.get("total_equity", 0)
            result = {
                "trader_id": trader_id,
                "total_equity_usdt": balance,
                "active_positions": positions
            }
            return json.dumps(result, ensure_ascii=False, indent=2)
    except Exception as e:
        return f"錯誤: 無法查詢持倉資訊（API 可能離線或認證失敗）: {e}"

@mcp.tool()
def read_trading_rules() -> str:
    """讀取目前的系統主策略模板 (adaptive.md) 內容。讓 Agent 了解開倉條件、Z-Score 門檻與風控邏輯。"""
    if not RULES_PATH.exists():
        return "錯誤: 策略規則檔案 adaptive.md 不存在。"
    return RULES_PATH.read_text(encoding="utf-8")

@mcp.tool()
def update_trading_rules(content: str) -> str:
    """動態更新主策略模板 (adaptive.md) 的交易規則或參數閾值。更新前會自動備份舊策略，更新後需要手動重啟交易容器以載入新規則。"""
    try:
        if RULES_PATH.exists():
            backup_path = RULES_PATH.with_suffix(".md.bak")
            backup_path.write_text(RULES_PATH.read_text(encoding="utf-8"), encoding="utf-8")
            backup_msg = f"已成功將舊策略備份至 {backup_path.name}。\n"
        else:
            backup_msg = "未找到舊策略檔案，直接建立新策略。\n"

        RULES_PATH.write_text(content, encoding="utf-8")
        reload_msg = "未能呼叫 /api/reload-prompts，請在 UI 按 Reload prompts。"
        token = get_auth_token()
        try:
            req = urllib.request.Request(
                f"{API_URL}/api/reload-prompts",
                data=b"{}",
                headers={"Content-Type": "application/json", **({"Authorization": f"Bearer {token}"} if token else {})},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=10) as response:
                if response.status == 200:
                    reload_msg = "已呼叫 /api/reload-prompts，下一輪會用新規則（不必重啟容器）。"
        except Exception as e:
            reload_msg = f"reload-prompts 失敗: {e}"
        return f"成功: 策略檔案已更新。\n{backup_msg}{reload_msg}"
    except Exception as e:
        return f"錯誤: 策略檔案寫入或備份失敗: {e}"

@mcp.tool()
def get_recent_decision_logs(limit: int = 5) -> str:
    """獲取最近幾次的交易決策日誌。包含了 AI 當初的思考過程、多空確認指標以及最終是 wait/open 還是 close。"""
    log_dir = WORKSPACE_DIR / "decision_logs"
    if not log_dir.exists():
        return "目前尚無任何交易決策日誌。"
    
    json_files = sorted(log_dir.glob("**/decision_*.json"), key=lambda f: f.stat().st_mtime, reverse=True)
    
    logs = []
    for file in json_files[:limit]:
        try:
            with open(file, 'r', encoding='utf-8') as f:
                data = json.load(f)
                logs.append({
                    "timestamp": data.get("timestamp"),
                    "cycle": data.get("cycle"),
                    "decisions": data.get("decisions"),
                    "reasoning_summary": data.get("reasoning", "")[:500] + "..." if len(data.get("reasoning", "")) > 500 else data.get("reasoning")
                })
        except Exception:
            continue
    
    return json.dumps(logs, ensure_ascii=False, indent=2)

def _api_json(method: str, path: str, payload=None):
    token = get_auth_token()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = None
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(f"{API_URL}{path}", data=data, headers=headers, method=method)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode())

@mcp.tool()
def get_operator_directive() -> str:
    """讀取目前外部 agent 干涉狀態：是否暫停新開倉、留言、到期時間。不下單。"""
    try:
        return json.dumps(_api_json("GET", "/api/operator-directive"), ensure_ascii=False, indent=2)
    except Exception as e:
        return f"錯誤: {e}"

@mcp.tool()
def list_operator_events(limit: int = 20) -> str:
    """列出最近的外部干涉審計紀錄（誰、何時、pause/resume/note）。"""
    try:
        return json.dumps(_api_json("GET", f"/api/operator-events?limit={int(limit)}"), ensure_ascii=False, indent=2)
    except Exception as e:
        return f"錯誤: {e}"

@mcp.tool()
def set_operator_directive(action: str, note: str = "", actor: str = "mcp", expires_in_minutes: int = 0) -> str:
    """寫入一筆外部干涉。action 只能是 pause_opens、resume_opens、note。不會下單、不會改風控數字。pause 預設 4 小時，note 預設 12 小時。expires_in_minutes>0 覆寫；<0 表示不過期。"""
    payload = {"action": action, "note": note, "actor": actor or "mcp"}
    if expires_in_minutes:
        payload["expires_in_minutes"] = expires_in_minutes
    try:
        return json.dumps(_api_json("POST", "/api/operator-events", payload), ensure_ascii=False, indent=2)
    except Exception as e:
        return f"錯誤: {e}"

if __name__ == "__main__":
    # FastMCP 會自動處理執行（本機 stdio 模式或 SSE 網絡模式）
    mcp.run()
