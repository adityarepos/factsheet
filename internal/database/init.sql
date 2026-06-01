-- 1. Drop existing tables to start fresh
DROP TABLE IF EXISTS asset_exposures, portfolio_holdings, assets, portfolios CASCADE;

-- 2. Create the upgraded Portfolios Table
CREATE TABLE portfolios (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. Create the UPGRADED Master Asset Table (Now with deep metrics)
CREATE TABLE assets (
    ticker VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    current_price NUMERIC(15, 4),
    pe_ratio NUMERIC(10, 4),             -- NEW
    beta NUMERIC(10, 4),                 -- NEW
    dividend_yield NUMERIC(10, 4),       -- NEW
    fifty_two_week_high NUMERIC(15, 4),  -- NEW
    market_cap_category VARCHAR(50), 
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 4. Create Portfolio Holdings & Exposures
CREATE TABLE portfolio_holdings (
    portfolio_id INT REFERENCES portfolios(id) ON DELETE CASCADE,
    asset_ticker VARCHAR(50) REFERENCES assets(ticker),
    weight NUMERIC(5, 4) NOT NULL,
    PRIMARY KEY (portfolio_id, asset_ticker)
);

CREATE TABLE asset_exposures (
    id SERIAL PRIMARY KEY,
    asset_ticker VARCHAR(50) REFERENCES assets(ticker) ON DELETE CASCADE,
    exposure_type VARCHAR(50) NOT NULL,         
    exposure_name VARCHAR(100) NOT NULL,        
    weight_percentage NUMERIC(5, 4) NOT NULL,   
    CONSTRAINT unique_asset_exposure UNIQUE (asset_ticker, exposure_type, exposure_name)
);

-- 5. Seed the Prisma Global Growth Portfolio
INSERT INTO portfolios (id, name, description) 
VALUES (1, 'Global Growth Prisma', 'End-to-end global investing diversified portfolio for resident Indians.');

-- 6. Insert 15 Diverse Global Assets (Prices/Metrics will be NULL until Python runs)
INSERT INTO assets (ticker, name) VALUES 
('AAPL', 'Apple Inc.'), ('MSFT', 'Microsoft Corp'), ('NVDA', 'NVIDIA Corp'), 
('TSLA', 'Tesla Inc.'), ('AMZN', 'Amazon.com Inc.'), ('GOOGL', 'Alphabet Inc.'),
('VOO', 'Vanguard S&P 500 ETF'), ('IEFA', 'iShares Core MSCI EAFE ETF'), ('VWO', 'Vanguard Emerging Markets ETF'),
('RELIANCE.NS', 'Reliance Industries'), ('TCS.NS', 'Tata Consultancy Services'), ('HDFCBANK.NS', 'HDFC Bank'),
('JPM', 'JPMorgan Chase & Co.'), ('JNJ', 'Johnson & Johnson'), ('XOM', 'Exxon Mobil Corp');

-- 7. Allocate Portfolio Weights (Must sum exactly to 1.0000 / 100%)
INSERT INTO portfolio_holdings (portfolio_id, asset_ticker, weight) VALUES
(1, 'VOO', 0.2500),         (1, 'IEFA', 0.1500),        (1, 'VWO', 0.0500),
(1, 'AAPL', 0.0700),        (1, 'MSFT', 0.0700),        (1, 'NVDA', 0.0600),
(1, 'AMZN', 0.0500),        (1, 'GOOGL', 0.0500),       (1, 'TSLA', 0.0300),
(1, 'RELIANCE.NS', 0.0500), (1, 'TCS.NS', 0.0400),      (1, 'HDFCBANK.NS', 0.0400),
(1, 'JPM', 0.0300),         (1, 'JNJ', 0.0300),         (1, 'XOM', 0.0300);