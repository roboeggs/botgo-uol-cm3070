package generate 

import (
	"sync"
	"time"


	"botgo/internal/database"
	"botgo/internal/market"

)


type MarketCacheItem struct {
	Data market.MarketData
	ExpiresAt time.Time
}

type LevelsCacheItem struct {
	Data *database.DetectorStateRow
	ExpiresAt time.Time
}

type ActiveTickersCacheItem struct {
	Tickers []string
	ExpiresAt time.Time
}

type Cache struct {
	mu sync.RWMutex

	market map[string]MarketCacheItem
	levels map[string]LevelsCacheItem

	activeTickers *ActiveTickersCacheItem
}

func NewCache() *Cache {
	return &Cache{
		market: make(map[string]MarketCacheItem),
		levels: make(map[string]LevelsCacheItem),
	}
}


func (c *Cache) GetMarket(ticker string) (market.MarketData, bool) {

	c.mu.RLock()

	item, ok := c.market[ticker]

	c.mu.RUnlock()

	if !ok {
		return market.MarketData{}, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		delete(c.market, ticker)
		c.mu.Unlock()

		return market.MarketData{}, false
	}

	return item.Data, true
}

func (c *Cache) SetMarket(
	ticker string,
	data market.MarketData,
) {

	c.mu.Lock()
	defer c.mu.Unlock()

	c.market[ticker] = MarketCacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
}


func endOfDay() time.Time {
	now := time.Now()

	return time.Date(
		now.Year(),
		now.Month(),
		now.Day()+1,
		0,
		0,
		0,
		0,
		now.Location(),
	)
}

func (c *Cache) GetLevels(ticker string,) (*database.DetectorStateRow, bool) {

	c.mu.RLock()

	item, ok := c.levels[ticker]

	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		delete(c.levels, ticker)
		c.mu.Unlock()

		return nil, false
	}

	return item.Data, true
}

func (c *Cache) SetLevels(
	ticker string,
	data *database.DetectorStateRow,
) {

	c.mu.Lock()
	defer c.mu.Unlock()

	c.levels[ticker] = LevelsCacheItem{
		Data:      data,
		ExpiresAt: endOfDay(),
	}
}


func (c *Cache) GetActiveTickers() ([]string, bool) {

	c.mu.RLock()

	item := c.activeTickers

	c.mu.RUnlock()

	if item == nil {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		c.activeTickers = nil
		c.mu.Unlock()

		return nil, false
	}

	return item.Tickers, true
}

func (c *Cache) SetActiveTickers(tickers []string) {

	c.mu.Lock()
	defer c.mu.Unlock()

	// We copy the slice so that external code cannot modify it.
	copyTickers := append([]string(nil), tickers...)

	c.activeTickers = &ActiveTickersCacheItem{
		Tickers:   copyTickers,
		ExpiresAt: endOfDay(),
	}
}