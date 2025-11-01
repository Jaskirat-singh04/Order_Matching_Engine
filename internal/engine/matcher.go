package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MatchingEngine manages order books for multiple symbols
type MatchingEngine struct {
	books map[string]*OrderBook // symbol -> OrderBook
	mu    sync.RWMutex
}

// NewMatchingEngine creates a new matching engine
func NewMatchingEngine() *MatchingEngine {
	return &MatchingEngine{
		books: make(map[string]*OrderBook),
	}
}

// GetOrCreateBook gets or creates an order book for a symbol
func (me *MatchingEngine) GetOrCreateBook(symbol string) *OrderBook {
	me.mu.Lock()
	defer me.mu.Unlock()

	if book, exists := me.books[symbol]; exists {
		return book
	}

	book := NewOrderBook(symbol)
	me.books[symbol] = book
	return book
}

// OrderResult represents the result of submitting an order
type OrderResult struct {
	OrderID          string      `json:"order_id"`
	Status           OrderStatus `json:"status"`
	FilledQuantity   int64       `json:"filled_quantity,omitempty"`
	RemainingQuantity int64      `json:"remaining_quantity,omitempty"`
	Trades           []Trade     `json:"trades,omitempty"`
	Message          string      `json:"message,omitempty"`
}

// SubmitOrder submits an order and attempts to match it
func (me *MatchingEngine) SubmitOrder(symbol string, side OrderSide, orderType OrderType, price, quantity int64) (*OrderResult, error) {
	// Validation
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if orderType == LIMIT && price <= 0 {
		return nil, fmt.Errorf("price must be positive for limit orders")
	}

	// Get order book
	book := me.GetOrCreateBook(symbol)

	// Create order
	order := NewOrder(symbol, side, orderType, price, quantity)

	// Try to match
	trades, err := me.matchOrder(book, order)
	if err != nil {
		return nil, err
	}

	// Build result
	result := &OrderResult{
		OrderID:        order.ID,
		Status:         order.Status,
		FilledQuantity: order.FilledQuantity,
		Trades:         trades,
	}

	// If not fully filled and it's a limit order, add to book
	if order.FilledQuantity < order.Quantity && orderType == LIMIT {
		remaining := order.Quantity - order.FilledQuantity
		result.RemainingQuantity = remaining
		book.AddOrder(order)
		
		if order.FilledQuantity > 0 {
			result.Status = PARTIAL_FILL
			result.Message = "Order partially filled and added to book"
		} else {
			result.Status = ACCEPTED
			result.Message = "Order added to book"
		}
	} else if order.FilledQuantity == order.Quantity {
		result.Status = FILLED
		result.Message = "Order fully filled"
	}

	return result, nil
}

// matchOrder attempts to match an order against the book
func (me *MatchingEngine) matchOrder(book *OrderBook, order *Order) ([]Trade, error) {
	trades := []Trade{}

	if order.Type == MARKET {
		// Market orders must execute immediately
		t, err := me.matchMarketOrder(book, order)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t...)
	} else {
		// Limit orders
		t := me.matchLimitOrder(book, order)
		trades = append(trades, t...)
	}

	return trades, nil
}

// matchLimitOrder matches a limit order
func (me *MatchingEngine) matchLimitOrder(book *OrderBook, order *Order) []Trade {
	book.mu.Lock()
	defer book.mu.Unlock()

	trades := []Trade{}

	if order.Side == BUY {
		// Match against asks (sell orders)
		trades = me.matchBuyOrder(book, order)
	} else {
		// Match against bids (buy orders)
		trades = me.matchSellOrder(book, order)
	}

	return trades
}

// matchBuyOrder matches a buy order against sell orders
func (me *MatchingEngine) matchBuyOrder(book *OrderBook, buyOrder *Order) []Trade {
	trades := []Trade{}

	// Walk through asks (sell orders) from lowest price
	for len(book.Asks) > 0 && buyOrder.FilledQuantity < buyOrder.Quantity {
		bestAsk := book.Asks[0]

		// Check if prices cross
		if buyOrder.Type == LIMIT && buyOrder.Price < bestAsk.Price {
			// No match possible
			break
		}

		// Match against orders at this price level (FIFO)
		for len(bestAsk.Orders) > 0 && buyOrder.FilledQuantity < buyOrder.Quantity {
			sellOrder := bestAsk.Orders[0]

			// Calculate trade quantity
			remainingBuy := buyOrder.Quantity - buyOrder.FilledQuantity
			remainingSell := sellOrder.Quantity - sellOrder.FilledQuantity
			tradeQty := min(remainingBuy, remainingSell)

			// Execute trade at the sell order's price (resting order price)
			trade := Trade{
				ID:        uuid.New().String(),
				Price:     sellOrder.Price,
				Quantity:  tradeQty,
				Timestamp: time.Now().UnixMilli(),
				BuyerID:   buyOrder.ID,
				SellerID:  sellOrder.ID,
			}
			trades = append(trades, trade)

			// Update filled quantities
			buyOrder.FilledQuantity += tradeQty
			sellOrder.FilledQuantity += tradeQty

			// Update statuses
			if sellOrder.FilledQuantity == sellOrder.Quantity {
				sellOrder.Status = FILLED
				// Remove from book
				bestAsk.Orders = bestAsk.Orders[1:]
			} else {
				sellOrder.Status = PARTIAL_FILL
			}
		}

		// If this price level is empty, remove it
		if len(bestAsk.Orders) == 0 {
			book.Asks = book.Asks[1:]
		}
	}

	return trades
}

// matchSellOrder matches a sell order against buy orders
func (me *MatchingEngine) matchSellOrder(book *OrderBook, sellOrder *Order) []Trade {
	trades := []Trade{}

	// Walk through bids (buy orders) from highest price
	for len(book.Bids) > 0 && sellOrder.FilledQuantity < sellOrder.Quantity {
		bestBid := book.Bids[0]

		// Check if prices cross
		if sellOrder.Type == LIMIT && sellOrder.Price > bestBid.Price {
			// No match possible
			break
		}

		// Match against orders at this price level (FIFO)
		for len(bestBid.Orders) > 0 && sellOrder.FilledQuantity < sellOrder.Quantity {
			buyOrder := bestBid.Orders[0]

			// Calculate trade quantity
			remainingSell := sellOrder.Quantity - sellOrder.FilledQuantity
			remainingBuy := buyOrder.Quantity - buyOrder.FilledQuantity
			tradeQty := min(remainingSell, remainingBuy)

			// Execute trade at the buy order's price (resting order price)
			trade := Trade{
				ID:        uuid.New().String(),
				Price:     buyOrder.Price,
				Quantity:  tradeQty,
				Timestamp: time.Now().UnixMilli(),
				BuyerID:   buyOrder.ID,
				SellerID:  sellOrder.ID,
			}
			trades = append(trades, trade)

			// Update filled quantities
			sellOrder.FilledQuantity += tradeQty
			buyOrder.FilledQuantity += tradeQty

			// Update statuses
			if buyOrder.FilledQuantity == buyOrder.Quantity {
				buyOrder.Status = FILLED
				// Remove from book
				bestBid.Orders = bestBid.Orders[1:]
				// DON'T delete from book.Orders - keep it for queries
				// delete(book.Orders, buyOrder.ID)  // <-- REMOVE THIS LINE
			} else {
				buyOrder.Status = PARTIAL_FILL
}

		}

		// If this price level is empty, remove it
		if len(bestBid.Orders) == 0 {
			book.Bids = book.Bids[1:]
		}
	}

	return trades
}

// matchMarketOrder matches a market order (must execute immediately or fail)
func (me *MatchingEngine) matchMarketOrder(book *OrderBook, order *Order) ([]Trade, error) {
	book.mu.Lock()
	defer book.mu.Unlock()

	// Check if there's enough liquidity
	availableLiquidity := int64(0)

	if order.Side == BUY {
		for _, level := range book.Asks {
			for _, o := range level.Orders {
				availableLiquidity += (o.Quantity - o.FilledQuantity)
			}
		}
	} else {
		for _, level := range book.Bids {
			for _, o := range level.Orders {
				availableLiquidity += (o.Quantity - o.FilledQuantity)
			}
		}
	}

	if availableLiquidity < order.Quantity {
		return nil, fmt.Errorf("insufficient liquidity: only %d shares available, requested %d", availableLiquidity, order.Quantity)
	}

	// Execute the market order (same logic as limit but no price check)
	var trades []Trade
	if order.Side == BUY {
		trades = me.matchBuyOrder(book, order)
	} else {
		trades = me.matchSellOrder(book, order)
	}

	return trades, nil
}

// CancelOrder cancels an order
func (me *MatchingEngine) CancelOrder(orderID string) error {
	me.mu.RLock()
	defer me.mu.RUnlock()

	// Find which book has this order
	for _, book := range me.books {
		book.mu.RLock()
		order, exists := book.Orders[orderID]
		book.mu.RUnlock()

		if exists {
			if order.Status == FILLED {
				return fmt.Errorf("cannot cancel: order already filled")
			}
			order.Status = CANCELLED
			return book.RemoveOrder(orderID)
		}
	}

	return fmt.Errorf("order not found")
}

// GetOrder retrieves an order by ID
func (me *MatchingEngine) GetOrder(orderID string) (*Order, error) {
	me.mu.RLock()
	defer me.mu.RUnlock()

	for _, book := range me.books {
		book.mu.RLock()
		order, exists := book.Orders[orderID]
		book.mu.RUnlock()

		if exists {
			return order, nil
		}
	}

	return nil, fmt.Errorf("order not found")
}

// GetOrderBook returns the order book for a symbol
func (me *MatchingEngine) GetOrderBook(symbol string, depth int) (*OrderBookSnapshot, error) {
	book := me.GetOrCreateBook(symbol)

	book.mu.RLock()
	defer book.mu.RUnlock()

	snapshot := &OrderBookSnapshot{
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Bids:      []PriceLevelSnapshot{},
		Asks:      []PriceLevelSnapshot{},
	}

	// Get bids (up to depth levels)
	for i := 0; i < len(book.Bids) && i < depth; i++ {
		level := book.Bids[i]
		totalQty := int64(0)
		for _, order := range level.Orders {
			totalQty += (order.Quantity - order.FilledQuantity)
		}
		if totalQty > 0 {
			snapshot.Bids = append(snapshot.Bids, PriceLevelSnapshot{
				Price:    level.Price,
				Quantity: totalQty,
			})
		}
	}

	// Get asks (up to depth levels)
	for i := 0; i < len(book.Asks) && i < depth; i++ {
		level := book.Asks[i]
		totalQty := int64(0)
		for _, order := range level.Orders {
			totalQty += (order.Quantity - order.FilledQuantity)
		}
		if totalQty > 0 {
			snapshot.Asks = append(snapshot.Asks, PriceLevelSnapshot{
				Price:    level.Price,
				Quantity: totalQty,
			})
		}
	}

	return snapshot, nil
}

// OrderBookSnapshot represents a point-in-time view of the order book
type OrderBookSnapshot struct {
	Symbol    string                `json:"symbol"`
	Timestamp int64                 `json:"timestamp"`
	Bids      []PriceLevelSnapshot  `json:"bids"`
	Asks      []PriceLevelSnapshot  `json:"asks"`
}

// PriceLevelSnapshot represents aggregated quantity at a price level
type PriceLevelSnapshot struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

// Helper function
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}