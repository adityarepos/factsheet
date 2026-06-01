package repository

import (
	"context"
	"database/sql"
	"factsheet/internal/models"
)

type FactsheetRepo struct {
	DB *sql.DB
}

// GetFactsheet aggregates portfolio data, high-level holdings, true underlying holdings, and exposures in real-time
func (r *FactsheetRepo) GetFactsheet(ctx context.Context, portfolioID int) (*models.Factsheet, error) {
	factsheet := &models.Factsheet{PortfolioID: portfolioID}

	// 1. Get Portfolio Metadata
	err := r.DB.QueryRowContext(ctx, "SELECT name, description FROM portfolios WHERE id = $1", portfolioID).
		Scan(&factsheet.Name, &factsheet.Description)
	if err != nil {
		return nil, err
	}

	// 2. Get Top-Level Holdings Data
	holdingsQuery := `
		SELECT a.ticker, a.name, ph.weight, a.current_price, 
		       a.pe_ratio, a.beta, a.dividend_yield, a.fifty_two_week_high
		FROM portfolio_holdings ph
		JOIN assets a ON ph.asset_ticker = a.ticker
		WHERE ph.portfolio_id = $1
		ORDER BY ph.weight DESC;`
	
	rows, err := r.DB.QueryContext(ctx, holdingsQuery, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var h models.Holding
		if err := rows.Scan(
			&h.Ticker, &h.Name, &h.Weight, &h.CurrentPrice, 
			&h.PERatio, &h.Beta, &h.DividendYield, &h.FiftyTwoWeekHigh,
		); err != nil {
			return nil, err
		}
		factsheet.Holdings = append(factsheet.Holdings, h)
	}

	// 3. ADVANCED LOOK-THROUGH: Calculate Ultimate Proportional Underlying Holdings
	ultimateQuery := `
		SELECT 
			COALESCE(euh.holding_ticker, ph.asset_ticker) as ultimate_ticker,
			COALESCE(euh.holding_name, a.name) as ultimate_name,
			SUM(ph.weight * COALESCE(euh.holding_weight, 1.0)) as true_effective_weight
		FROM portfolio_holdings ph
		JOIN assets a ON ph.asset_ticker = a.ticker
		LEFT JOIN etf_underlying_holdings euh ON ph.asset_ticker = euh.etf_ticker
		WHERE ph.portfolio_id = $1
		GROUP BY ultimate_ticker, ultimate_name
		ORDER BY true_effective_weight DESC;`

	ultRows, err := r.DB.QueryContext(ctx, ultimateQuery, portfolioID)
	if err != nil {
		return nil, err
	}
	defer ultRows.Close()

	for ultRows.Next() {
		var uh models.UltimateHolding
		if err := ultRows.Scan(&uh.Ticker, &uh.Name, &uh.TrueEffectiveWeight); err != nil {
			return nil, err
		}
		factsheet.UltimateUnderlyingHoldings = append(factsheet.UltimateUnderlyingHoldings, uh)
	}

	// 4. Calculate Aggregated Exposures
	factsheet.SectorExposure, err = r.getExposure(ctx, portfolioID, "sector")
	if err != nil {
		return nil, err
	}

	factsheet.RegionalExposure, err = r.getExposure(ctx, portfolioID, "region")
	if err != nil {
		return nil, err
	}

	return factsheet, nil
}

// getExposure calculates the blended portfolio weight for a given exposure type (sector or region)
func (r *FactsheetRepo) getExposure(ctx context.Context, portfolioID int, exposureType string) ([]models.Exposure, error) {
	query := `
		SELECT ae.exposure_name, SUM(ph.weight * ae.weight_percentage) as blended_weight
		FROM portfolio_holdings ph
		JOIN asset_exposures ae ON ph.asset_ticker = ae.asset_ticker
		WHERE ph.portfolio_id = $1 AND ae.exposure_type = $2
		GROUP BY ae.exposure_name
		ORDER BY blended_weight DESC;`

	rows, err := r.DB.QueryContext(ctx, query, portfolioID, exposureType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exposures []models.Exposure
	for rows.Next() {
		var e models.Exposure
		if err := rows.Scan(&e.Name, &e.Percentage); err != nil {
			return nil, err
		}
		exposures = append(exposures, e)
	}
	return exposures, nil
}