package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"aetheris/market"
	"regexp"
	"strings"
)

// 预编译正则表达式（JSON解析专用）
var (
	// ✅ 安全的正則：精確匹配 ```json 代碼塊
	// 使用反引號 + 拼接避免轉義問題
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")
)

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, positions []PositionInfo, marketDataMap map[string]*market.Data, minRiskRewardRatio float64, maxPositions int, feeInfo *TradingFeeInfo) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w", err)
	}

	// 3. 逐条过滤未通过验证的开仓；平仓/wait 尽量保留，避免整批 abort。
	kept := sanitizeDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, positions, marketDataMap, minRiskRewardRatio, maxPositions, feeInfo)
	if len(kept) == 0 {
		kept = []Decision{{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: "全部决策未通过验证，本周期不执行开仓",
		}}
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: kept,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 1. 尝试匹配 ```json 代码块
	if loc := reJSONFence.FindStringIndex(response); loc != nil {
		return strings.TrimSpace(response[:loc[0]])
	}

	// 2. 尝试匹配 JSON 数组 [...]
	// 使用 FindAllStringIndex 找到最后一个匹配项，或者更严谨地，找到看起来像决策数组的那个
	// 由于 extractDecisions 使用的是 FindString (即第一个匹配项)，我们也应该保持一致
	// 但为了避免匹配到 CoT 中的小数组（如 [1, 2, 3]），我们依赖 reJSONArray 的正则
	// reJSONArray = `(?is)\[\s*\{.*?\}\s*\]` 要求数组内部是对象 {...}
	// 这能有效过滤掉纯数字数组
	if loc := reJSONArray.FindStringIndex(response); loc != nil {
		return strings.TrimSpace(response[:loc[0]])
	}

	// 3. 如果都找不到，尝试查找最后一个 "[" 且后面跟着 "{" 的位置 (最后的手段)
	// 这可以处理一些正则没匹配到但确实是 JSON 的情况
	lastJsonStart := -1
	for i := 0; i < len(response); i++ {
		// 查找 [...{... 结构
		if response[i] == '[' {
			// 检查后面是否有 {
			for j := i + 1; j < len(response) && j < i+20; j++ { // 只向后查20个字符
				if response[j] == '{' {
					// 找到了可能的 JSON 开始，记录位置
					// 但我们想要的是*最后*一个这样的结构吗？通常决策在最后。
					// 或者我们想要的是第一个*决策*结构。
					// 如果 CoT 里引用了代码示例 `[{"a":1}]`，这也会误判。
					// 但通常 CoT 里的数组是 `[1, 2, 3]`。
					// 让我们保守一点：如果正则没匹配到，说明格式很乱。
					// 我们假设最后一个 `[` 且后面紧跟 `{` 的是 JSON。
					lastJsonStart = i
					break
				} else if response[j] != ' ' && response[j] != '\n' && response[j] != '\t' && response[j] != '\r' {
					// 遇到非空白字符且不是 {，说明不是对象数组的开始
					break
				}
			}
		}
	}

	if lastJsonStart > 0 {
		return strings.TrimSpace(response[:lastJsonStart])
	}

	// 如果找不到任何 JSON 迹象，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]Decision, error) {
	// 预清洗：去零宽/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔍 調試日誌：記錄 AI 原始響應
	responseLen := len(s)
	responsePreview := s
	if len(responsePreview) > 500 {
		responsePreview = responsePreview[:500] + "..."
	}
	log.Printf("📥 [Parser] AI 響應長度: %d 字符", responseLen)
	if responseLen < 100 {
		log.Printf("⚠️  [Parser] AI 響應內容（完整）: %s", s)
	} else {
		log.Printf("📥 [Parser] AI 響應預覽（前 500 字符）: %s", responsePreview)
	}

	// 🔧 关键修复 (Critical Fix)：在正则匹配之前就先修复全角字符！
	// 否则正则表达式 \[ 无法匹配全角的 ［
	s = fixMissingQuotes(s)

	var decisions []Decision

	// 1) 优先从 ```json 代码块中提取
	if m := reJSONFence.FindStringSubmatch(s); len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // 把 "[ {" 规整为 "[{"
		jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
		}
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
		}
	} else {
		// 2) 退而求其次 (Fallback)：全文寻找对象数组
		// 注意：此时 s 已经过 fixMissingQuotes()，全角字符已转换为半角
		// 🔧 关键修复 (Critical Fix): 使用 FindAllString 取最后一个匹配项！
		// AI 可能在思维链 (CoT) 中打草稿 (draft) 或引用 JSON 示例，导致 FindString 抓取到前面的无效/不完整 JSON
		// 真正的决策通常在响应的最后
		matches := reJSONArray.FindAllString(s, -1)
		var jsonContent string
		if len(matches) > 0 {
			jsonContent = strings.TrimSpace(matches[len(matches)-1])
		}

		if jsonContent == "" {
			// 🔧 安全回退 (Safe Fallback)：当AI只输出思维链没有JSON时，生成保底决策（避免系统崩溃）
			log.Printf("⚠️  [SafeFallback] AI未输出JSON决策，进入安全等待模式 (AI response without JSON, entering safe wait mode)")

			// 提取思维链摘要（最多 240 字符）
			cotSummary := s
			if len(cotSummary) > 240 {
				cotSummary = cotSummary[:240] + "..."
			}

			// 生成保底决策：所有币种进入 wait 状态
			fallbackDecision := Decision{
				Symbol:    "ALL",
				Action:    "wait",
				Reasoning: fmt.Sprintf("模型未输出结构化JSON决策，进入安全等待；摘要：%s", cotSummary),
			}

			return []Decision{fallbackDecision}, nil
		}

		// 🔧 规整格式（此时全角字符已在前面修复过）
		jsonContent = compactArrayOpen(jsonContent)
		jsonContent = fixMissingQuotes(jsonContent)         // 二次修复（防止 regex 提取后还有残留全角）
		jsonContent = stripJSONComments(jsonContent)        // 🔧 去除注释（如 // ...）
		jsonContent = fixArithmeticExpressions(jsonContent) // 🔧 修复算术表达式（如 "risk_usd": 12 * 3 = 36）

		// 🔧 验证 JSON 格式（检测常见错误）
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
		}

		// 解析JSON
		var rawDecisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &rawDecisions); err != nil {
			return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
		}

		// 过滤掉空决策（Symbol和Action都为空的）
		for _, d := range rawDecisions {
			if d.Symbol == "" && d.Action == "" {
				continue
			}
			decisions = append(decisions, d)
		}
	}

	// 🔧 智能字段映射 (Smart Field Mapping)
	// AI 可能在 update_stop_loss 时输出 "stop_loss" 而不是 "new_stop_loss"
	// 我们在这里做一次兼容性处理
	applySmartFieldMapping(decisions)

	// 🔧 过滤掉 AI 复制的示例 JSON 决策
	decisions = filterDemoDecisions(decisions)

	return decisions, nil
}

// applySmartFieldMapping 智能字段映射
// AI 可能在 update_stop_loss 时输出 "stop_loss" 而不是 "new_stop_loss"
// 同样，update_take_profit 时可能输出 "take_profit" 而不是 "new_take_profit"
// 这个函数做兼容性处理，确保验证器能正确读取这些字段
func applySmartFieldMapping(decisions []Decision) {
	for i := range decisions {
		d := &decisions[i]
		// 别名字段映射: profit_target -> take_profit
		if d.TakeProfit == 0 && d.ProfitTarget > 0 {
			d.TakeProfit = d.ProfitTarget
			log.Printf("  🔧 智能映射: %s 使用 profit_target=%.2f 填充 take_profit", d.Symbol, d.ProfitTarget)
		}
		if d.Action == "update_stop_loss" {
			if d.NewStopLoss == 0 && d.StopLoss > 0 {
				d.NewStopLoss = d.StopLoss
				log.Printf("  🔧 智能映射: update_stop_loss 使用 stop_loss=%.2f 填充 new_stop_loss", d.StopLoss)
			}
		}
		if d.Action == "update_take_profit" {
			if d.NewTakeProfit == 0 && d.TakeProfit > 0 {
				d.NewTakeProfit = d.TakeProfit
				log.Printf("  🔧 智能映射: update_take_profit 使用 take_profit=%.2f 填充 new_take_profit", d.TakeProfit)
			} else if d.NewTakeProfit == 0 && d.ProfitTarget > 0 {
				d.NewTakeProfit = d.ProfitTarget
				log.Printf("  🔧 智能映射: update_take_profit 使用 profit_target=%.2f 填充 new_take_profit", d.ProfitTarget)
			}
		}
	}
}

// fixMissingQuotes 替换中文引号和全角字符为英文引号和半角字符（避免AI输出全角JSON字符导致解析失败）
func fixMissingQuotes(jsonStr string) string {
	// 替换中文引号
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ 替换全角括号、冒号、逗号（防止AI输出全角JSON字符）
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B 全角左方括号
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D 全角右方括号
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B 全角左花括号
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D 全角右花括号
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A 全角冒号
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C 全角逗号

	// ⚠️ 替换CJK标点符号（AI在中文上下文中也可能输出这些）
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK左方头括号 U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK右方头括号 U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK左龟壳括号 U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK右龟壳括号 U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK顿号 U+3001

	// ⚠️ 替换全角空格为半角空格（JSON中不应该有全角空格）
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 全角空格

	return jsonStr
}

// validateJSONFormat 验证 JSON 格式，检测常见错误
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// 允许 [ 和 { 之间存在任意空白（含零宽）
	if !reArrayHead.MatchString(trimmed) {
		// 检查是否是纯数字/范围数组（常见错误）
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("不是有效的决策数组（必须包含对象 {}），实际内容: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON 必须以 [{ 开头（允许空白），实际: %s", trimmed[:min(20, len(trimmed))])
	}

	// 检查是否在数值字段中包含范围符号 ~（LLM 常见错误）
	// 允许在字符串值（如reasoning字段）中使用~符号，但禁止在数值字段中使用
	// 🔧 修正（v5.6.1）：仅当 ~ 符号出现在字符串值外部的键值位置（冒号后）才视为错误，避免 reasoning 中正常的 "79000~80000" 误触
	reNumericRange := regexp.MustCompile(`:\s*(-?\d+\.?\d*)\s*~\s*(-?\d+\.?\d*)`)
	if matches := reNumericRange.FindAllStringIndex(jsonStr, -1); matches != nil {
		for _, match := range matches {
			colonPos := match[0]
			// 检查是否位于字符串内部
			quoteCount := 0
			for i := 0; i < colonPos; i++ {
				if jsonStr[i] == '"' && (i == 0 || jsonStr[i-1] != '\\') {
					quoteCount++
				}
			}
			if quoteCount%2 == 1 {
				continue // 位于字符串内，允许
			}

			// 检查冒号后是否是引号
			valueStart := colonPos + 1
			for valueStart < len(jsonStr) && (jsonStr[valueStart] == ' ' || jsonStr[valueStart] == '\t') {
				valueStart++
			}
			if valueStart < len(jsonStr) && jsonStr[valueStart] == '"' {
				continue
			}

			return fmt.Errorf("JSON 数值字段中不可包含范围符号 ~，所有数字必须是精确的单一值（reasoning字段中的文本描述可以使用~）")
		}
	}

	// 检查是否在数值字段中包含千位分隔符（如 98,000）
	// ⚠️ 重要：只检查数值字段中的千位分隔符，字符串值（如reasoning字段）中的千位分隔符是允许的
	// 🔧 修正（v5.6.1）：同样增加字符串上下文判断，防止 reasoning 内部的 ":80,000" 触发误报
	reNumericComma := regexp.MustCompile(`:\s*(-?\d+),(\d{3})([,\s\]\}])`)
	if matches := reNumericComma.FindAllStringIndex(jsonStr, -1); matches != nil {
		for _, match := range matches {
			colonPos := match[0] // 冒号的位置

			// 检查是否位于字符串内部
			quoteCount := 0
			for i := 0; i < colonPos; i++ {
				if jsonStr[i] == '"' && (i == 0 || jsonStr[i-1] != '\\') {
					quoteCount++
				}
			}
			if quoteCount%2 == 1 {
				continue // 位于字符串内，跳过
			}

			// 检查冒号后是否有引号：如果有引号，说明是字符串值，跳过
			valueStart := colonPos + 1
			for valueStart < len(jsonStr) && (jsonStr[valueStart] == ' ' || jsonStr[valueStart] == '\t') {
				valueStart++
			}

			if valueStart < len(jsonStr) && jsonStr[valueStart] == '"' {
				continue
			}

			// 这里已经匹配到了数值字段中的千位分隔符，报错
			return fmt.Errorf("JSON 数值字段中不可包含千位分隔符逗号，发现: %s", jsonStr[colonPos:min(colonPos+15, len(jsonStr))])
		}
	}

	// ⚠️ 关键修复：检查是否在数值字段中包含算术表达式（如 26.42 * (3/5) 或 34.24 - (34.24 * 0.01)）
	// 这是 AI 常见错误：在 JSON 数值字段中输出计算过程而不是最终结果
	// 匹配模式：冒号后跟着数字和算术运算符（+, -, *, /, (, )）的组合
	// 例如："position_size_usd": 26.42 * (3/5) 或 "stop_loss": 3056.67 - (3056.67 * 0.01)
	//
	// 🔧 修正（v5.6.0）：旧版本仅简单跳过「冒号后紧跟引号」的情况，但 reasoning 字段中的文本
	// 如 "XPLUSDT做多:7/8多空确认一致" 中的冒号不是 JSON key-value 分隔符，而是字符串内容。
	// 旧正则会误将其匹配为数值字段的算术表达式，导致合法交易被拦截。
	// 新方案：通过追踪 JSON 引号开闭状态（lexer），确保只检查真正位于字符串外部的冒号。
	reArithmeticExpr := regexp.MustCompile(`:\s*(-?\d+\.?\d*)\s*[\+\-\*/\(]`)
	if matches := reArithmeticExpr.FindAllStringIndex(jsonStr, -1); matches != nil {
		for _, match := range matches {
			colonPos := match[0] // 冒号的位置

			// 🔧 关键修正：检查该冒号是否位于 JSON 字符串值内部
			// 通过计算 colonPos 之前的未转义引号数量来判断
			// 奇数个引号 = 在字符串内部（跳过），偶数个 = 在字符串外部（需要检查）
			quoteCount := 0
			for i := 0; i < colonPos; i++ {
				if jsonStr[i] == '"' && (i == 0 || jsonStr[i-1] != '\\') {
					quoteCount++
				}
			}
			if quoteCount%2 == 1 {
				// 该冒号位于 JSON 字符串值内部（如 reasoning 字段），跳过
				continue
			}

			// 冒号位于字符串外部，这是 JSON key-value 分隔符
			// 检查冒号后是否有引号：如果有引号，说明是字符串值，跳过
			valueStart := colonPos + 1
			for valueStart < len(jsonStr) && (jsonStr[valueStart] == ' ' || jsonStr[valueStart] == '\t') {
				valueStart++
			}

			// 如果冒号后是引号，说明这是字符串值，跳过（允许字符串值中的算术表达式）
			if valueStart < len(jsonStr) && jsonStr[valueStart] == '"' {
				continue
			}

			// 提取错误的算术表达式片段（最多50个字符）
			exprEnd := min(match[1]+50, len(jsonStr))
			// 找到下一个逗号或右括号作为表达式结束
			for i := match[1]; i < exprEnd; i++ {
				if jsonStr[i] == ',' || jsonStr[i] == '}' || jsonStr[i] == ']' {
					exprEnd = i
					break
				}
			}
			errorSnippet := strings.TrimSpace(jsonStr[colonPos:exprEnd])

			return fmt.Errorf("JSON 数值字段中不可包含算术表达式，必须是计算后的最终数值！发现: %s", errorSnippet)
		}
	}

	return nil
}

// removeInvisibleRunes 去除零宽字符和 BOM，避免肉眼看不见的前缀破坏校验
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen 规整开头的 "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fixArithmeticExpressions 修复JSON数值字段中的算术表达式
// 例如将 "risk_usd": 12.70 * (34.31 - 3145.00 / 34.31) = 27.98 修复为 "risk_usd": 27.98
func fixArithmeticExpressions(jsonStr string) string {
	// 1. 处理带等号的表达式：提取等号后面的数值
	// 匹配模式：冒号 -> 空白 -> 任意非逗号/右括号字符 -> 等号 -> 空白 -> 数值
	// 例如：: 1+1 = 2
	// 注意：排除引号内的内容（虽然正则很难完美做到，但简单排除",}可以覆盖大部分情况）
	reEquals := regexp.MustCompile(`:\s*[^",\}\]]*?=\s*(-?\d+\.?\d*)`)
	jsonStr = reEquals.ReplaceAllString(jsonStr, ": $1")

	return jsonStr
}

// stripJSONComments 去除JSON字符串中的注释（支持 // 和 /* */）
func stripJSONComments(jsonStr string) string {
	// 1. 去除多行注释 /* ... */
	reBlockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	jsonStr = reBlockComment.ReplaceAllString(jsonStr, "")

	// 2. 去除单行注释 // ...
	// 注意：需要避免匹配 URL 中的 // (如 http://) 或字符串中的 //
	// 简单策略：匹配行尾的 //，且前面有空白或其他分隔符
	// 更好的策略：逐行处理，如果 // 不在引号内，则去除

	lines := strings.Split(jsonStr, "\n")
	var cleanLines []string

	for _, line := range lines {
		// 查找 // 的位置
		idx := strings.Index(line, "//")
		if idx == -1 {
			cleanLines = append(cleanLines, line)
			continue
		}

		// 检查 // 是否在引号内
		// 简单检查：统计 // 前面的双引号数量，如果是偶数，说明不在引号内（或者引号已闭合）
		// 这不是完美方案，但对于 JSON 来说通常足够

		// 实际上 strings.Count(prefix, "\"") 包含了 \" 中的 "
		// 我们只关心未转义的 "
		// 让我们用更简单的方法：遍历字符

		inString := false
		commentStart := -1

		for i := 0; i < len(line); i++ {
			char := line[i]

			if char == '"' && (i == 0 || line[i-1] != '\\') {
				inString = !inString
			}

			if !inString && i+1 < len(line) && char == '/' && line[i+1] == '/' {
				commentStart = i
				break
			}
		}

		if commentStart != -1 {
			cleanLines = append(cleanLines, strings.TrimRight(line[:commentStart], " \t"))
		} else {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// filterDemoDecisions 过滤掉 AI 直接复制 Prompt 示例的决策
func filterDemoDecisions(decisions []Decision) []Decision {
	var cleanDecisions []Decision
	for _, d := range decisions {
		upperSymbol := strings.ToUpper(d.Symbol)
		
		// 1. 过滤 DEMO_ 前缀币种
		if strings.HasPrefix(upperSymbol, "DEMO_") {
			log.Printf("⚠️  [Parser] 过滤掉模拟示例币种决策: %s %s", d.Symbol, d.Action)
			continue
		}
		
		// 2. 过滤完全复制的示例特征值
		// BTCUSDT open_short, risk_usd >= 100 且 stop_loss 97000（模板硬编码）
		isBtcExample := (upperSymbol == "BTCUSDT") && (d.Action == "open_short") && (d.RiskUSD >= 100 && d.StopLoss == 97000)
		// SOLUSDT open_long, risk_usd >= 30 且 stop_loss 150（模板硬编码）
		isSolExample := (upperSymbol == "SOLUSDT") && (d.Action == "open_long") && (d.RiskUSD >= 30 && d.StopLoss == 150)
		// ETHUSDT close_long reasoning 完全匹配 "止盈离场"（模板硬编码示例）或包含 "示例数据"
		// ⚠️ 防止误杀 AI 真实的平仓推理句子（例如包含 "触及阻力位，止盈离场锁定利润"）
		isEthExample := (upperSymbol == "ETHUSDT") && (d.Action == "close_long") && (strings.TrimSpace(d.Reasoning) == "止盈离场" || strings.Contains(d.Reasoning, "示例数据"))
		
		if isBtcExample || isSolExample || isEthExample {
			log.Printf("⚠️  [Parser] 过滤掉高度疑似复制示例的决策: %+v", d)
			continue
		}
		
		cleanDecisions = append(cleanDecisions, d)
	}
	
	// 如果过滤后无有效决策，返回一个默认 wait 决策
	if len(cleanDecisions) == 0 {
		log.Printf("📋 [Parser] 所有决策均被过滤或为空，回退到默认 wait 状态")
		cleanDecisions = append(cleanDecisions, Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: "系统自动过滤模拟示例决策，无实际有效交易动作",
		})
	}
	
	return cleanDecisions
}
