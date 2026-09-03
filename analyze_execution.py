#!/usr/bin/env python3
"""
分析执行日志，找出止损止盈执行情况
"""
import json
import sys
from pathlib import Path
from collections import defaultdict

def analyze_execution(log_dir):
    """分析执行情况"""
    log_path = Path(log_dir)
    records = []
    
    for json_file in sorted(log_path.glob("decision_*.json")):
        try:
            with open(json_file, 'r', encoding='utf-8') as f:
                data = json.load(f)
                records.append(data)
        except:
            continue
    
    if not records:
        print("❌ 没有找到任何交易记录")
        return
    
    print("📊 执行情况详细分析\n")
    print("=" * 80)
    
    # 1. 分析执行日志
    print("\n## 1. 执行日志统计")
    action_stats = defaultdict(int)
    success_count = 0
    fail_count = 0
    
    for record in records:
        exec_log = record.get('execution_log', [])
        for log_entry in exec_log:
            if '成功' in log_entry and '✓' in log_entry:
                success_count += 1
                # 提取action
                parts = log_entry.split()
                if len(parts) >= 3:
                    symbol = parts[1]
                    action = parts[2]
                    action_stats[f"{action}_成功"] += 1
            elif '失败' in log_entry and '❌' in log_entry:
                fail_count += 1
                parts = log_entry.split()
                if len(parts) >= 3:
                    symbol = parts[1]
                    action = parts[2]
                    action_stats[f"{action}_失败"] += 1
    
    print(f"  总成功: {success_count} 次")
    print(f"  总失败: {fail_count} 次")
    print("\n  各操作统计:")
    for action, count in sorted(action_stats.items(), key=lambda x: x[1], reverse=True):
        print(f"    {action}: {count} 次")
    
    # 2. 查找开仓记录
    print("\n## 2. 开仓记录分析")
    open_records = []
    for record in records:
        exec_log = record.get('execution_log', [])
        for log_entry in exec_log:
            if 'open' in log_entry.lower() and '成功' in log_entry:
                open_records.append((record.get('timestamp', '')[:19], log_entry))
    
    print(f"  找到 {len(open_records)} 次开仓成功记录")
    if open_records:
        print("\n  最近10次开仓:")
        for i, (ts, log_entry) in enumerate(open_records[-10:], 1):
            print(f"    {i}. [{ts}] {log_entry}")
    
    # 3. 查找止损止盈相关记录
    print("\n## 3. 止损止盈相关记录")
    stop_loss_records = []
    take_profit_records = []
    
    for record in records:
        exec_log = record.get('execution_log', [])
        timestamp = record.get('timestamp', '')[:19]
        for log_entry in exec_log:
            if '止损' in log_entry:
                stop_loss_records.append((timestamp, log_entry))
            if '止盈' in log_entry:
                take_profit_records.append((timestamp, log_entry))
    
    print(f"  止损相关记录: {len(stop_loss_records)} 条")
    if stop_loss_records:
        print("  最近10条止损记录:")
        for i, (ts, log_entry) in enumerate(stop_loss_records[-10:], 1):
            print(f"    {i}. [{ts}] {log_entry[:80]}")
    
    print(f"\n  止盈相关记录: {len(take_profit_records)} 条")
    if take_profit_records:
        print("  最近10条止盈记录:")
        for i, (ts, log_entry) in enumerate(take_profit_records[-10:], 1):
            print(f"    {i}. [{ts}] {log_entry[:80]}")
    
    # 4. 分析决策中的止损止盈设置
    print("\n## 4. 决策中的止损止盈设置")
    decisions_with_stop = 0
    decisions_without_stop = 0
    
    for record in records:
        decisions = record.get('decisions', [])
        if decisions:
            for dec in decisions:
                action = dec.get('action', '')
                if action in ['open_long', 'open_short']:
                    stop_loss = dec.get('stop_loss', 0)
                    take_profit = dec.get('take_profit', 0)
                    if stop_loss > 0 and take_profit > 0:
                        decisions_with_stop += 1
                    else:
                        decisions_without_stop += 1
    
    total_decisions = decisions_with_stop + decisions_without_stop
    print(f"  开仓决策总数: {total_decisions}")
    print(f"  设置了止损止盈: {decisions_with_stop} 次")
    print(f"  未设置止损止盈: {decisions_without_stop} 次")
    if total_decisions > 0:
        coverage = (decisions_with_stop / total_decisions) * 100
        print(f"  止损止盈覆盖率: {coverage:.2f}%")
    
    # 5. 分析净值归零的原因
    print("\n## 5. 净值归零分析")
    zero_equity_records = []
    for record in records:
        equity = record.get('account_state', {}).get('total_balance', 0)
        if equity == 0.0:
            timestamp = record.get('timestamp', '')[:19]
            error = record.get('error_message', '')
            zero_equity_records.append((timestamp, error))
    
    print(f"  发现 {len(zero_equity_records)} 次净值归零")
    if zero_equity_records:
        print("\n  净值归零记录:")
        for i, (ts, error) in enumerate(zero_equity_records[:10], 1):
            print(f"    {i}. [{ts}]")
            if error:
                print(f"       错误: {error[:100]}")
    
    print("\n" + "=" * 80)
    print("\n🔍 关键发现:")
    print(f"  1. 开仓成功: {len(open_records)} 次")
    print(f"  2. 止损记录: {len(stop_loss_records)} 条")
    print(f"  3. 止盈记录: {len(take_profit_records)} 条")
    print(f"  4. 决策中止损止盈覆盖率: {coverage:.2f}%" if total_decisions > 0 else "  4. 无开仓决策")
    print(f"  5. 净值归零: {len(zero_equity_records)} 次")

if __name__ == '__main__':
    if len(sys.argv) > 1:
        log_dir = sys.argv[1]
    else:
        log_dir = '/Users/huangjunyou/aetheris/decision_logs/hyperliquid_admin_deepseek_1762369142'
    
    analyze_execution(log_dir)

