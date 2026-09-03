#!/usr/bin/env python3
"""
分析关键问题：大亏损和止损止盈执行情况
"""
import json
import sys
from pathlib import Path
from collections import defaultdict

def analyze_critical(log_dir):
    """分析关键问题"""
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
    
    print("🔍 关键问题分析\n")
    print("=" * 80)
    
    # 1. 找出净值大幅下降的时间点
    print("\n## 1. 净值大幅下降分析")
    prev_equity = None
    big_drops = []
    
    for record in records:
        equity = record.get('account_state', {}).get('total_balance', 0)
        timestamp = record.get('timestamp', '')[:19]
        
        if prev_equity is not None and equity < prev_equity:
            drop = prev_equity - equity
            if drop > 1.0:  # 只记录大于1 USDT的下降
                drop_pct = (drop / prev_equity * 100) if prev_equity > 0 else 0
                big_drops.append((timestamp, prev_equity, equity, drop, drop_pct))
        prev_equity = equity
    
    # 排序找出最大的下降
    big_drops.sort(key=lambda x: x[3], reverse=True)
    print(f"  发现 {len(big_drops)} 次大幅下降（>1 USDT）")
    print("\n  前10次最大下降：")
    for i, (ts, prev, curr, drop, pct) in enumerate(big_drops[:10], 1):
        print(f"    {i}. [{ts}] {prev:.2f} → {curr:.2f} USDT (下降 {drop:.2f} USDT, {pct:.2f}%)")
    
    # 2. 分析11月4日07:00附近的大亏损
    print("\n## 2. 11月4日07:00大亏损详细分析")
    critical_records = []
    for record in records:
        timestamp = record.get('timestamp', '')
        if '2025-11-04T07:00' in timestamp or '2025-11-04T06:5' in timestamp:
            critical_records.append(record)
    
    if critical_records:
        print(f"  找到 {len(critical_records)} 条相关记录")
        for i, record in enumerate(critical_records[:5], 1):
            timestamp = record.get('timestamp', '')[:19]
            equity = record.get('account_state', {}).get('total_balance', 0)
            pnl = record.get('account_state', {}).get('total_unrealized_profit', 0)
            positions = record.get('positions', [])
            exec_log = record.get('execution_log', [])
            error = record.get('error_message', '')
            
            print(f"\n  记录 {i}: [{timestamp}]")
            print(f"    净值: {equity:.2f} USDT, 盈亏: {pnl:.2f} USDT")
            print(f"    持仓数: {len(positions) if positions else 0}")
            if positions:
                for pos in positions:
                    print(f"      {pos.get('symbol', 'N/A')} {pos.get('side', 'N/A')}: 盈亏={pos.get('unrealized_profit', 0):.4f}")
            if exec_log:
                print(f"    执行记录: {exec_log[:3]}")
            if error:
                print(f"    错误: {error[:100]}")
    else:
        print("  未找到相关记录")
    
    # 3. 分析止损止盈设置情况
    print("\n## 3. 止损止盈设置分析")
    stop_loss_success = 0
    stop_loss_failed = 0
    take_profit_success = 0
    take_profit_failed = 0
    stop_loss_skipped = 0
    take_profit_skipped = 0
    
    for record in records:
        exec_log = record.get('execution_log', [])
        for log_entry in exec_log:
            if '止损价设置' in log_entry:
                stop_loss_success += 1
            elif '设置止损失败' in log_entry or '⚠ 设置止损失败' in log_entry:
                stop_loss_failed += 1
            elif '跳过设置止损' in log_entry or '跳過設置止損' in log_entry:
                stop_loss_skipped += 1
            elif '止盈价设置' in log_entry:
                take_profit_success += 1
            elif '设置止盈失败' in log_entry or '⚠ 设置止盈失败' in log_entry:
                take_profit_failed += 1
            elif '跳过设置止盈' in log_entry or '跳過設置止盈' in log_entry:
                take_profit_skipped += 1
    
    print(f"  止损设置成功: {stop_loss_success} 次")
    print(f"  止损设置失败: {stop_loss_failed} 次")
    print(f"  止损跳过: {stop_loss_skipped} 次")
    print(f"  止盈设置成功: {take_profit_success} 次")
    print(f"  止盈设置失败: {take_profit_failed} 次")
    print(f"  止盈跳过: {take_profit_skipped} 次")
    
    total_stop_loss_attempts = stop_loss_success + stop_loss_failed + stop_loss_skipped
    total_take_profit_attempts = take_profit_success + take_profit_failed + take_profit_skipped
    
    if total_stop_loss_attempts > 0:
        stop_loss_rate = (stop_loss_success / total_stop_loss_attempts) * 100
        print(f"  止损设置成功率: {stop_loss_rate:.2f}%")
    if total_take_profit_attempts > 0:
        take_profit_rate = (take_profit_success / total_take_profit_attempts) * 100
        print(f"  止盈设置成功率: {take_profit_rate:.2f}%")
    
    # 4. 分析开仓时止损止盈设置情况
    print("\n## 4. 开仓时止损止盈设置情况")
    open_with_stop = 0
    open_without_stop = 0
    
    for record in records:
        decisions = record.get('decisions', [])
        if decisions:
            for dec in decisions:
                action = dec.get('action', '')
                if action in ['open_long', 'open_short']:
                    stop_loss = dec.get('stop_loss', 0)
                    take_profit = dec.get('take_profit', 0)
                    if stop_loss > 0 and take_profit > 0:
                        open_with_stop += 1
                    else:
                        open_without_stop += 1
    
    print(f"  开仓时设置了止损止盈: {open_with_stop} 次")
    print(f"  开仓时未设置止损止盈: {open_without_stop} 次")
    total_opens = open_with_stop + open_without_stop
    if total_opens > 0:
        stop_coverage = (open_with_stop / total_opens) * 100
        print(f"  止损止盈覆盖率: {stop_coverage:.2f}%")
    
    # 5. 分析持仓未平仓情况
    print("\n## 5. 持仓未平仓分析")
    position_history = defaultdict(list)
    
    for record in records:
        positions = record.get('positions', [])
        timestamp = record.get('timestamp', '')[:19]
        if positions:
            for pos in positions:
                symbol = pos.get('symbol', 'N/A')
                side = pos.get('side', 'N/A')
                pnl = pos.get('unrealized_profit', 0)
                key = f"{symbol}_{side}"
                position_history[key].append((timestamp, pnl))
    
    print(f"  发现 {len(position_history)} 个不同的持仓")
    print("\n  持仓时间最长的前5个：")
    longest_positions = sorted(position_history.items(), key=lambda x: len(x[1]), reverse=True)[:5]
    for symbol_side, history in longest_positions:
        if history:
            first_time = history[0][0]
            last_time = history[-1][0]
            first_pnl = history[0][1]
            last_pnl = history[-1][1]
            print(f"    {symbol_side}:")
            print(f"      首次出现: {first_time}, 盈亏: {first_pnl:.4f} USDT")
            print(f"      最后出现: {last_time}, 盈亏: {last_pnl:.4f} USDT")
            print(f"      持续时间: {len(history)} 个周期")
            print(f"      盈亏变化: {last_pnl - first_pnl:.4f} USDT")
    
    print("\n" + "=" * 80)
    print("\n💡 关键发现:")
    if big_drops:
        print(f"  1. 发现 {len(big_drops)} 次大幅下降，最大下降 {big_drops[0][3]:.2f} USDT")
    print(f"  2. 止损设置成功率: {stop_loss_rate:.2f}%" if total_stop_loss_attempts > 0 else "  2. 止损设置数据不足")
    print(f"  3. 开仓时止损止盈覆盖率: {stop_coverage:.2f}%" if total_opens > 0 else "  3. 开仓数据不足")
    print("  4. 需要检查止损止盈是否真正在交易所执行")

if __name__ == '__main__':
    if len(sys.argv) > 1:
        log_dir = sys.argv[1]
    else:
        log_dir = '/Users/huangjunyou/aetheris/decision_logs/hyperliquid_admin_deepseek_1762369142'
    
    analyze_critical(log_dir)

