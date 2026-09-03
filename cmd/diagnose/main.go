package main

import (
	"log"

	"aetheris/market"
)

func main() {
	log.Println("🔍 Starting System Diagnostics...")
	log.Println("Target: Verify Data Pipeline & AI Prompt Visibility")

	symbol := "BTCUSDT"

	log.Println("Delegating to market.RunDiagnostics...")
	market.RunDiagnostics(symbol)
}
