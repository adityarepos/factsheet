package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	// TODO: Ensure this matches your go.mod module name
	"factsheet/internal/models"
)

type FactsheetRepository struct {
	db         *pgxpool.Pool
	mu         sync.RWMutex
	cache      map[int]*models.Factsheet
	cacheTTL   time.Duration
	expiration map[int]time.Time
}

func NewFactsheetRepository(db *pgxpool.Pool) *FactsheetRepository {
	return &FactsheetRepository{
		db:         db,
		cache:      make(map[int]*models.Factsheet),
		cacheTTL:   12 * time.Hour,
		expiration: make(map[int]time.Time),
	}
}

// Returns the Factsheet, a Boolean indicating if it was a Cache HIT (true), and an Error
func (repo *FactsheetRepository) GetFactsheet(ctx context.Context, portfolioID int) (*models.Factsheet, bool, error) {
	repo.mu.RLock()
	cachedData, exists := repo.cache[portfolioID]
	expTime, hasExp := repo.expiration[portfolioID]
	repo.mu.RUnlock()

	// CACHE HIT: Valid in memory, served instantly
	if exists && hasExp && time.Now().Before(expTime) {
		return cachedData, true, nil
	}

	// CACHE MISS: Write Lock required
	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Double-Check pattern
	if cachedData, exists = repo.cache[portfolioID]; exists && time.Now().Before(repo.expiration[portfolioID]) {
		return cachedData, true, nil
	}

	factsheet, err := repo.fetchFromDatabase(ctx, portfolioID)
	if err != nil {
		return nil, false, fmt.Errorf("database query failure: %w", err)
	}

	// Save to state memory
	repo.cache[portfolioID] = factsheet
	repo.expiration[portfolioID] = time.Now().Add(repo.cacheTTL)

	// Return FALSE because we hit the Cold Database
	return factsheet, false, nil
}

func (repo *FactsheetRepository) fetchFromDatabase(ctx context.Context, portfolioID int) (*models.Factsheet, error) {
	// Initialize explicitly to prevent 'null' arrays in JSON output
	factsheet := &models.Factsheet{
		Holdings:                   make([]models.Holding, 0),
		UltimateUnderlyingHoldings: make([]models.UltimateHolding, 0),
		SectorExposure:             make([]models.Exposure, 0),
		RegionalExposure:           make([]models.Exposure, 0),
	}

	err := repo.db.QueryRow(ctx, 
		"SELECT id, name, description FROM portfolios WHERE id = $1", portfolioID,
	).Scan(&factsheet.PortfolioID, &factsheet.Name, &factsheet.Description)
	if err != nil {
		return nil, err
	}

	holdingsQuery := `
		SELECT a.ticker, a.name, a.current_price, a.pe_ratio, ph.weight 
		FROM portfolio_holdings ph
		JOIN assets a ON ph.asset_ticker = a.ticker
		WHERE ph.portfolio_id = $1
		ORDER BY ph.weight DESC`
	
	hRows, err := repo.db.Query(ctx, holdingsQuery, portfolioID)
	if err != nil {
		return nil, err
	}
	defer hRows.Close()

	for hRows.Next() {
		var h models.Holding
		err := hRows.Scan(&h.Ticker, &h.Name, &h.CurrentPrice, &h.PERatio, &h.Weight)
		if err != nil {
			return nil, err
		}
		factsheet.Holdings = append(factsheet.Holdings, h)
	}

	lookThroughQuery := `
		SELECT 
			COALESCE(euh.holding_ticker, ph.asset_ticker) as true_ticker,
			COALESCE(euh.holding_name, a.name) as true_name,
			SUM(ph.weight * COALESCE(euh.holding_weight, 1.0000)) as true_effective_weight
		FROM portfolio_holdings ph
		JOIN assets a ON ph.asset_ticker = a.ticker
		LEFT JOIN etf_underlying_holdings euh ON ph.asset_ticker = euh.etf_ticker
		WHERE ph.portfolio_id = $1
		GROUP BY true_ticker, true_name
		ORDER BY true_effective_weight DESC`

	ltRows, err := repo.db.Query(ctx, lookThroughQuery, portfolioID)
	if err != nil {
		return nil, err
	}
	defer ltRows.Close()

	for ltRows.Next() {
		var lt models.UltimateHolding
		err := ltRows.Scan(&lt.Ticker, &lt.Name, &lt.TrueEffectiveWeight)
		if err != nil {
			return nil, err
		}
		factsheet.UltimateUnderlyingHoldings = append(factsheet.UltimateUnderlyingHoldings, lt)
	}

	sectorQuery := `
		SELECT ae.exposure_name, SUM(ph.weight * ae.weight_percentage) as blended_weight
		FROM portfolio_holdings ph
		JOIN asset_exposures ae ON ph.asset_ticker = ae.asset_ticker
		WHERE ph.portfolio_id = $1 AND ae.exposure_type = 'sector'
		GROUP BY ae.exposure_name
		ORDER BY blended_weight DESC`

	sRows, err := repo.db.Query(ctx, sectorQuery, portfolioID)
	if err != nil {
		return nil, err
	}
	defer sRows.Close()

	for sRows.Next() {
		var s models.Exposure
		err := sRows.Scan(&s.Name, &s.Percentage)
		if err != nil {
			return nil, err
		}
		factsheet.SectorExposure = append(factsheet.SectorExposure, s)
	}

	regionQuery := `
		SELECT ae.exposure_name, SUM(ph.weight * ae.weight_percentage) as blended_weight
		FROM portfolio_holdings ph
		JOIN asset_exposures ae ON ph.asset_ticker = ae.asset_ticker
		WHERE ph.portfolio_id = $1 AND ae.exposure_type = 'region'
		GROUP BY ae.exposure_name
		ORDER BY blended_weight DESC`

	rRows, err := repo.db.Query(ctx, regionQuery, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rRows.Close()

	for rRows.Next() {
		var r models.Exposure
		err := rRows.Scan(&r.Name, &r.Percentage)
		if err != nil {
			return nil, err
		}
		factsheet.RegionalExposure = append(factsheet.RegionalExposure, r)
	}

	return factsheet, nil
}