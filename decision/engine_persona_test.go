package decision

import (
	"strings"
	"testing"
)

func TestDynamicPersonaInDrawdown(t *testing.T) {
	// 1. 設置帳戶處於回撤期 PnL = -3.5%
	ctx := &Context{
		Account: AccountInfo{
			TotalEquity:      21.0,
			AvailableBalance: 20.0,
			TotalPnLPct:      -3.5,
		},
	}

	prompt := buildUserPrompt(ctx)

	// 驗證 1: 必須包含防禦模式提示
	if !strings.Contains(prompt, "CAPITAL DEFENSE MODE (防禦避險模式)") {
		t.Fatalf("回撤期應注入 CAPITAL DEFENSE MODE，但未找到！Prompt:\n%s", prompt)
	}

	// 驗證 2: 必須包含防禦性倉位要求
	if !strings.Contains(prompt, "DEFENSIVE Sizing (CUT RISK IN HALF)") {
		t.Fatalf("回撤期應要求減半倉位，但未找到！")
	}

	// 驗證 3: 絕不能包含賭徒心態詞彙
	gamblerTerms := []string{
		"do not shrink",
		"conviction to recover",
		"SNIPER RECOVERY TEAM",
	}
	for _, term := range gamblerTerms {
		if strings.Contains(prompt, term) {
			t.Fatalf("嚴重錯誤：回撤期提示詞依然殘留賭徒心態詞彙: %s", term)
		}
	}
}
