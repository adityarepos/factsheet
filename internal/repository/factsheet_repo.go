package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	// TODO: Replace "factsheet" with the exact module name defined in your go.mod file
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

func (repo *FactsheetRepository) GetFactsheet(ctx context.Context, portfolioID int) (*models.Factsheet, error) {
	// 1. Evaluate Current Memory Blocks using a Concurrent Read-Lock
	repo.mu.RLock()
	cachedData, exists := repo.cache[portfolioID]
	expTime, hasExp := repo.expiration[portfolioID]
	repo.mu.RUnlock()

	// TTL Condition Check: If cache exists and has not expired yet, serve instantly from RAM
	if exists && hasExp && time.Now().Before(expTime) {
		return cachedData, nil
	}

	// 2. CACHE MISS: Secure Full Write Lock to prevent thread race collisions during DB queries
	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Double-Check pattern: Did another concurrent thread populate the cache while we were waiting for the lock?
	if cachedData, exists = repo.cache[portfolioID]; exists && time.Now().Before(repo.expiration[portfolioID]) {
		return cachedData, nil
	}

	// 3. DATABASE CALL: Rebuild metrics using optimal relational aggregations
	factsheet, err := repo.fetchFromDatabase(ctx, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("database query failure: %w", err)
	}

	// 4. PRIMING STEP: Save to state memory and assign clean 12-hour expiration window
	repo.cache[portfolioID] = factsheet
	repo.expiration[portfolioID] = time.Now().Add(repo.cacheTTL)

	return factsheet, nil
}

func (repo *FactsheetRepository) fetchFromDatabase(ctx context.Context, portfolioID int) (*models.Factsheet, error) {
	factsheet := &models.Factsheet{
		Holdings:                  []models.Holding{},
		UltimateUnderlyingHoldings: []models.UltimateHolding{},
		SectorExposure:            []models.Exposure{},
		RegionalExposure:          []models.Exposure{},
	}

	// Query A: Fetch Base Portfolio Metadata Info
	err := repo.db.QueryRow(ctx, 
		"SELECT id, name, description FROM portfolios WHERE id = $1", portfolioID,
	).Scan(&factsheet.PortfolioID, &factsheet.Name, &factsheet.Description)
	if err != nil {
		return nil, err
	}

	// Query B: Fetch Top-Level Seeded Portfolio Allocations 
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

	// Query C: The Ultimate Look-Through Exposure Aggregator (SQL LEFT JOIN + COALESCE)
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

	// Query D: Extract Macro Factor Exposure Blocks (Sectors)
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

	// Query E: Extract Macro Factor Exposure Blocks (Regions)
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