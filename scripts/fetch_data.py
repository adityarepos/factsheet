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
    
    # 2. Extract NEW deeper financial metrics (Use .get to safely return None if missing)
    pe_ratio = info.get('trailingPE')
    beta = info.get('beta')
    div_yield = info.get('dividendYield')
    high_52 = info.get('fiftyTwoWeekHigh')

    mcap = info.get('marketCap', 0)
    if mcap > 10_000_000_000:
        mcap_cat = 'Large Cap'
    elif mcap > 2_000_000_000:
        mcap_cat = 'Mid Cap'
    else:
        mcap_cat = 'Small Cap'

    with conn.cursor() as cur:
        # Upsert into main asset table with new metrics
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
        
        # Extract Sector & Regional Exposures (unchanged)
        exposures = []
        if 'sector' in info:
            exposures.append((ticker, 'sector', info['sector'], 1.0))
        if 'country' in info:
            exposures.append((ticker, 'region', info['country'], 1.0))

        if exposures:
            from psycopg2.extras import execute_values
            execute_values(cur, """
                INSERT INTO asset_exposures (asset_ticker, exposure_type, exposure_name, weight_percentage)
                VALUES %s
                ON CONFLICT (asset_ticker, exposure_type, exposure_name)
                DO UPDATE SET weight_percentage = EXCLUDED.weight_percentage;
            """, exposures)

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