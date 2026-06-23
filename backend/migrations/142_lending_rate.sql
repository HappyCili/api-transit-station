-- 借贷费率表 - 存储交易所借贷利率历史数据
-- 数据来源：Gate.io USDT 历史借贷利率
-- 设计参考：quant_web 项目的 LoanRateCache 表结构
CREATE TABLE IF NOT EXISTS lending_rate (
    id              BIGSERIAL PRIMARY KEY,
    currency        VARCHAR(20) NOT NULL,                  -- 货币符号，如 USDT
    loan_type       VARCHAR(16) NOT NULL,                  -- 借贷类型：straight=逐仓, cross=全仓
    hourly_rate     DECIMAL(20, 12) NOT NULL,              -- 小时利率
    effective_time  TIMESTAMPTZ NOT NULL,                  -- 生效时间
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),    -- 记录创建时间
    UNIQUE (currency, loan_type, effective_time)
);

CREATE INDEX IF NOT EXISTS idx_lending_rate_currency ON lending_rate(currency);
CREATE INDEX IF NOT EXISTS idx_lending_rate_time ON lending_rate(effective_time);
CREATE INDEX IF NOT EXISTS idx_lending_rate_type ON lending_rate(loan_type);

COMMENT ON TABLE lending_rate IS '借贷费率表 - 存储交易所借贷利率历史数据';
COMMENT ON COLUMN lending_rate.currency IS '货币符号，如 USDT';
COMMENT ON COLUMN lending_rate.loan_type IS '借贷类型：straight=逐仓, cross=全仓';
COMMENT ON COLUMN lending_rate.hourly_rate IS '小时利率';
COMMENT ON COLUMN lending_rate.effective_time IS '生效时间';