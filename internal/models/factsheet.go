package models

// Factsheet represents the final aggregated JSON response
type Factsheet struct {
	PortfolioID               int               `json:"portfolio_id"`
	Name                      string            `json:"name"`
	Description               string            `json:"description"`
	Holdings                  []Holding         `json:"holdings"`
	UltimateUnderlyingHoldings []UltimateHolding `json:"ultimate_underlying_holdings"` // ADVANCED LOOK-THROUGH
	SectorExposure            []Exposure        `json:"sector_exposure"`
	RegionalExposure          []Exposure        `json:"regional_exposure"`
}

// Holding represents the high-level asset allocation within the portfolio (including the raw ETFs)
type Holding struct {
	Ticker           string   `json:"ticker"`
	Name             string   `json:"name"`
	Weight           float64  `json:"weight"`
	CurrentPrice     float64  `json:"current_price"`
	PERatio          *float64 `json:"pe_ratio"` // Pointer handles SQL nulls
	Beta             *float64 `json:"beta"`
	DividendYield    *float64 `json:"dividend_yield"`
	FiftyTwoWeekHigh *float64 `json:"fifty_two_week_high"`
}

// UltimateHolding represents the true proportional underlying holding computed across the fund dataset
type UltimateHolding struct {
	Ticker             string  `json:"ticker"`
	Name               string  `json:"name"`
	TrueEffectiveWeight float64 `json:"true_effective_weight"`
}

// Exposure is a generic struct used for both Sector and Regional geographic weights
type Exposure struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
}