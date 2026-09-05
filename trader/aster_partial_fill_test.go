package trader

import (
	"encoding/json"
	"strconv"
	"testing"
)

// TestAsterIOCPartialFillRemainingCalculation 測試 IOC 回傳 partial fill 時的剩餘數量計算
func TestAsterIOCPartialFillRemainingCalculation(t *testing.T) {
	// 情景 1: IOC 訂單部分成交 30，剩餘 70
	orderResponseJSON := `{
		"symbol": "BTCUSDT",
		"status": "EXPIRED",
		"origQty": "100.00000000",
		"executedQty": "30.00000000"
	}`

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(orderResponseJSON), &result); err != nil {
		t.Fatalf("JSON 解析失敗: %v", err)
	}

	totalQuantity := 100.0
	status, _ := result["status"].(string)
	if status != "EXPIRED" {
		t.Fatalf("預期 EXPIRED，得到: %s", status)
	}

	var executedQty float64
	if eq, ok := result["executedQty"]; ok {
		switch v := eq.(type) {
		case string:
			executedQty, _ = strconv.ParseFloat(v, 64)
		case float64:
			executedQty = v
		}
	}

	remainingQty := totalQuantity - executedQty
	if remainingQty != 70.0 {
		t.Fatalf("預期剩餘數量為 70.0，實際得到: %.8f", remainingQty)
	}

	// 情景 2: IOC 訂單全部成交 (100.0) 但 status 回傳 EXPIRED
	orderResponseFullJSON := `{
		"symbol": "BTCUSDT",
		"status": "EXPIRED",
		"origQty": "100.00000000",
		"executedQty": "100.00000000"
	}`

	var resultFull map[string]interface{}
	if err := json.Unmarshal([]byte(orderResponseFullJSON), &resultFull); err != nil {
		t.Fatalf("JSON 解析失敗: %v", err)
	}

	var executedQtyFull float64
	if eq, ok := resultFull["executedQty"]; ok {
		switch v := eq.(type) {
		case string:
			executedQtyFull, _ = strconv.ParseFloat(v, 64)
		case float64:
			executedQtyFull = v
		}
	}

	remainingQtyFull := totalQuantity - executedQtyFull
	if remainingQtyFull > 1e-8 {
		t.Fatalf("預期全部成交後剩餘 <= 1e-8，實際得到: %.8f", remainingQtyFull)
	}
}
