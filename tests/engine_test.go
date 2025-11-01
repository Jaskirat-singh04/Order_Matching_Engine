package tests

import (
	"order-matching-engine/internal/engine"
	"testing"
)

func TestSimpleMatch(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Submit sell order
	sellResult, err := me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15050, 100)
	if err != nil {
		t.Fatalf("Failed to submit sell order: %v", err)
	}
	if sellResult.Status != engine.ACCEPTED {
		t.Errorf("Expected ACCEPTED, got %s", sellResult.Status)
	}

	// Submit matching buy order
	buyResult, err := me.SubmitOrder("AAPL", engine.BUY, engine.LIMIT, 15050, 100)
	if err != nil {
		t.Fatalf("Failed to submit buy order: %v", err)
	}

	if buyResult.Status != engine.FILLED {
		t.Errorf("Expected FILLED, got %s", buyResult.Status)
	}

	if buyResult.FilledQuantity != 100 {
		t.Errorf("Expected filled quantity 100, got %d", buyResult.FilledQuantity)
	}

	if len(buyResult.Trades) != 1 {
		t.Errorf("Expected 1 trade, got %d", len(buyResult.Trades))
	}
}

func TestPartialFill(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Submit sell order for 50 shares
	me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15050, 50)

	// Try to buy 100 shares
	buyResult, err := me.SubmitOrder("AAPL", engine.BUY, engine.LIMIT, 15050, 100)
	if err != nil {
		t.Fatalf("Failed to submit buy order: %v", err)
	}

	if buyResult.Status != engine.PARTIAL_FILL {
		t.Errorf("Expected PARTIAL_FILL, got %s", buyResult.Status)
	}

	if buyResult.FilledQuantity != 50 {
		t.Errorf("Expected filled quantity 50, got %d", buyResult.FilledQuantity)
	}

	if buyResult.RemainingQuantity != 50 {
		t.Errorf("Expected remaining quantity 50, got %d", buyResult.RemainingQuantity)
	}
}

func TestNoMatch(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Sell at $151
	sellResult, _ := me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15100, 100)
	if sellResult.Status != engine.ACCEPTED {
		t.Errorf("Expected ACCEPTED, got %s", sellResult.Status)
	}

	// Buy at $150 (no match)
	buyResult, _ := me.SubmitOrder("AAPL", engine.BUY, engine.LIMIT, 15000, 100)
	if buyResult.Status != engine.ACCEPTED {
		t.Errorf("Expected ACCEPTED, got %s", buyResult.Status)
	}

	if len(buyResult.Trades) != 0 {
		t.Errorf("Expected no trades, got %d", len(buyResult.Trades))
	}
}

func TestMarketOrder(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Add some sell orders
	me.SubmitOrder("TSLA", engine.SELL, engine.LIMIT, 20000, 100)
	me.SubmitOrder("TSLA", engine.SELL, engine.LIMIT, 20050, 200)

	// Market buy order
	result, err := me.SubmitOrder("TSLA", engine.BUY, engine.MARKET, 0, 150)
	if err != nil {
		t.Fatalf("Failed to submit market order: %v", err)
	}

	if result.Status != engine.FILLED {
		t.Errorf("Expected FILLED, got %s", result.Status)
	}

	if result.FilledQuantity != 150 {
		t.Errorf("Expected filled quantity 150, got %d", result.FilledQuantity)
	}

	// Should execute at 2 different prices
	if len(result.Trades) != 2 {
		t.Errorf("Expected 2 trades, got %d", len(result.Trades))
	}
}

func TestMarketOrderInsufficientLiquidity(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Add only 50 shares
	me.SubmitOrder("GOOGL", engine.SELL, engine.LIMIT, 14000, 50)

	// Try to buy 100 shares with market order
	_, err := me.SubmitOrder("GOOGL", engine.BUY, engine.MARKET, 0, 100)
	if err == nil {
		t.Error("Expected error for insufficient liquidity")
	}
}

func TestCancelOrder(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Submit order
	result, _ := me.SubmitOrder("AAPL", engine.BUY, engine.LIMIT, 15000, 100)
	orderID := result.OrderID

	// Cancel it
	err := me.CancelOrder(orderID)
	if err != nil {
		t.Errorf("Failed to cancel order: %v", err)
	}

	// Try to get the order
	_, err = me.GetOrder(orderID)
	if err == nil {
		t.Error("Expected error when getting cancelled order")
	}
}

func TestFIFOPriority(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Add 3 sell orders at same price, different times
	result1, _ := me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15050, 100)
	result2, _ := me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15050, 100)
	result3, _ := me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15050, 100)

	// Buy 150 shares (should match first 2 orders)
	buyResult, _ := me.SubmitOrder("AAPL", engine.BUY, engine.LIMIT, 15050, 150)

	if len(buyResult.Trades) != 2 {
		t.Errorf("Expected 2 trades, got %d", len(buyResult.Trades))
	}

	// First trade should be with first order
	if buyResult.Trades[0].SellerID != result1.OrderID {
		t.Error("FIFO priority violated: first order not matched first")
	}

	// Second trade should be with second order
	if buyResult.Trades[1].SellerID != result2.OrderID {
		t.Error("FIFO priority violated: second order not matched second")
	}

	// Third order should still be in book
	order3, err := me.GetOrder(result3.OrderID)
	if err != nil || order3.Status != engine.ACCEPTED {
		t.Error("Third order should still be in book")
	}
}

func TestMultipleSymbols(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Add orders for different symbols
	me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15050, 100)
	me.SubmitOrder("TSLA", engine.SELL, engine.LIMIT, 20000, 100)
	me.SubmitOrder("GOOGL", engine.SELL, engine.LIMIT, 14000, 100)

	// Get order books
	aaplBook, _ := me.GetOrderBook("AAPL", 10)
	tslaBook, _ := me.GetOrderBook("TSLA", 10)

	if len(aaplBook.Asks) != 1 {
		t.Error("AAPL book should have 1 ask")
	}

	if len(tslaBook.Asks) != 1 {
		t.Error("TSLA book should have 1 ask")
	}

	// Orders shouldn't cross symbols
	if aaplBook.Asks[0].Price == tslaBook.Asks[0].Price {
		// This is fine, just checking they're independent
	}
}

func TestPriceImprovement(t *testing.T) {
	me := engine.NewMatchingEngine()

	// Seller wants $150.00
	me.SubmitOrder("AAPL", engine.SELL, engine.LIMIT, 15000, 100)

	// Buyer willing to pay $151.00
	buyResult, _ := me.SubmitOrder("AAPL", engine.BUY, engine.LIMIT, 15100, 100)

	if len(buyResult.Trades) != 1 {
		t.Fatalf("Expected 1 trade")
	}

	// Trade should execute at seller's price (resting order)
	if buyResult.Trades[0].Price != 15000 {
		t.Errorf("Expected trade at 15000, got %d", buyResult.Trades[0].Price)
	}
}