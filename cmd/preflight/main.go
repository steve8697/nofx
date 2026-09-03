package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"aetheris/config"
	"aetheris/decision"
	"aetheris/mcp"
	"aetheris/trader"
)

func main() {
	dbPath := flag.String("db", "config.db", "SQLite 配置库路径")
	pingAI := flag.Bool("ai", false, "额外打一发极短的 AI 请求（会消耗 token）")
	flag.Parse()

	fmt.Println("NOFX preflight — 只读检查，不会下单、不会启动交易循环")
	fmt.Println()

	failed := 0
	check := func(name string, err error) {
		if err != nil {
			failed++
			fmt.Printf("FAIL  %s: %v\n", name, err)
			return
		}
		fmt.Printf("PASS  %s\n", name)
	}

	if _, err := os.Stat(*dbPath); err != nil {
		check("打开 "+*dbPath, err)
		os.Exit(1)
	}

	db, err := config.NewDatabase(*dbPath)
	if err != nil {
		check("读取配置库", err)
		os.Exit(1)
	}
	defer db.Close()
	check("读取配置库", nil)

	if err := decision.ReloadPromptTemplates(); err != nil {
		check("加载 prompts/", err)
	} else if _, err := decision.GetPromptTemplate("adaptive"); err != nil {
		check("prompts/adaptive.md", err)
	} else {
		check("prompts/adaptive.md", nil)
	}

	traders, err := db.GetAllTraders()
	if err != nil {
		check("列出交易员", err)
		os.Exit(1)
	}
	if len(traders) == 0 {
		check("列出交易员", fmt.Errorf("没有任何交易员"))
	} else {
		check(fmt.Sprintf("列出交易员 (%d)", len(traders)), nil)
	}

	for _, tr := range traders {
		label := fmt.Sprintf("%s [%s]", tr.Name, tr.ID)
		if tr.IsRunning {
			fmt.Printf("WARN  %s is_running=1（进程启动不会自动开仓，除非你手动 start）\n", label)
		} else {
			fmt.Printf("INFO  %s is_running=0\n", label)
		}

		exchanges, err := db.GetExchanges(tr.UserID)
		if err != nil {
			check(label+" 交易所配置", err)
			continue
		}
		var ex *config.ExchangeConfig
		for _, item := range exchanges {
			if item.ID == tr.ExchangeID {
				ex = item
				break
			}
		}
		if ex == nil {
			check(label+" 交易所配置", fmt.Errorf("找不到 exchange_id=%s", tr.ExchangeID))
			continue
		}
		if !ex.Enabled {
			check(label+" 交易所启用", fmt.Errorf("%s 未启用", ex.ID))
			continue
		}

		client, err := newReadOnlyClient(ex)
		if err != nil {
			check(label+" 初始化交易所客户端", err)
			continue
		}
		check(label+" 初始化交易所客户端", nil)

		balance, err := client.GetBalance()
		if err != nil {
			check(label+" GetBalance", err)
		} else {
			wallet, _ := balance["totalWalletBalance"].(float64)
			avail, _ := balance["availableBalance"].(float64)
			fmt.Printf("PASS  %s GetBalance wallet=%.4f available=%.4f\n", label, wallet, avail)
		}

		positions, err := client.GetPositions()
		if err != nil {
			check(label+" GetPositions", err)
		} else {
			fmt.Printf("PASS  %s GetPositions count=%d\n", label, len(positions))
		}

		price, err := client.GetMarketPrice("BTCUSDT")
		if err != nil {
			check(label+" GetMarketPrice(BTCUSDT)", err)
		} else {
			fmt.Printf("PASS  %s GetMarketPrice(BTCUSDT)=%.4f\n", label, price)
		}

		models, err := db.GetAIModels(tr.UserID)
		if err != nil {
			check(label+" AI 配置", err)
			continue
		}
		var model *config.AIModelConfig
		for _, item := range models {
			if item.ID == tr.AIModelID {
				model = item
				break
			}
		}
		if model == nil {
			check(label+" AI 配置", fmt.Errorf("找不到 ai_model_id=%s", tr.AIModelID))
			continue
		}
		settings := model.ClientSettings()
		if settings.APIKey == "" {
			check(label+" AI API key", fmt.Errorf("未设置（api_key 与 env_key 皆空）"))
			continue
		}
		fmt.Printf("PASS  %s AI key 已配置 provider=%s model=%s url=%s\n", label, model.Provider, settings.ModelName, settings.BaseURL)

		if *pingAI {
			ai := mcp.New()
			switch settings.Kind {
			case "qwen":
				ai.SetQwenAPIKey(settings.APIKey, settings.BaseURL, settings.ModelName)
			case "custom":
				ai.SetCustomAPI(settings.BaseURL, settings.APIKey, settings.ModelName)
			default:
				ai.SetDeepSeekAPIKey(settings.APIKey, settings.BaseURL, settings.ModelName)
			}
			reply, err := ai.CallWithMessages("Reply with the single word PONG.", "ping")
			if err != nil {
				check(label+" AI ping", err)
			} else if strings.TrimSpace(reply) == "" {
				check(label+" AI ping", fmt.Errorf("空响应"))
			} else {
				preview := strings.ReplaceAll(strings.TrimSpace(reply), "\n", " ")
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				fmt.Printf("PASS  %s AI ping: %s\n", label, preview)
			}
		}
	}

	fmt.Println()
	if failed > 0 {
		fmt.Printf("preflight 结束: %d 项失败。未下任何订单。\n", failed)
		os.Exit(1)
	}
	fmt.Println("preflight 结束: 只读 API 均通过。未下任何订单。")
}

func newReadOnlyClient(ex *config.ExchangeConfig) (trader.Trader, error) {
	kind := strings.ToLower(ex.ID + " " + ex.Name + " " + ex.Type)
	switch {
	case strings.Contains(kind, "aster"):
		return trader.NewAsterTrader(ex.AsterUser, strings.TrimSpace(ex.AsterSigner), ex.AsterPrivateKey, ex.Testnet)
	case strings.Contains(kind, "hyperliquid"):
		return trader.NewHyperliquidTrader(ex.APIKey, ex.HyperliquidWalletAddr, ex.Testnet)
	case strings.Contains(kind, "binance"):
		return trader.NewFuturesTrader(ex.APIKey, ex.SecretKey), nil
	default:
		return nil, fmt.Errorf("不支持的交易所 id=%s type=%s", ex.ID, ex.Type)
	}
}
