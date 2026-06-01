package models

type Holding struct {
	Ticker       string  `json:"ticker"`
	Name         string  `json:"name"`
	Weight       float64 `json:"weight"`
	CurrentPrice float64 `json:"current_price"`
	PERatio      float64 `json:"pe_ratio"`
}

type UltimateHolding struct {
	Ticker              string  `json:"ticker"`
	Name                string  `json:"name"`
	TrueEffectiveWeight float64 `json:"true_effective_weight"`
}

type Exposure struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
}

type Factsheet struct {
	PortfolioID                int               `json:"portfolio_id"`
	Name                       string            `json:"name"`
	Description                string            `json:"description"`
	Holdings                   []Holding         `json:"holdings"`
	UltimateUnderlyingHoldings []UltimateHolding `json:"ultimate_underlying_holdings"`
	SectorExposure             []Exposure        `json:"sector_exposure"`
	RegionalExposure           []Exposure        `json:"regional_exposure"`
}