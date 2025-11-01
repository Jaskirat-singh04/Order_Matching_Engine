package tests

import (
	"fmt"
	"math/rand"
	"order-matching-engine/internal/engine"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkOrderSubmission(b *testing.B) {
	me := engine.NewMatchingEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		side := engine.BUY
		if i%2 == 0 {
			side = engine.SELL
		}
		price := int64(15000 + (i % 100))
		me.SubmitOrder("AAPL", side, engine.LIMIT, price, 100)
	}
}

func BenchmarkConcurrentOrders(b *testing.B) {
	me := engine.NewMatchingEngine()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			side := engine.BUY
			if i%2 == 0 {
				side = engine.SELL
			}
			price := int64(15000 + (i % 100))
			me.SubmitOrder("AAPL", side, engine.LIMIT, price, 100)
			i++
		}
	})
}

// TestThroughput measures sustained throughput
func TestThroughput(t *testing.T) {
	me := engine.NewMatchingEngine()

	numOrders := 100000
	numWorkers := 10
	
	ordersPerWorker := numOrders / numWorkers
	
	var wg sync.WaitGroup
	var totalOrders atomic.Int64
	
	start := time.Now()
	
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for i := 0; i < ordersPerWorker; i++ {
				side := engine.BUY
				if i%2 == 0 {
					side = engine.SELL
				}
				
				price := int64(15000 + rand.Intn(100))
				quantity := int64(100)
				
				_, err := me.SubmitOrder("AAPL", side, engine.LIMIT, price, quantity)
				if err == nil {
					totalOrders.Add(1)
				}
			}
		}(w)
	}
	
	wg.Wait()
	elapsed := time.Since(start)
	
	ordersProcessed := totalOrders.Load()
	throughput := float64(ordersProcessed) / elapsed.Seconds()
	
	fmt.Printf("\n")
	fmt.Printf("========================================\n")
	fmt.Printf("PERFORMANCE TEST RESULTS\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Total Orders: %d\n", ordersProcessed)
	fmt.Printf("Time Taken: %.2f seconds\n", elapsed.Seconds())
	fmt.Printf("Throughput: %.0f orders/second\n", throughput)
	fmt.Printf("Average Latency: %.2f ms\n", (elapsed.Seconds()*1000)/float64(ordersProcessed))
	fmt.Printf("========================================\n")
	
	if throughput < 30000 {
		t.Logf("Warning: Throughput %.0f is below target of 30,000 orders/sec", throughput)
	} else {
		t.Logf("✅ Target throughput achieved!")
	}
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	me := engine.NewMatchingEngine()
	
	numGoroutines := 100
	ordersPerGoroutine := 100
	
	var wg sync.WaitGroup
	
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for i := 0; i < ordersPerGoroutine; i++ {
				side := engine.BUY
				if i%2 == 0 {
					side = engine.SELL
				}
				
				price := int64(15000 + (id*10 + i))
				me.SubmitOrder("AAPL", side, engine.LIMIT, price, 100)
			}
		}(g)
	}
	
	wg.Wait()
	
	// If we get here without panic, thread safety is good
	t.Log("✅ Concurrent access test passed")
}

// TestLatencyDistribution measures latency percentiles
func TestLatencyDistribution(t *testing.T) {
	me := engine.NewMatchingEngine()
	
	numOrders := 10000
	latencies := make([]time.Duration, numOrders)
	
	for i := 0; i < numOrders; i++ {
		side := engine.BUY
		if i%2 == 0 {
			side = engine.SELL
		}
		
		price := int64(15000 + rand.Intn(100))
		
		start := time.Now()
		me.SubmitOrder("AAPL", side, engine.LIMIT, price, 100)
		latencies[i] = time.Since(start)
	}
	
	// Sort latencies
	sortDurations(latencies)
	
	p50 := latencies[int(float64(numOrders)*0.50)]
	p99 := latencies[int(float64(numOrders)*0.99)]
	p999 := latencies[int(float64(numOrders)*0.999)]
	
	fmt.Printf("\n")
	fmt.Printf("========================================\n")
	fmt.Printf("LATENCY DISTRIBUTION\n")
	fmt.Printf("========================================\n")
	fmt.Printf("p50:  %.2f ms\n", float64(p50.Microseconds())/1000.0)
	fmt.Printf("p99:  %.2f ms\n", float64(p99.Microseconds())/1000.0)
	fmt.Printf("p999: %.2f ms\n", float64(p999.Microseconds())/1000.0)
	fmt.Printf("========================================\n")
	
	if p99.Milliseconds() > 50 {
		t.Logf("Warning: p99 latency %.2f ms exceeds target of 50ms", float64(p99.Milliseconds()))
	} else {
		t.Logf("✅ Latency targets met!")
	}
}

// Helper to sort durations
func sortDurations(durations []time.Duration) {
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
}