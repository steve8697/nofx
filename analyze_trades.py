#!/usr/bin/env python3
"""
分析交易记录，找出亏损原因
"""
import json
import os
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path

def analyze_trades(log_dir):
    """分析交易记录"""
    log_path = Path(log_dir)
    if not log_path.exists():
        print(f"❌ 日志目录不存在: {log_dir}")
        return
    
    # 收集所有决策记录
    records = []
    for json_file in sorted(log_path.glob("decision_*.json")):
        try:
            with open(json_file, 'r', encoding='utf-8') as f:
                data = json.load(f)
                records.append(data)
        except Exception as e:
            print(f"⚠️ 读取文件失败 {json_file}: {e}")
            continue
    
    if not records:
        print("❌ 没有找到任何交易记录")
        return
    
    print(f"📊 分析报告：共 {len(records)} 个决策周期\n")
    print("=" * 80)
    
    # 1. 账户净值变化
    print("\n## 1. 账户净值变化")
    first_record = records[0]
    last_record = records[-1]
    
    first_equity = first_record.get('account_state', {}).get('total_balance', 0)
    last_equity = last_record.get('account_state', {}).get('total_balance', 0)
    first_pnl = first_record.get('account_state', {}).get('total_unrealized_profit', 0)
    last_pnl = last_record.get('account_state', {}).get('total_unrealized_profit', 0)
    
    print(f"  起始净值: {first_equity:.2f} USDT (盈亏: {first_pnl:.2f} USDT)")
    print(f"  当前净值: {last_equity:.2f} USDT (盈亏: {last_pnl:.2f} USDT)")
    print(f"  总亏损: {last_equity - first_equity:.2f} USDT")
    if first_equity > 0:
        loss_pct = ((last_equity - first_equity) / first_equity) * 100
        print(f"  亏损百分比: {loss_pct:.2f}%")
    
    # 2. 分析执行记录
    print("\n## 2. 交易执行统计")
    open_trades = 0
    close_trades = 0
    hold_actions = 0
    wait_actions = 0
    errors = 0
    
    for record in records:
        exec_log = record.get('execution_log', [])
        for log_entry in exec_log:
            if '开仓' in log_entry or 'open' in log_entry.lower():
                open_trades += 1
            elif '平仓' in log_entry or 'close' in log_entry.lower():
                close_trades += 1
            elif 'hold' in log_entry.lower():
                hold_actions += 1
            elif 'wait' in log_entry.lower():
                wait_actions += 1
            elif '失败' in log_entry or 'error' in log_entry.lower() or '❌' in log_entry:
                errors += 1
    
    print(f"  开仓次数: {open_trades}")
    print(f"  平仓次数: {close_trades}")
    print(f"  Hold 次数: {hold_actions}")
    print(f"  Wait 次数: {wait_actions}")
    print(f"  错误次数: {errors}")
    
    # 3. 分析持仓情况
    print("\n## 3. 当前持仓分析")
    last_positions = last_record.get('positions', [])
    if last_positions:
        print(f"  当前持仓数: {len(last_positions)}")
        total_unrealized = 0
        for pos in last_positions:
            symbol = pos.get('symbol', 'N/A')
            side = pos.get('side', 'N/A')
            entry = pos.get('entry_price', 0)
            mark = pos.get('mark_price', 0)
            pnl = pos.get('unrealized_profit', 0)
            leverage = pos.get('leverage', 0)
            total_unrealized += pnl
            
            pnl_pct = ((mark - entry) / entry * 100) if entry > 0 else 0
            if side == 'short':
                pnl_pct = -pnl_pct
            
            print(f"    {symbol} {side.upper()}: 入场={entry:.6f}, 当前={mark:.6f}, 盈亏={pnl:.4f} USDT ({pnl_pct:.2f}%), 杠杆={leverage}x")
        print(f"  总未实现盈亏: {total_unrealized:.4f} USDT")
    else:
        print("  当前无持仓")
    
    # 4. 分析决策错误
    print("\n## 4. 决策错误统计")
    error_count = 0
    error_types = defaultdict(int)
    
    for record in records:
        if not record.get('success', True):
            error_count += 1
            error_msg = record.get('error_message', '')
            if '验证失败' in error_msg:
                error_types['验证失败'] += 1
            elif '余额不足' in error_msg or '保证金' in error_msg:
                error_types['余额不足'] += 1
            elif '开仓金额' in error_msg:
                error_types['开仓金额'] += 1
            else:
                error_types['其他'] += 1
    
    print(f"  总错误数: {error_count}")
    for err_type, count in error_types.items():
        print(f"    {err_type}: {count} 次")
    
    # 5. 分析决策类型分布
    print("\n## 5. 决策类型分布")
    action_counts = defaultdict(int)
    for record in records:
        decisions = record.get('decisions', [])
        if decisions:
            for dec in decisions:
                action = dec.get('action', 'unknown')
                action_counts[action] += 1
    
    for action, count in sorted(action_counts.items(), key=lambda x: x[1], reverse=True):
        print(f"  {action}: {count} 次")
    
    # 6. 找出净值最低点
    print("\n## 6. 净值变化趋势")
    equity_history = []
    for record in records:
        equity = record.get('account_state', {}).get('total_balance', 0)
        timestamp = record.get('timestamp', '')
        equity_history.append((timestamp, equity))
    
    if equity_history:
        min_equity = min(equity_history, key=lambda x: x[1])
        max_equity = max(equity_history, key=lambda x: x[1])
        print(f"  最高净值: {max_equity[1]:.2f} USDT ({max_equity[0][:19]})")
        print(f"  最低净值: {min_equity[1]:.2f} USDT ({min_equity[0][:19]})")
        print(f"  最大回撤: {max_equity[1] - min_equity[1]:.2f} USDT")
        if max_equity[1] > 0:
            drawdown_pct = ((max_equity[1] - min_equity[1]) / max_equity[1]) * 100
            print(f"  最大回撤百分比: {drawdown_pct:.2f}%")
    
    print("\n" + "=" * 80)
    print("\n💡 建议:")
    print("  1. 检查止损止盈是否真正执行")
    print("  2. 分析胜率和盈亏比")
    print("  3. 检查是否有过度交易")
    print("  4. 分析哪些币种表现最差")

if __name__ == '__main__':
    if len(sys.argv) > 1:
        log_dir = sys.argv[1]
    else:
        log_dir = '/Users/huangjunyou/aetheris/decision_logs/hyperliquid_admin_deepseek_1762369142'
    
    analyze_trades(log_dir)

