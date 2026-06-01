-- internal/database/init.sql

-- 1. Create Core Portfolios Table [cite: 13]
CREATE TABLE IF NOT EXISTS portfolios (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Create Master Asset Table [cite: 13]
CREATE TABLE IF NOT EXISTS assets (
    ticker VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    current_price NUMERIC(15, 4),
    market_cap_category VARCHAR(50), 
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. Create Portfolio Holdings Map [cite: 13, 25]
CREATE TABLE IF NOT EXISTS portfolio_holdings (
    portfolio_id INT REFERENCES portfolios(id) ON DELETE CASCADE,
    asset_ticker VARCHAR(50) REFERENCES assets(ticker),
    weight NUMERIC(5, 4) NOT NULL, -- e.g., 0.1550 for 15.5%
    PRIMARY KEY (portfolio_id, asset_ticker)
);

-- 4. Create Dynamic Exposures Table [cite: 13, 22, 23, 24]
CREATE TABLE IF NOT EXISTS asset_exposures (
    id SERIAL PRIMARY KEY,
    asset_ticker VARCHAR(50) REFERENCES assets(ticker) ON DELETE CASCADE,
    exposure_type VARCHAR(50) NOT NULL,         -- 'sector', 'region', 'market_cap'
    exposure_name VARCHAR(100) NOT NULL,        -- e.g., 'Technology', 'United States'
    weight_percentage NUMERIC(5, 4) NOT NULL,   -- Weight within THAT specific asset
    CONSTRAINT unique_asset_exposure UNIQUE (asset_ticker, exposure_type, exposure_name)
);

---
--- SEED DATA FOR TESTING (Prisma Global Growth Example) [cite: 10]
---
INSERT INTO portfolios (id, name, description) 
VALUES (1, 'Global Growth Prisma', 'End-to-end global investing diversified portfolio.')
ON CONFLICT DO NOTHING;

-- Let's seed some major tickers into our portfolio allocation [cite: 19]
-- (We only need to set the tracking ticker and user weight; Python updates prices/exposures)
INSERT INTO assets (ticker, name) VALUES 
('AAPL', 'Apple Inc.'),
('MSFT', 'Microsoft Corp'),
('VOO', 'Vanguard S&P 500 ETF'),
('IEFA', 'iShares Core MSCI EAFE ETF')
ON CONFLICT DO NOTHING;

-- Map the holdings to our Prisma Portfolio [cite: 25]
INSERT INTO portfolio_holdings (portfolio_id, asset_ticker, weight) VALUES
(1, 'AAPL', 0.1500), -- 15%
(1, 'MSFT', 0.1500), -- 15%
(1, 'VOO',  0.4000), -- 40%
(1, 'IEFA', 0.3000)  -- 30%
ON CONFLICT DO NOTHING;