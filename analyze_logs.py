import json
import glob
import os
import re

log_dir = "decision_logs/aster_admin_deepseek_1762714086/"
files = glob.glob(os.path.join(log_dir, "*.json"))

total_trades = 0
wins = 0
losses = 0
total_pnl = 0.0

print(f"Analyzing {len(files)} logs...")

for filepath in files:
    try:
        with open(filepath, "r") as f:
            content = f.read()
            # Handle concatenated JSONs if any (simple approach)
            # Assuming one JSON object per file for now based on previous cats
            data = json.loads(content)
            
            decisions = data.get("decisions", [])
            for d in decisions:
                action = d.get("action", "")
                
                # Check for active closes
                if action.startswith("close_"):
                    # Try to extract PnL from reasoning text using regex
                    reasoning = d.get("reasoning", "")
                    
                    # Look for "PnL: x.xx" or "亏损-x.xx%" or "盈利+x.xx%"
                    # Patterns seen: "亏损-0.37%", "PnL: -0.37" (hypothetical)
                    
                    pnl_match = re.search(r"(亏损|盈利|PnL)[:\s]+([-+]?[\d\.]+)(%?)", reasoning)
                    
                    pnl = 0.0
                    is_win = False
                    
                    if pnl_match:
                        val_str = pnl_match.group(2)
                        pnl = float(val_str)
                        if "亏损" in pnl_match.group(1) or pnl < 0:
                            losses += 1
                        else:
                            wins += 1
                            is_win = True
                        
                        # Adjust sign if it was "亏损" and number was positive (e.g. 亏损 0.37%)
                        if "亏损" in pnl_match.group(1) and pnl > 0:
                            pnl = -pnl
                            
                        total_pnl += pnl
                        total_trades += 1
                        print(f"Trade: {d.get('symbol')} | {action} | PnL: {pnl:.2f}% | Reason: {reasoning[:50]}...")
                    
                    else:
                        print(f"Trade: {d.get('symbol')} | {action} | PnL: UNKNOWN | Reason: {reasoning[:50]}...")
                        # Assume loss if "stop" or "loss" in reasoning?
                        if "loss" in reasoning.lower() or "止损" in reasoning or "亏损" in reasoning:
                           losses += 1
                           total_trades += 1
                        elif "profit" in reasoning.lower() or "止盈" in reasoning or "盈利" in reasoning:
                           wins += 1
                           total_trades += 1

            
            # Check for passive closes in execution log or decision log?
            # The previous 'detectAndLogPassiveClose' logs to decisionLogger, so it should be in a file.
            # But the 'Action' in decision structure for passive close might be 'passive_close'.
            
            # Let's check for "passive_close" action
            if decisions:
                 for d in decisions:
                     if d.get("action") == "passive_close":
                         reasoning = d.get("reasoning", "")
                         pnl_match = re.search(r"PnL: ([-+]?[\d\.]+)", reasoning)
                         if pnl_match:
                             pnl = float(pnl_match.group(1))
                             total_pnl += pnl
                             if pnl > 0:
                                 wins += 1
                             else:
                                 losses += 1
                             total_trades += 1
                             print(f"Passive: {d.get('symbol')} | PnL: {pnl:.2f} | Reason: {reasoning[:50]}...")

    except Exception as e:
        # print(f"Error reading {filepath}: {e}")
        pass

if total_trades > 0:
    win_rate = (wins / total_trades) * 100
    avg_pnl = total_pnl / total_trades
    print("\n" + "="*30)
    print(f"Total Trades: {total_trades}")
    print(f"Wins: {wins}")
    print(f"Losses: {losses}")
    print(f"Win Rate: {win_rate:.2f}%")
    print(f"Total PnL (Sum of %): {total_pnl:.2f}%")
    print(f"Avg PnL per Trade: {avg_pnl:.2f}%")
    print("="*30)
else:
    print("No trades found.")
