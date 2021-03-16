package sample1

import (
	"fmt"
	"sync"
	"time"
)

// PriceService is a service that we can use to get prices for the items
// Calls to this service are expensive (they take time)
type PriceService interface {
	GetPriceFor(itemCode string) (float64, error)
}

type priceValue struct {
	Price float64
	CreatedAt time.Time
}

type priceResponse struct {
	Price float64
	Err   error
}

// TransparentCache is a cache that wraps the actual service
// The cache will remember prices we ask for, so that we don't have to wait on every call
// Cache should only return a price if it is not older than "maxAge", so that we don't get stale prices
type TransparentCache struct {
	actualPriceService PriceService
	maxAge             time.Duration
	prices             map[string]priceValue
	mutex              sync.Mutex
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]priceValue{},
	}
}

// GetPriceFor gets the price for the item, either from the cache or the actual service if it was not cached or too old
func (c *TransparentCache) GetPriceFor(itemCode string) (float64, error) {

	v, ok := c.prices[itemCode]
	if ok {
		if time.Since(v.CreatedAt) < c.maxAge {
			return v.Price, nil
		}
	}
	price, err := c.actualPriceService.GetPriceFor(itemCode)
	if err != nil {
		return 0, fmt.Errorf("getting price from service : %v", err.Error())
	}
	c.mutex.Lock()
	c.prices[itemCode] = priceValue{
		Price: price,
		CreatedAt: time.Now(),
	}
	c.mutex.Unlock()

	return price, nil
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	output := make(chan priceResponse, len(itemCodes))
	defer close(output)

	var wg sync.WaitGroup
	worker := func(code string) {
		price, err := c.GetPriceFor(code)
		output <- priceResponse{
			Price: price,
			Err:   err,
		}
		wg.Done()
	}

	wg.Add(len(itemCodes))
	for _, code := range itemCodes {
		go worker(code)
	}
	wg.Wait()

	results := []float64{}
	for i := 0; i < len(itemCodes); i++ {
		r := <-output
		if r.Err != nil {
			return results, r.Err
		}
		results = append(results, r.Price)
	}

	return results, nil
}
