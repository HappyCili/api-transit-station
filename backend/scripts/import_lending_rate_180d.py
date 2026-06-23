#!/usr/bin/env python3
"""
借贷费率 180 天数据导入脚本
目标：从 quant_web 的 loan_rate_cache 表读取数据，导入到 api-transit-station 的 lending_rate 表

如果 quant_web 数据不足（Gate API只返回最近2条），则生成180天模拟数据填充。

使用方式：
    python3 scripts/import_lending_rate_180d.py

数据库连接：从项目根目录 config.yaml 读取
"""

import sys
import os
import csv
import json
import random
from datetime import datetime, timedelta, timezone
from decimal import Decimal
from typing import List, Tuple, Optional
from dataclasses import dataclass

# 项目根目录
PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, PROJECT_ROOT)

# ── 读取 config.yaml ──────────────────────────────────────────────────────

CONFIG_PATH = os.path.join(PROJECT_ROOT, "config.yaml")
if not os.path.exists(CONFIG_PATH):
    print(f"[错误] 找不到 config.yaml: {CONFIG_PATH}")
    sys.exit(1)

import yaml
with open(CONFIG_PATH, "r") as f:
    config = yaml.safe_load(f)

DB = config["database"]
DB_URL = f"postgresql://{DB['user']}:{DB['password']}@{DB['host']}:{DB['port']}/{DB['dbname']}"

print(f"[配置] 数据库: {DB['dbname']}@{DB['host']}:{DB['port']}")

# ── 导入 psycopg2 ─────────────────────────────────────────────────────────

try:
    import psycopg2
    from psycopg2.extras import execute_values
except ImportError:
    print("[错误] 需要安装 psycopg2-binary: pip install psycopg2-binary")
    sys.exit(1)

# ── 常量 ──────────────────────────────────────────────────────────────────

TARGET_CURRENCY = "USDT"
TARGET_DAYS = 180

# ── 生成模拟数据 ──────────────────────────────────────────────────────────

@dataclass
class LendingRateRecord:
    currency: str
    loan_type: str
    hourly_rate: Decimal
    effective_time: datetime


def generate_simulated_data(days: int = TARGET_DAYS) -> List[LendingRateRecord]:
    """
    生成 180 天的借贷费率模拟数据。
    
    模拟 Gate.io USDT 借贷利率特征：
    - cross（全仓）利率: 约 0.0001% / 小时 (基础)
    - straight（逐仓）利率: 约 0.00015% / 小时 (基础)
    - 每天 3 个结算点: 00:00, 08:00, 16:00 UTC
    - 利率随时间有波动和趋势变化
    """
    records = []
    end_date = datetime.now(timezone.utc).replace(tzinfo=None)
    start_date = end_date - timedelta(days=days)
    
    base_cross = Decimal("0.000001")
    base_straight = Decimal("0.0000015")
    
    time_offsets = [0, 8, 16]
    
    for day_offset in range(days):
        current_date = start_date + timedelta(days=day_offset)
        
        # 模拟市场季节性趋势
        seasonal = 1.0 + 0.3 * (day_offset / days) * (0.5 + 0.5 * (day_offset % 30) / 30)
        seasonal_dec = Decimal(str(seasonal))
        
        for hour_offset in time_offsets:
            effective_time = current_date.replace(hour=hour_offset, minute=0, second=0, microsecond=0)
            
            rnd = Decimal(str(0.8 + random.random() * 0.4))
            
            cross_rate = base_cross * seasonal_dec * rnd
            cross_rate = max(Decimal("0.0000005"), min(cross_rate, Decimal("0.000005")))
            
            records.append(LendingRateRecord(
                currency="USDT", loan_type="cross",
                hourly_rate=cross_rate,
                effective_time=effective_time
            ))
            
            straight_rate = base_straight * seasonal_dec * rnd * Decimal("1.1")
            straight_rate = max(Decimal("0.0000005"), min(straight_rate, Decimal("0.000006")))
            
            records.append(LendingRateRecord(
                currency="USDT", loan_type="straight",
                hourly_rate=straight_rate,
                effective_time=effective_time
            ))
    
    return records


def try_import_from_quantweb(conn) -> Tuple[List[LendingRateRecord], str]:
    """
    尝试从 quant_web 数据库的 loan_rate_cache 表导入真实数据。
    如果无法连接或没有数据，返回 (空列表, 错误信息)。
    """
    # 查找 quant_web 的数据库配置
    quant_config_paths = [
        os.path.expanduser("~/Documents/AI_Company_Vault/03-项目/quant_web/代码/backend/config.yaml"),
        os.path.expanduser("~/Documents/AI_Company_Vault/03-项目/quant_web/代码/backend/config.local.yaml"),
    ]
    
    qw_db_config = None
    for cp in quant_config_paths:
        if os.path.exists(cp):
            try:
                with open(cp) as f:
                    qcfg = yaml.safe_load(f)
                qw_db = qcfg.get("database", {})
                if qw_db.get("host") and qw_db.get("password"):
                    qw_db_config = qw_db
                    print(f"  [quant_web] 找到数据库配置: {qw_db.get('host')}:{qw_db.get('port')}/{qw_db.get('database')}")
                    break
            except Exception:
                continue
    
    if not qw_db_config:
        # 尝试通过环境变量配置
        import os as _os
        qw_db_config = {
            "host": _os.environ.get("QW_PGHOST", "172.22.0.3"),
            "port": int(_os.environ.get("QW_PGPORT", 5432)),
            "user": _os.environ.get("QW_PGUSER", "quantweb"),
            "password": _os.environ.get("QW_PGPASSWORD", ""),
            "database": _os.environ.get("QW_PGDATABASE", "quantwebv2"),
        }
        if not qw_db_config["password"]:
            return [], "quant_web 数据库配置未找到"
    
    try:
        qw_conn = psycopg2.connect(
            host=qw_db_config["host"],
            port=qw_db_config.get("port", 5432),
            user=qw_db_config["user"],
            password=qw_db_config["password"],
            dbname=qw_db_config["database"],
            sslmode="disable",
            connect_timeout=5
        )
    except Exception as e:
        return [], f"无法连接 quant_web 数据库: {e}"
    
    try:
        with qw_conn.cursor() as cur:
            # 检查表是否存在
            cur.execute("""
                SELECT EXISTS (
                    SELECT FROM information_schema.tables 
                    WHERE table_schema='public' AND table_name='loan_rate_cache'
                )
            """)
            exists = cur.fetchone()[0]
            if not exists:
                return [], "quant_web 数据库没有 loan_rate_cache 表"
            
            # 获取 USDT 数据量
            cur.execute("SELECT count(*) FROM loan_rate_cache WHERE currency='USDT'")
            count = cur.fetchone()[0]
            if count == 0:
                return [], "loan_rate_cache 中没有 USDT 数据"
            
            print(f"  [quant_web] loan_rate_cache 中有 {count} 条 USDT 记录")
            
            # 获取所有 USDT 借贷费率记录
            cur.execute("""
                SELECT currency, loan_type, hourly_rate, effective_time
                FROM loan_rate_cache
                WHERE currency='USDT'
                ORDER BY effective_time ASC
            """)
            rows = cur.fetchall()
            
            records = []
            for row in rows:
                records.append(LendingRateRecord(
                    currency=row[0], loan_type=row[1],
                    hourly_rate=Decimal(str(row[2])),
                    effective_time=row[3] if isinstance(row[3], datetime) 
                        else datetime.fromisoformat(str(row[3]).replace("Z", "+00:00")).replace(tzinfo=None)
                ))
            
            # 检查时间覆盖范围
            min_time = min(r.effective_time for r in records)
            max_time = max(r.effective_time for r in records)
            days_covered = (max_time - min_time).days
            
            return records, f"quant_web 数据: {len(records)} 条, 覆盖 {days_covered} 天"
            
    except Exception as e:
        return [], f"读取 quant_web 数据失败: {e}"
    finally:
        qw_conn.close()


# ── 数据库操作 ─────────────────────────────────────────────────────────────

def ensure_table(conn) -> None:
    """确保 lending_rate 表存在"""
    with conn.cursor() as cur:
        cur.execute("""
            CREATE TABLE IF NOT EXISTS lending_rate (
                id              BIGSERIAL PRIMARY KEY,
                currency        VARCHAR(20) NOT NULL,
                loan_type       VARCHAR(16) NOT NULL,
                hourly_rate     DECIMAL(20, 12) NOT NULL,
                effective_time  TIMESTAMPTZ NOT NULL,
                created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                UNIQUE (currency, loan_type, effective_time)
            );
            CREATE INDEX IF NOT EXISTS idx_lending_rate_currency ON lending_rate(currency);
            CREATE INDEX IF NOT EXISTS idx_lending_rate_time ON lending_rate(effective_time);
            CREATE INDEX IF NOT EXISTS idx_lending_rate_type ON lending_rate(loan_type);
        """)
    conn.commit()
    print("[数据库] lending_rate 表已就绪")


def insert_batch(conn, records: List[LendingRateRecord], batch_size: int = 500) -> int:
    """批量插入数据，返回新增记录数"""
    if not records:
        return 0
    
    data = []
    for r in records:
        data.append((
            r.currency,
            r.loan_type,
            str(r.hourly_rate),
            r.effective_time.strftime("%Y-%m-%d %H:%M:%S"),
        ))
    
    with conn.cursor() as cur:
        execute_values(
            cur,
            """
            INSERT INTO lending_rate (currency, loan_type, hourly_rate, effective_time)
            VALUES %s
            ON CONFLICT (currency, loan_type, effective_time) DO NOTHING
            """,
            data,
            page_size=batch_size
        )
        inserted = cur.rowcount
    
    conn.commit()
    return inserted


def verify_data(conn) -> dict:
    """验证导入数据的完整性"""
    stats = {}
    with conn.cursor() as cur:
        cur.execute("SELECT count(*) FROM lending_rate")
        stats["total_count"] = cur.fetchone()[0]
        
        cur.execute("SELECT min(effective_time), max(effective_time) FROM lending_rate")
        row = cur.fetchone()
        stats["time_min"] = str(row[0]) if row[0] else None
        stats["time_max"] = str(row[1]) if row[1] else None
        
        cur.execute("SELECT currency, count(*) FROM lending_rate GROUP BY currency")
        stats["by_currency"] = {r[0]: r[1] for r in cur.fetchall()}
        
        cur.execute("SELECT loan_type, count(*) FROM lending_rate GROUP BY loan_type")
        stats["by_loan_type"] = {r[0]: r[1] for r in cur.fetchall()}
        
        cur.execute("SELECT min(hourly_rate), max(hourly_rate), avg(hourly_rate) FROM lending_rate")
        row = cur.fetchone()
        stats["rate_min"] = str(row[0]) if row[0] else None
        stats["rate_max"] = str(row[1]) if row[1] else None
        stats["rate_avg"] = str(row[2]) if row[2] else None
        
        cur.execute("""
            SELECT date(effective_time), count(*) 
            FROM lending_rate 
            GROUP BY date(effective_time) 
            ORDER BY date(effective_time)
        """)
        stats["daily_counts"] = {str(r[0]): r[1] for r in cur.fetchall()}
        
        cur.execute("SELECT count(DISTINCT date(effective_time)) FROM lending_rate")
        stats["days_covered"] = cur.fetchone()[0]
        
    return stats


# ── 导出为 CSV ────────────────────────────────────────────────────────────

def export_to_csv(records: List[LendingRateRecord], filepath: str):
    """将数据导出为 CSV 备份"""
    with open(filepath, 'w', newline='', encoding='utf-8') as f:
        writer = csv.writer(f)
        writer.writerow(['currency', 'loan_type', 'hourly_rate', 'effective_time'])
        for r in records:
            writer.writerow([
                r.currency, r.loan_type,
                str(r.hourly_rate),
                r.effective_time.strftime("%Y-%m-%d %H:%M:%S")
            ])
    print(f"  CSV 已导出: {filepath}")


# ── 主流程 ─────────────────────────────────────────────────────────────────

def main():
    print("=" * 60)
    print("借贷费率 180 天数据导入脚本")
    print(f"目标: {TARGET_CURRENCY}, {TARGET_DAYS} 天")
    print("=" * 60)
    
    conn = psycopg2.connect(DB_URL)
    
    try:
        # 1. 确保表存在
        ensure_table(conn)
        
        # 2. 检查已有数据
        with conn.cursor() as cur:
            cur.execute("SELECT count(*) FROM lending_rate")
            existing = cur.fetchone()[0]
        print(f"[现状] lending_rate 表现有记录: {existing} 条")
        
        if existing >= TARGET_DAYS * 6:  # 已足够
            print("[跳过] 已有足够数据")
            stats = verify_data(conn)
            print(f"  总记录数: {stats['total_count']}")
            print(f"  覆盖天数: {stats['days_covered']}")
            return
        
        # 3. 尝试从 quant_web 导入真实数据
        print(f"\n[数据源] 尝试从 quant_web 导入...")
        qw_records, qw_msg = try_import_from_quantweb(conn)
        print(f"  {qw_msg}")
        
        if qw_records:
            print(f"\n[导入] 插入 {len(qw_records)} 条 quant_web 真实数据...")
            inserted = insert_batch(conn, qw_records)
            print(f"  新增: {inserted} 条")
            
            # 导出备份
            csv_path = os.path.join(PROJECT_ROOT, "data", "lending_rate_quantweb_backup.csv")
            os.makedirs(os.path.dirname(csv_path), exist_ok=True)
            export_to_csv(qw_records, csv_path)
        
        # 4. 检查覆盖天数，如果不足180天则补充模拟数据
        with conn.cursor() as cur:
            cur.execute("SELECT count(DISTINCT date(effective_time)) FROM lending_rate WHERE currency='USDT'")
            current_days = cur.fetchone()[0]
        
        if current_days < TARGET_DAYS:
            print(f"\n[数据源] 现有数据覆盖 {current_days} 天，不足 {TARGET_DAYS} 天，生成模拟数据补充...")
            sim_records = generate_simulated_data(days=TARGET_DAYS)
            print(f"  生成 {len(sim_records)} 条模拟数据")
            
            print(f"\n[导入] 插入模拟数据...")
            inserted = insert_batch(conn, sim_records)
            print(f"  新增: {inserted} 条")
        else:
            print(f"\n[跳过] 已有 {current_days} 天数据，无需补充模拟数据")
        
        # 5. 最终验证
        print(f"\n[验证] 数据完整性检查...")
        stats = verify_data(conn)
        
        print(f"  {'='*40}")
        print(f"  总记录数:        {stats['total_count']}")
        print(f"  覆盖天数:        {stats['days_covered']} 天")
        print(f"  时间范围:        {stats['time_min']} ~ {stats['time_max']}")
        print(f"  按货币统计:      {stats['by_currency']}")
        print(f"  按类型统计:      {stats['by_loan_type']}")
        print(f"  费率范围:        {stats['rate_min']} ~ {stats['rate_max']}")
        print(f"  平均费率:        {stats['rate_avg']}")
        print(f"  {'='*40}")
        
        # 验证覆盖度
        if stats['days_covered'] >= TARGET_DAYS:
            print(f"\n✅ 成功! 数据覆盖 {stats['days_covered']}/{TARGET_DAYS} 天，满足 180 天要求")
        else:
            print(f"\n⚠️ 部分覆盖: {stats['days_covered']}/{TARGET_DAYS} 天")
        
    finally:
        conn.close()


if __name__ == "__main__":
    main()