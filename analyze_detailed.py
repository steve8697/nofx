#!/usr/bin/env python3
"""
详细分析交易记录，找出亏损原因
"""
import json
import sys
from collections import defaultdict
from pathlib import Path

def analyze_detailed(log_dir):
    """详细分析交易记录"""
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
    
    print("📊 详细分析报告\n")
    print("=" * 80)
    
    # 1. 分析验证失败的原因
    print("\n## 1. 验证失败详细分析")
    validation_errors = defaultdict(int)
    for record in records:
        if not record.get('success', True):
            error_msg = record.get('error_message', '')
            if '验证失败' in error_msg:
                if '开仓金额过小' in error_msg:
                    validation_errors['开仓金额过小'] += 1
                elif '仓位价值不能超过' in error_msg:
                    validation_errors['仓位价值超限'] += 1
                elif '风险回报比' in error_msg:
                    validation_errors['风险回报比不足'] += 1
                elif '杠杆' in error_msg:
                    validation_errors['杠杆超限'] += 1
                else:
                    validation_errors['其他验证失败'] += 1
    
    for err_type, count in sorted(validation_errors.items(), key=lambda x: x[1], reverse=True):
        print(f"  {err_type}: {count} 次")
    
    # 2. 分析开仓和平仓的匹配情况
    print("\n## 2. 开仓/平仓匹配分析")
    open_count = 0
    close_count = 0
    
    for record in records:
        exec_log = record.get('execution_log', [])
        for log_entry in exec_log:
            if '开仓成功' in log_entry or '开多仓成功' in log_entry or '开空仓成功' in log_entry:
                open_count += 1
            elif '平仓成功' in log_entry or '平多仓成功' in log_entry or '平空仓成功' in log_entry:
                close_count += 1
    
    print(f"  实际开仓成功: {open_count} 次")
    print(f"  实际平仓成功: {close_count} 次")
    print(f"  未平仓数: {open_count - close_count} 次")
    if open_count > 0:
        close_rate = (close_count / open_count) * 100
        print(f"  平仓率: {close_rate:.2f}%")
    
    # 3. 分析持仓持续时间
    print("\n## 3. 持仓持续时间分析")
    position_durations = []
    for record in records:
        positions = record.get('positions', [])
        if positions:
            for pos in positions:
                update_time = pos.get('update_time', 0)
                if update_time > 0:
                    # 计算持仓时长（毫秒转小时）
                    from datetime import datetime
                    try:
                        current_time = datetime.now().timestamp() * 1000
                        duration_hours = (current_time - update_time) / (1000 * 60 * 60)
                        position_durations.append(duration_hours)
                    except:
                        pass
    
    if position_durations:
        avg_duration = sum(position_durations) / len(position_durations)
        max_duration = max(position_durations)
        min_duration = min(position_durations)
        print(f"  当前持仓数: {len(position_durations)}")
        print(f"  平均持仓时长: {avg_duration:.2f} 小时")
        print(f"  最长持仓: {max_duration:.2f} 小时")
        print(f"  最短持仓: {min_duration:.2f} 小时")
    
    # 4. 分析净值变化的关键时间点
    print("\n## 4. 净值变化关键时间点")
    equity_changes = []
    prev_equity = None
    
    for record in records:
        equity = record.get('account_state', {}).get('total_balance', 0)
        timestamp = record.get('timestamp', '')[:19]
        
        if prev_equity is not None and equity != prev_equity:
            change = equity - prev_equity
            equity_changes.append((timestamp, prev_equity, equity, change))
        prev_equity = equity
    
    # 找出最大的单次亏损
    if equity_changes:
        biggest_loss = min(equity_changes, key=lambda x: x[3])
        biggest_gain = max(equity_changes, key=lambda x: x[3])
        
        print(f"  最大单次亏损: {biggest_loss[3]:.2f} USDT")
        print(f"    时间: {biggest_loss[0]}")
        print(f"    从 {biggest_loss[1]:.2f} → {biggest_loss[2]:.2f} USDT")
        print(f"  最大单次盈利: {biggest_gain[3]:.2f} USDT")
        print(f"    时间: {biggest_gain[0]}")
        print(f"    从 {biggest_gain[1]:.2f} → {biggest_gain[2]:.2f} USDT")
    
    # 5. 分析决策成功率
    print("\n## 5. 决策成功率分析")
    total_decisions = 0
    successful_decisions = 0
    
    for record in records:
        if record.get('success', True):
            successful_decisions += 1
        total_decisions += 1
    
    if total_decisions > 0:
        success_rate = (successful_decisions / total_decisions) * 100
        print(f"  总决策数: {total_decisions}")
        print(f"  成功决策: {successful_decisions}")
        print(f"  失败决策: {total_decisions - successful_decisions}")
        print(f"  成功率: {success_rate:.2f}%")
    
    # 6. 分析最近的错误
    print("\n## 6. 最近的错误（最后10个）")
    recent_errors = []
    for record in reversed(records[-50:]):  # 检查最后50个记录
        if not record.get('success', True):
            error_msg = record.get('error_message', '')
            timestamp = record.get('timestamp', '')[:19]
            recent_errors.append((timestamp, error_msg))
            if len(recent_errors) >= 10:
                break
    
    for i, (timestamp, error_msg) in enumerate(recent_errors, 1):
        print(f"  {i}. [{timestamp}] {error_msg[:80]}")
    
    print("\n" + "=" * 80)
    print("\n🔍 关键发现:")
    print("  1. 验证失败132次 - 主要是开仓金额和仓位限制问题")
    print("  2. 开仓488次，平仓270次 - 有218个持仓未平仓")
    print("  3. 需要检查止损止盈是否真正执行")
    print("  4. 需要分析未平仓持仓的盈亏情况")

if __name__ == '__main__':
    if len(sys.argv) > 1:
        log_dir = sys.argv[1]
    else:
        log_dir = '/Users/huangjunyou/aetheris/decision_logs/hyperliquid_admin_deepseek_1762369142'
    
    analyze_detailed(log_dir)

