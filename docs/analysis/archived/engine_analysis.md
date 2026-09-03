# engine.go 代码分析报告

## 📊 文件统计

- **总行数**: 878 行
- **函数数量**: 约 20 个
- **类型定义**: 7 个结构体
- **整体评价**: 中等偏长，但结构清晰

---

## 🔍 代码结构分析

### 主要模块

1. **类型定义** (27-113行) - 87行
   - PositionInfo, AccountInfo, CandidateCoin, OITopData
   - Context, Decision, FullDecision
   - ✅ 结构清晰，职责明确

2. **核心流程** (115-147行) - 33行
   - GetFullDecision, GetFullDecisionWithCustomPrompt
   - ✅ 流程清晰，逻辑简单

3. **数据获取** (149-253行) - 105行
   - fetchMarketDataForContext
   - calculateMaxCandidates
   - ✅ 功能单一，逻辑清晰

4. **Prompt构建** (255-495行) - 241行 ⚠️ **较长**
   - buildSystemPromptWithCustom (25行)
   - buildSystemPrompt (92行) ⚠️ **可拆分**
   - buildUserPrompt (119行) ⚠️ **可拆分**

5. **JSON解析** (497-603行) - 107行
   - parseFullDecisionResponse
   - extractCoTTrace
   - extractDecisions
   - fixMissingQuotes
   - validateJSONFormat
   - ✅ 功能独立，可提取到单独文件

6. **辅助函数** (605-683行) - 79行
   - min, removeInvisibleRunes, compactArrayOpen
   - findMatchingBracket
   - ✅ 工具函数，可提取

7. **验证逻辑** (685-877行) - 193行 ⚠️ **最长**
   - validateDecisions
   - validateDecision (160行) ⚠️ **可拆分**

---

## ⚠️ 潜在问题

### 1. 函数过长

**`validateDecision` 函数 (718-877行) - 160行**
- 包含多种验证逻辑
- 建议拆分：
  - `validateAction`
  - `validateOpenPosition`
  - `validatePositionSize`
  - `validateStopLossTakeProfit`
  - `validateRiskRewardRatio`

**`buildSystemPrompt` 函数 (283-374行) - 92行**
- 包含模板加载、硬约束生成、输出格式
- 建议拆分：
  - `loadPromptTemplate`
  - `buildHardConstraints`
  - `buildOutputFormat`

**`buildUserPrompt` 函数 (377-495行) - 119行**
- 包含多个部分：系统状态、BTC、账户、持仓、候选币种
- 建议拆分：
  - `buildSystemStatusSection`
  - `buildAccountSection`
  - `buildPositionsSection`
  - `buildCandidateCoinsSection`

### 2. 职责混合

- JSON解析逻辑和业务逻辑混在一起
- 验证逻辑和核心流程混在一起

### 3. 可维护性

- 文件较长，查找特定功能需要滚动
- 相关功能分散，修改时需要跳转

---

## 💡 优化建议

### 方案1：按功能拆分（推荐）

```
decision/
├── engine.go          (核心流程，200行)
├── prompt_builder.go  (Prompt构建，250行)
├── json_parser.go     (JSON解析，150行)
├── validator.go       (验证逻辑，200行)
└── types.go           (类型定义，100行)
```

**优点**：
- 职责清晰，易于维护
- 每个文件大小适中（100-250行）
- 便于测试和修改

**缺点**：
- 需要重构，工作量较大
- 可能增加包内函数调用

### 方案2：提取辅助函数（轻量级）

保持 `engine.go` 为主文件，提取：
- `json_parser.go` - JSON解析相关
- `validator.go` - 验证逻辑

**优点**：
- 改动小，风险低
- 立即见效

**缺点**：
- 主文件仍然较长（约600行）

### 方案3：保持现状（如果团队习惯）

如果团队习惯单文件结构，且代码可读性良好，可以保持现状。

---

## 🎯 具体优化建议

### 高优先级（立即优化）

1. **拆分 `validateDecision` 函数**
   ```go
   // validator.go
   func validateDecision(d *Decision, ...) error {
       if err := validateAction(d); err != nil {
           return err
       }
       if d.Action == "open_long" || d.Action == "open_short" {
           return validateOpenPosition(d, ...)
       }
       // ...
   }
   
   func validateOpenPosition(d *Decision, ...) error {
       // 验证杠杆、仓位大小、止损止盈、风险回报比
   }
   ```

2. **提取 JSON 解析到单独文件**
   ```go
   // json_parser.go
   func extractDecisions(response string) ([]Decision, error)
   func fixMissingQuotes(jsonStr string) string
   func validateJSONFormat(jsonStr string) error
   ```

### 中优先级（逐步优化）

3. **拆分 `buildSystemPrompt`**
   ```go
   func buildSystemPrompt(...) string {
       template := loadPromptTemplate(templateName)
       constraints := buildHardConstraints(...)
       format := buildOutputFormat(...)
       return template + constraints + format
   }
   ```

4. **拆分 `buildUserPrompt`**
   ```go
   func buildUserPrompt(ctx *Context) string {
       var sb strings.Builder
       sb.WriteString(buildSystemStatusSection(ctx))
       sb.WriteString(buildAccountSection(ctx))
       sb.WriteString(buildPositionsSection(ctx))
       sb.WriteString(buildCandidateCoinsSection(ctx))
       return sb.String()
   }
   ```

### 低优先级（可选）

5. **提取类型定义到 `types.go`**
6. **提取辅助函数到 `utils.go`**

---

## 📈 优化效果预期

### 优化前
- `engine.go`: 878 行
- 函数平均长度: 44 行
- 最长函数: 160 行

### 优化后（方案1）
- `engine.go`: ~200 行（核心流程）
- `prompt_builder.go`: ~250 行
- `json_parser.go`: ~150 行
- `validator.go`: ~200 行
- `types.go`: ~100 行
- 函数平均长度: ~30 行
- 最长函数: ~60 行

---

## ✅ 结论

**当前状态**：代码质量良好，但可以优化

**建议**：
1. **短期**：提取 JSON 解析和验证逻辑到单独文件（方案2）
2. **中期**：按功能拆分文件（方案1）
3. **长期**：保持代码整洁，定期重构

**是否必须优化**：❌ 不是必须的，但优化后会更好维护

