package main

import (
	"fmt"
	"order-matching-engine/internal/engine"
)

func main() {
	// Create order book for AAPL
	book := engine.NewOrderBook("AAPL")
	
	// Create some orders
	buyOrder := engine.NewOrder("AAPL", engine.BUY, engine.LIMIT, 15050, 100)
	sellOrder := engine.NewOrder("AAPL", engine.SELL, engine.LIMIT, 15100, 50)
	
	// Add to book
	book.AddOrder(buyOrder)
	book.AddOrder(sellOrder)
	
	// Check best prices
	fmt.Printf("Best Bid: %d (should be 15050)\n", book.GetBestBid())
	fmt.Printf("Best Ask: %d (should be 15100)\n", book.GetBestAsk())
	
	fmt.Println("\n✅ Basic order book working!")
}