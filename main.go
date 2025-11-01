package main

import (
	"fmt"
	"log"
	"order-matching-engine/internal/api"
)

func main() {
	fmt.Println("🚀 Starting Order Matching Engine...")

	// Create server
	server := api.NewServer()

	// Start server
	port := "8081"
	fmt.Printf("✅ Server running on http://localhost:%s\n", port)
	fmt.Println("📖 Endpoints:")
	fmt.Println("   POST   /api/v1/orders")
	fmt.Println("   DELETE /api/v1/orders/{id}")
	fmt.Println("   GET    /api/v1/orders/{id}")
	fmt.Println("   GET    /api/v1/orderbook/{symbol}")
	fmt.Println("   GET    /health")
	fmt.Println("   GET    /metrics")
	fmt.Println()

	// Start server (blocking call)
	if err := server.Start(port); err != nil {
		log.Fatal("Server failed:", err)
	}
}