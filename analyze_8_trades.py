#!/usr/bin/env python3
"""
详细分析8笔交易的表现
"""
import json
import sys
from pathlib import Path
from collections import defaultdict
from datetime import datetime

def analyze_8_trades(log_dir):
    """详细分析交易表现"""
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
            continue
    
    if not records:
        print("❌ 没有找到任何交易记录")
        return
    
    print("=" * 80)
    print("📊 8笔交易详细分析报告")
    print("=" * 80)
    
    # 追踪开仓和平仓
    open_positions = {}  # symbol_side -> {open_record, open_decision}
    completed_trades = []
    
    for record in records:
        decisions = record.get('decisions', [])
        if not decisions:
            continue
            
        for dec in decisions:
            if not dec.get('success', False):
                continue
                
            action = dec.get('action', '')
            symbol = dec.get('symbol', '')
            
            if action in ['open_long', 'open_short']:
                side = action.split('_')[1]
                key = f'{symbol}_{side}'
                open_positions[key] = {
                    'open_record': record,
                    'open_decision': dec,
                    'open_time': record.get('timestamp', ''),
                }
                
            elif action in ['close_long', 'close_short', 'partial_close']:
                side = action.split('_')[1] if action != 'partial_close' else 'unknown'
                # 对于partial_close，需要从持仓中找方向
                if action == 'partial_close':
                    for pos_key in list(open_positions.keys()):
                        if pos_key.startswith(symbol + '_'):
                            side = pos_key.split('_')[1]
                            break
                
                key = f'{symbol}_{side}'
                if key in open_positions:
                    open_info = open_positions[key]
                    completed_trades.append({
                        'symbol': symbol,
                        'side': side,
                        'open_record': open_info['open_record'],
                        'open_decision': open_info['open_decision'],
                        'close_record': record,
                        'close_decision': dec,
                        'open_time': open_info['open_time'],
                        'close_time': record.get('timestamp', ''),
                    })
                    
                    if action in ['close_long', 'close_short']:
                        del open_positions[key]
    
    print(f"\n找到 {len(completed_trades)} 笔完整交易\n")
    
    # 统计
    winning = 0
    losing = 0
    total_pnl_pct = 0
    total_pnl_usdt = 0
    
    # 详细分析每笔交易
    for i, trade in enumerate(completed_trades, 1):
        open_dec = trade['open_decision']
        close_dec = trade['close_decision']
        
        open_price = open_dec.get('price', 0)
        close_price = close_dec.get('price', 0)
        leverage = open_dec.get('leverage', 1)
        
        # 计算盈亏
        if trade['side'] == 'long':
            pnl_pct = ((close_price - open_price) / open_price) * 100 * leverage
        else:
            pnl_pct = ((open_price - close_price) / open_price) * 100 * leverage
        
        # 计算USDT盈亏（需要知道仓位大小）
        position_size = open_dec.get('quantity', 0) * open_price if open_price > 0 else 0
        margin_used = position_size / leverage if leverage > 0 else 0
        pnl_usdt = margin_used * (pnl_pct / 100) if margin_used > 0 else 0
        
        result = '✅' if pnl_pct > 0 else '❌'
        if pnl_pct > 0:
            winning += 1
        else:
            losing += 1
        total_pnl_pct += pnl_pct
        total_pnl_usdt += pnl_usdt
        
        # 获取开仓原因
        reasoning = open_dec.get('reasoning', '无')
        confidence = open_dec.get('confidence', 0)
        stop_loss = open_dec.get('stop_loss', 0)
        take_profit = open_dec.get('take_profit', 0)
        
        # 获取开仓时的市场状态
        open_record = trade['open_record']
        account_state = open_record.get('account_state', {})
        positions = open_record.get('positions') or []
        
        # 计算持仓时长
        try:
            open_dt = datetime.fromisoformat(trade['open_time'].replace('Z', '+00:00'))
            close_dt = datetime.fromisoformat(trade['close_time'].replace('Z', '+00:00'))
            duration = close_dt - open_dt
            duration_str = f"{duration.total_seconds() / 60:.1f} 分钟"
        except:
            duration_str = "未知"
        
        print(f"{result} 交易 #{i}: {trade['symbol']} {trade['side'].upper()}")
        print(f"  时间: {trade['open_time'][:19]} → {trade['close_time'][:19]} ({duration_str})")
        print(f"  价格: {open_price:.4f} → {close_price:.4f}")
        print(f"  杠杆: {leverage}x | 盈亏: {pnl_pct:+.2f}% ({pnl_usdt:+.4f} USDT)")
        print(f"  止损: {stop_loss:.4f} | 止盈: {take_profit:.4f}")
        print(f"  信心度: {confidence}")
        print(f"  开仓原因: {reasoning[:100] if reasoning else '无'}...")
        print(f"  开仓时账户: 净值={account_state.get('total_balance', 0):.2f}, 持仓数={len(positions)}")
        print()
    
    # 总体统计
    print("=" * 80)
    print("📈 总体表现统计")
    print("=" * 80)
    print(f"总交易数: {len(completed_trades)} 笔")
    print(f"盈利: {winning} 笔 ({winning/len(completed_trades)*100:.1f}%)")
    print(f"亏损: {losing} 笔 ({losing/len(completed_trades)*100:.1f}%)")
    if len(completed_trades) > 0:
        print(f"平均盈亏: {total_pnl_pct/len(completed_trades):+.2f}%")
        print(f"总盈亏: {total_pnl_usdt:+.4f} USDT")
    
    # 分析问题
    print("\n" + "=" * 80)
    print("🔍 问题分析")
    print("=" * 80)
    
    # 检查止损止盈
    no_sl_tp = sum(1 for t in completed_trades 
                   if t['open_decision'].get('stop_loss', 0) == 0 or 
                      t['open_decision'].get('take_profit', 0) == 0)
    if no_sl_tp > 0:
        print(f"⚠️  {no_sl_tp} 笔交易没有设置止损/止盈（可能是早期版本问题）")
    
    # 检查持仓时长
    short_trades = []
    for t in completed_trades:
        try:
            open_dt = datetime.fromisoformat(t['open_time'].replace('Z', '+00:00'))
            close_dt = datetime.fromisoformat(t['close_time'].replace('Z', '+00:00'))
            duration_min = (close_dt - open_dt).total_seconds() / 60
            if duration_min < 30:
                short_trades.append((t['symbol'], duration_min))
        except:
            pass
    
    if short_trades:
        print(f"⚠️  {len(short_trades)} 笔交易持仓时间很短（<30分钟），可能是过早平仓")
        for symbol, duration in short_trades:
            print(f"     - {symbol}: {duration:.1f} 分钟")
    
    # 分析亏损交易
    losing_trades = [t for t in completed_trades 
                    if ((t['close_decision'].get('price', 0) - t['open_decision'].get('price', 0)) * 
                        (1 if t['side'] == 'long' else -1)) < 0]
    
    if losing_trades:
        print(f"\n❌ 亏损交易分析 ({len(losing_trades)} 笔):")
        for t in losing_trades:
            pnl = ((t['close_decision'].get('price', 0) - t['open_decision'].get('price', 0)) / 
                   t['open_decision'].get('price', 0) * 100 * t['open_decision'].get('leverage', 1))
            print(f"   - {t['symbol']} {t['side']}: {pnl:.2f}% | 原因: {t['open_decision'].get('reasoning', '无')[:60]}")
    
    print("\n" + "=" * 80)
    print("💡 建议")
    print("=" * 80)
    
    win_rate = winning / len(completed_trades) * 100 if len(completed_trades) > 0 else 0
    
    if win_rate < 50:
        print("⚠️  胜率较低，建议:")
        print("   1. 提高开仓标准（当前可能过于宽松）")
        print("   2. 加强止损执行（确保止损真正生效）")
        print("   3. 分析亏损交易，找出共同模式")
    elif win_rate >= 50:
        print("✅ 胜率尚可，但交易次数太少，建议:")
        print("   1. 可以适当降低开仓门槛，增加交易频率")
        print("   2. 保持当前质量，但增加资金规模")
        print("   3. 分析盈利交易，找出成功模式并复制")
    
    if no_sl_tp > 0:
        print("⚠️  止损止盈问题:")
        print("   1. 确保所有开仓都设置了止损止盈")
        print("   2. 验证止损止盈是否真正执行")
    
    if total_pnl_usdt < 0:
        print("⚠️  总体亏损，建议:")
        print("   1. 暂停交易，重新评估策略")
        print("   2. 降低杠杆倍数")
        print("   3. 缩小仓位大小")

if __name__ == '__main__':
    if len(sys.argv) > 1:
        log_dir = sys.argv[1]
    else:
        log_dir = 'decision_logs/aster_admin_deepseek_1762714086'
    
    analyze_8_trades(log_dir)

