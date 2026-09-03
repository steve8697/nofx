# web-beta（試驗前端）

這不是正式 UI。Docker / `aetheris-frontend` 打的是 `../web/`。

本目錄是換皮實驗：Dashboard / Diagnostics / Settings 版面較新，但路由、認證、部分 API 路徑曾與後端不一致。已補上：

- Vite proxy `/api` → `localhost:3636`（dev 埠 4545）
- `public-config` 路徑
- 系統模板改為模板**名**下拉，不再把整份 prompt 寫進 `system_prompt_template`
- 預設掃週期 3 分鐘

不要把 compose 切到這個目錄，除非先把認證/路由做到與 `web/` 同級。
