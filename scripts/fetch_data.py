import os
import yfinance as yf
import psycopg2
from psycopg2.extras import execute_values

# Replace with your actual Neon connection string for testing, 
# or set it as an environment variable (export DATABASE_URL="...")
DB_CONN = os.getenv("DATABASE_URL", "YOUR_NEON_CONNECTION_STRING_HERE")

def get_portfolio_tickers(conn):
    """Fetches unique tickers currently defined in your portfolios."""
    with conn.cursor() as cur:
        cur.execute("SELECT DISTINCT asset_ticker FROM portfolio_holdings;")
        return [row[0] for row in cur.fetchall()]

def update_asset_data(conn, ticker):
    """Fetches data from Yahoo Finance and upserts into PostgreSQL."""
    asset = yf.Ticker(ticker)
    info = asset.info
    
    # 1. Extract standard metrics
    name = info.get('longName', info.get('shortName', ticker))
    price = info.get('currentPrice') or info.get('navPrice') or info.get('previousClose')
    
    # 2. Extract deeper financial metrics (Use .get to safely return None if missing)
    pe_ratio = info.get('trailingPE')
    beta = info.get('beta')
    div_yield = info.get('dividendYield')
    high_52 = info.get('fiftyTwoWeekHigh')

    # Detect if it's an ETF or determine by Market Cap
    quote_type = info.get('quoteType', '').upper()
    is_etf = quote_type == 'ETF' or ticker in ['VOO', 'IEFA', 'VWO']

    if is_etf:
        mcap_cat = 'ETF'
    else:
        mcap = info.get('marketCap', 0)
        if mcap > 10_000_000_000:
            mcap_cat = 'Large Cap'
        elif mcap > 2_000_000_000:
            mcap_cat = 'Mid Cap'
        else:
            mcap_cat = 'Small Cap'

    with conn.cursor() as cur:
        # Upsert into main asset table with metrics
        cur.execute("""
            INSERT INTO assets (ticker, name, current_price, pe_ratio, beta, dividend_yield, fifty_two_week_high, market_cap_category, last_updated)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, CURRENT_TIMESTAMP)
            ON CONFLICT (ticker) 
            DO UPDATE SET 
                current_price = EXCLUDED.current_price,
                pe_ratio = EXCLUDED.pe_ratio,
                beta = EXCLUDED.beta,
                dividend_yield = EXCLUDED.dividend_yield,
                fifty_two_week_high = EXCLUDED.fifty_two_week_high,
                market_cap_category = EXCLUDED.market_cap_category,
                last_updated = CURRENT_TIMESTAMP;
        """, (ticker, name, price, pe_ratio, beta, div_yield, high_52, mcap_cat))
        
        # Extract Sector & Regional Exposures
        exposures = []
        if 'sector' in info:
            exposures.append((ticker, 'sector', info['sector'], 1.0))
        if 'country' in info:
            exposures.append((ticker, 'region', info['country'], 1.0))

        if exposures:
            execute_values(cur, """
                INSERT INTO asset_exposures (asset_ticker, exposure_type, exposure_name, weight_percentage)
                VALUES %s
                ON CONFLICT (asset_ticker, exposure_type, exposure_name)
                DO UPDATE SET weight_percentage = EXCLUDED.weight_percentage;
            """, exposures)

        # 3. ADVANCED: Look-Through Dataset Extraction for ETFs/Funds
        if is_etf:
            try:
                # yfinance returns top holdings as a pandas DataFrame via .funds_data.top_holdings
                funds = asset.funds_data
                if funds is not None and hasattr(funds, 'top_holdings') and funds.top_holdings is not None:
                    df_holdings = funds.top_holdings
                    
                    underlying_records = []
                    for _, row in df_holdings.iterrows():
                        h_ticker = row.get('Symbol')
                        h_name = row.get('Name')
                        h_weight = row.get('Holding Percent')
                        
                        if h_ticker and h_weight is not None:
                            # Safely convert percentage format (e.g. 0.065 or 6.5) to decimal fraction
                            if h_weight > 1.0:
                                h_weight = h_weight / 100.0
                                
                            underlying_records.append((ticker, h_ticker, h_name, h_weight))
                    
                    if underlying_records:
                        execute_values(cur, """
                            INSERT INTO etf_underlying_holdings (etf_ticker, holding_ticker, holding_name, holding_weight)
                            VALUES %s
                            ON CONFLICT (etf_ticker, holding_ticker)
                            DO UPDATE SET holding_weight = EXCLUDED.holding_weight;
                        """, underlying_records)
                        print(f" -> Extracted {len(underlying_records)} look-through components inside {ticker}.")
            except Exception as etf_err:
                print(f" -> Skip underlying check for {ticker}: {etf_err}")

def main():
    print("Connecting to database...")
    conn = psycopg2.connect(DB_CONN)
    try:
        tickers = get_portfolio_tickers(conn)
        print(f"Found {len(tickers)} tickers to update.")
        
        for ticker in tickers:
            try:
                print(f"Syncing data for {ticker}...")
                update_asset_data(conn, ticker)
                print(f"Successfully synced {ticker}.")
            except Exception as e:
                print(f"Error syncing {ticker}: {e}")
                
        conn.commit()
        print("Database commit successful. Pipeline finished.")
    except Exception as e:
        print(f"Database connection error: {e}")
    finally:
        conn.close()

if __name__ == "__main__":
    main()