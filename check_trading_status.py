#!/usr/bin/env python3
"""
检查交易状态诊断工具
用于诊断为什么交易员长时间没有进行交易
"""

import json
import os
import sys
from datetime import datetime, timedelta
from pathlib import Path
from typing import List, Dict, Optional

def find_latest_decision_logs(log_dir: str, days: int = 3) -> List[Dict]:
    """查找最近N天的决策日志"""
    if not os.path.exists(log_dir):
        print(f"❌ 日志目录不存在: {log_dir}")
        return []
    
    logs = []
    cutoff_time = datetime.now() - timedelta(days=days)
    
    for filename in sorted(os.listdir(log_dir), reverse=True):
        if not filename.endswith('.json'):
            continue
        
        filepath = os.path.join(log_dir, filename)
        try:
            with open(filepath, 'r', encoding='utf-8') as f:
                data = json.load(f)
            
            # 解析时间戳
            timestamp_str = data.get('timestamp', '')
            if timestamp_str:
                try:
                    timestamp = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                    if timestamp.replace(tzinfo=None) >= cutoff_time:
                        logs.append(data)
                except:
                    pass
        except Exception as e:
            print(f"⚠️ 读取日志文件失败 {filename}: {e}")
            continue
    
    return sorted(logs, key=lambda x: x.get('timestamp', ''), reverse=True)

def analyze_trading_status(logs: List[Dict]) -> Dict:
    """分析交易状态"""
    if not logs:
        return {
            'status': 'no_logs',
            'message': '没有找到最近的决策日志'
        }
    
    latest_log = logs[0]
    total_logs = len(logs)
    
    # 统计决策类型
    action_counts = {}
    wait_count = 0
    open_count = 0
    hold_count = 0
    error_count = 0
    
    for log in logs:
        decisions = log.get('decisions', [])
        if not decisions or decisions is None:
            wait_count += 1
            continue
        
        for decision in decisions:
            action = decision.get('action', 'wait')
            action_counts[action] = action_counts.get(action, 0) + 1
            
            if action == 'wait':
                wait_count += 1
            elif action in ['open_long', 'open_short']:
                open_count += 1
            elif action == 'hold':
                hold_count += 1
        
        # 检查是否有错误
        if not log.get('success', True):
            error_count += 1
    
    # 检查最新日志中的暂停信息
    error_message = latest_log.get('error_message', '')
    is_paused = '暂停交易' in error_message or '风险控制暂停' in error_message
    
    # 检查最后交易时间
    last_trade_time = None
    for log in logs:
        decisions = log.get('decisions', [])
        if not decisions or decisions is None:
            continue
        for decision in decisions:
            if decision.get('action') in ['open_long', 'open_short']:
                timestamp_str = log.get('timestamp', '')
                if timestamp_str:
                    try:
                        timestamp = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                        if not last_trade_time or timestamp > last_trade_time:
                            last_trade_time = timestamp.replace(tzinfo=None)
                    except:
                        pass
    
    # 分析AI思维链（从最新日志）
    cot_trace = latest_log.get('cot_trace', '')
    reasoning_patterns = []
    
    if 'wait' in cot_trace.lower() or '观望' in cot_trace:
        reasoning_patterns.append('AI选择观望')
    if '不确定' in cot_trace or '不确定' in cot_trace:
        reasoning_patterns.append('AI表示不确定')
    if '信心度' in cot_trace and '不足' in cot_trace:
        reasoning_patterns.append('信心度不足')
    if '冷却' in cot_trace or 'cooldown' in cot_trace.lower():
        reasoning_patterns.append('冷却期')
    if '暂停' in cot_trace or '暂停交易' in cot_trace:
        reasoning_patterns.append('暂停交易')
    
    result = {
        'status': 'analyzed',
        'total_logs': total_logs,
        'latest_log_time': latest_log.get('timestamp', ''),
        'is_paused': is_paused,
        'pause_reason': error_message if is_paused else None,
        'action_counts': action_counts,
        'wait_count': wait_count,
        'open_count': open_count,
        'hold_count': hold_count,
        'error_count': error_count,
        'last_trade_time': last_trade_time.isoformat() if last_trade_time else None,
        'reasoning_patterns': reasoning_patterns,
        'latest_cot_trace': cot_trace[:500] if cot_trace else None,  # 只显示前500字符
    }
    
    return result

def print_diagnosis(result: Dict, trader_id: str):
    """打印诊断结果"""
    print(f"\n{'='*70}")
    print(f"📊 交易状态诊断报告 - {trader_id}")
    print(f"{'='*70}\n")
    
    if result['status'] == 'no_logs':
        print(f"❌ {result['message']}")
        print("\n可能的原因：")
        print("  1. 交易员未启动")
        print("  2. 日志目录路径错误")
        print("  3. 系统最近没有运行")
        return
    
    print(f"📅 最近日志时间: {result['latest_log_time']}")
    print(f"📝 总日志数量: {result['total_logs']} 条\n")
    
    # 暂停状态
    if result['is_paused']:
        print(f"⏸️  ⚠️  交易已暂停！")
        print(f"   原因: {result['pause_reason']}")
        print(f"\n💡 解决方案：")
        print(f"   1. 检查风险控制配置（最大回撤、日亏损限制）")
        print(f"   2. 检查是否触发连续亏损暂停机制")
        print(f"   3. 等待暂停时间到期或手动重置")
        print()
    
    # 最后交易时间
    if result['last_trade_time']:
        last_trade = datetime.fromisoformat(result['last_trade_time'])
        days_since_last_trade = (datetime.now() - last_trade).days
        hours_since_last_trade = (datetime.now() - last_trade).total_seconds() / 3600
        
        print(f"🕐 最后交易时间: {last_trade.strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"   距离现在: {days_since_last_trade} 天 ({hours_since_last_trade:.1f} 小时)\n")
    else:
        print(f"❌ 最近3天内没有找到任何交易记录\n")
    
    # 决策统计
    print(f"📊 决策统计:")
    print(f"   Wait (观望): {result['wait_count']} 次")
    print(f"   Open (开仓): {result['open_count']} 次")
    print(f"   Hold (持有): {result['hold_count']} 次")
    print(f"   错误: {result['error_count']} 次")
    
    if result['action_counts']:
        print(f"\n   详细统计:")
        for action, count in sorted(result['action_counts'].items(), key=lambda x: x[1], reverse=True):
            print(f"     {action}: {count} 次")
    print()
    
    # 推理模式
    if result['reasoning_patterns']:
        print(f"🤔 AI推理模式:")
        for pattern in result['reasoning_patterns']:
            print(f"   - {pattern}")
        print()
    
    # 建议
    print(f"💡 可能的原因和建议:")
    
    if result['is_paused']:
        print(f"   ⚠️  主要问题：风险控制暂停")
        print(f"   → 检查账户盈亏状态")
        print(f"   → 检查是否触发连续亏损机制")
    elif result['open_count'] == 0:
        print(f"   ⚠️  主要问题：AI没有做出任何开仓决策")
        print(f"   → 检查AI提示词是否过于保守（零号原则）")
        print(f"   → 检查开仓条件是否过于严格（信心度≥85）")
        print(f"   → 检查市场数据是否正常获取")
        print(f"   → 检查账户余额是否足够（最小开仓金额）")
    elif result['wait_count'] > result['open_count'] * 10:
        print(f"   ⚠️  主要问题：AI过于保守，频繁选择wait")
        print(f"   → 考虑调整提示词，降低开仓门槛")
        print(f"   → 检查市场环境是否确实不适合交易")
    else:
        print(f"   ℹ️  系统运行正常，但可能市场条件不满足开仓要求")
    
    # 显示最新思维链片段
    if result['latest_cot_trace']:
        print(f"\n📋 最新AI思维链片段:")
        print(f"{'-'*70}")
        print(result['latest_cot_trace'])
        print(f"{'-'*70}")

def main():
    if len(sys.argv) < 2:
        print("用法: python check_trading_status.py <trader_id> [days]")
        print("示例: python check_trading_status.py aster_admin_deepseek_1762714086 3")
        sys.exit(1)
    
    trader_id = sys.argv[1]
    days = int(sys.argv[2]) if len(sys.argv) > 2 else 3
    
    log_dir = f"decision_logs/{trader_id}"
    
    print(f"🔍 正在分析交易员 {trader_id} 最近 {days} 天的交易状态...")
    print(f"📁 日志目录: {log_dir}\n")
    
    logs = find_latest_decision_logs(log_dir, days)
    result = analyze_trading_status(logs)
    print_diagnosis(result, trader_id)

if __name__ == '__main__':
    main()

