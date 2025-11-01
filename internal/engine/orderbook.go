package engine

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OrderBook manages all orders for a symbol
type OrderBook struct {
	Symbol string
	
	// Buy orders sorted by price (high to low), then time
	Bids []*PriceLevel
	
	// Sell orders sorted by price (low to high), then time
	Asks []*PriceLevel
	
	// Quick lookup by order ID
	Orders map[string]*Order
	
	// Lock for thread safety
	mu sync.RWMutex
}

// NewOrderBook creates a new order book
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   make([]*PriceLevel, 0),
		Asks:   make([]*PriceLevel, 0),
		Orders: make(map[string]*Order),
	}
}

// AddOrder adds an order to the book (not matched yet)
func (ob *OrderBook) AddOrder(order *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	
	// Store in lookup map
	ob.Orders[order.ID] = order
	
	// Add to appropriate side
	if order.Side == BUY {
		ob.addToBids(order)
	} else {
		ob.addToAsks(order)
	}
}

// addToBids adds order to buy side
func (ob *OrderBook) addToBids(order *Order) {
	// Find or create price level
	for _, level := range ob.Bids {
		if level.Price == order.Price {
			level.Orders = append(level.Orders, order)
			return
		}
	}
	
	// Create new price level
	newLevel := &PriceLevel{
		Price:  order.Price,
		Orders: []*Order{order},
	}
	ob.Bids = append(ob.Bids, newLevel)
	
	// Sort: highest price first
	sort.Slice(ob.Bids, func(i, j int) bool {
		return ob.Bids[i].Price > ob.Bids[j].Price
	})
}

// addToAsks adds order to sell side
func (ob *OrderBook) addToAsks(order *Order) {
	// Find or create price level
	for _, level := range ob.Asks {
		if level.Price == order.Price {
			level.Orders = append(level.Orders, order)
			return
		}
	}
	
	// Create new price level
	newLevel := &PriceLevel{
		Price:  order.Price,
		Orders: []*Order{order},
	}
	ob.Asks = append(ob.Asks, newLevel)
	
	// Sort: lowest price first
	sort.Slice(ob.Asks, func(i, j int) bool {
		return ob.Asks[i].Price < ob.Asks[j].Price
	})
}

// RemoveOrder removes an order from the book
func (ob *OrderBook) RemoveOrder(orderID string) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	
	order, exists := ob.Orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}
	
	// Only delete if it's being cancelled (not if it's filled)
	// Filled orders should stay in the map for status queries
	
	// Remove from price level
	if order.Side == BUY {
		ob.removeFromBids(order)
	} else {
		ob.removeFromAsks(order)
	}
	
	return nil
}

func (ob *OrderBook) removeFromBids(order *Order) {
	for i, level := range ob.Bids {
		if level.Price == order.Price {
			// Remove order from this level
			for j, o := range level.Orders {
				if o.ID == order.ID {
					level.Orders = append(level.Orders[:j], level.Orders[j+1:]...)
					break
				}
			}
			// If level is empty, remove it
			if len(level.Orders) == 0 {
				ob.Bids = append(ob.Bids[:i], ob.Bids[i+1:]...)
			}
			return
		}
	}
}

func (ob *OrderBook) removeFromAsks(order *Order) {
	for i, level := range ob.Asks {
		if level.Price == order.Price {
			// Remove order from this level
			for j, o := range level.Orders {
				if o.ID == order.ID {
					level.Orders = append(level.Orders[:j], level.Orders[j+1:]...)
					break
				}
			}
			// If level is empty, remove it
			if len(level.Orders) == 0 {
				ob.Asks = append(ob.Asks[:i], ob.Asks[i+1:]...)
			}
			return
		}
	}
}

func (ob *OrderBook) RemoveFromPriceLevels(order *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	
	if order.Side == BUY {
		ob.removeFromBids(order)
	} else {
		ob.removeFromAsks(order)
	}
}

// GetBestBid returns highest buy price
func (ob *OrderBook) GetBestBid() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	
	if len(ob.Bids) == 0 {
		return 0
	}
	return ob.Bids[0].Price
}

// GetBestAsk returns lowest sell price
func (ob *OrderBook) GetBestAsk() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	
	if len(ob.Asks) == 0 {
		return 0
	}
	return ob.Asks[0].Price
}

// Helper function to create new order with generated ID
func NewOrder(symbol string, side OrderSide, orderType OrderType, price, quantity int64) *Order {
	return &Order{
		ID:             uuid.New().String(),
		Symbol:         symbol,
		Side:           side,
		Type:           orderType,
		Price:          price,
		Quantity:       quantity,
		FilledQuantity: 0,
		Status:         ACCEPTED,
		Timestamp:      time.Now().UnixMilli(),
	}
}