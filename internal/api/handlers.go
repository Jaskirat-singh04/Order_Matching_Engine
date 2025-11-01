package api

import (
	"encoding/json"
	"net/http"
	"order-matching-engine/internal/engine"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
)

// Server holds the HTTP server and matching engine
type Server struct {
	engine         *engine.MatchingEngine
	router         *mux.Router
	startTime      time.Time
	ordersReceived atomic.Int64
	ordersMatched  atomic.Int64
	ordersCancelled atomic.Int64
	tradesExecuted atomic.Int64
}

// NewServer creates a new API server
func NewServer() *Server {
	s := &Server{
		engine:    engine.NewMatchingEngine(),
		router:    mux.NewRouter(),
		startTime: time.Now(),
	}

	// Register routes
	s.registerRoutes()

	return s
}

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes() {
	// API v1 routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/orders", s.handleSubmitOrder).Methods("POST")
	api.HandleFunc("/orders/{order_id}", s.handleCancelOrder).Methods("DELETE")
	api.HandleFunc("/orders/{order_id}", s.handleGetOrder).Methods("GET")
	api.HandleFunc("/orderbook/{symbol}", s.handleGetOrderBook).Methods("GET")

	// Health and metrics
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/metrics", s.handleMetrics).Methods("GET")
}

// SubmitOrderRequest represents the JSON request body
type SubmitOrderRequest struct {
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`
	Type     string `json:"type"`
	Price    int64  `json:"price,omitempty"`
	Quantity int64  `json:"quantity"`
}

// handleSubmitOrder handles POST /api/v1/orders
func (s *Server) handleSubmitOrder(w http.ResponseWriter, r *http.Request) {
	var req SubmitOrderRequest

	// Parse JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate
	if req.Symbol == "" {
		respondError(w, http.StatusBadRequest, "symbol is required")
		return
	}
	if req.Side != "BUY" && req.Side != "SELL" {
		respondError(w, http.StatusBadRequest, "side must be BUY or SELL")
		return
	}
	if req.Type != "LIMIT" && req.Type != "MARKET" {
		respondError(w, http.StatusBadRequest, "type must be LIMIT or MARKET")
		return
	}
	if req.Quantity <= 0 {
		respondError(w, http.StatusBadRequest, "quantity must be positive")
		return
	}
	if req.Type == "LIMIT" && req.Price <= 0 {
		respondError(w, http.StatusBadRequest, "price must be positive for LIMIT orders")
		return
	}

	// Convert types
	side := engine.OrderSide(req.Side)
	orderType := engine.OrderType(req.Type)

	// Submit order
	result, err := s.engine.SubmitOrder(req.Symbol, side, orderType, req.Price, req.Quantity)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Update metrics
	s.ordersReceived.Add(1)
	if result.Status == engine.FILLED || result.Status == engine.PARTIAL_FILL {
		s.ordersMatched.Add(1)
		s.tradesExecuted.Add(int64(len(result.Trades)))
	}

	// Response status code
	statusCode := http.StatusOK
	if result.Status == engine.FILLED {
		statusCode = http.StatusOK
	} else if result.Status == engine.PARTIAL_FILL {
		statusCode = http.StatusAccepted
	} else if result.Status == engine.ACCEPTED {
		statusCode = http.StatusCreated
	}

	respondJSON(w, statusCode, result)
}

// handleCancelOrder handles DELETE /api/v1/orders/{order_id}
func (s *Server) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["order_id"]

	if orderID == "" {
		respondError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	err := s.engine.CancelOrder(orderID)
	if err != nil {
		if err.Error() == "order not found" {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	s.ordersCancelled.Add(1)

	response := map[string]interface{}{
		"order_id": orderID,
		"status":   "CANCELLED",
	}
	respondJSON(w, http.StatusOK, response)
}

// handleGetOrder handles GET /api/v1/orders/{order_id}
func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["order_id"]

	if orderID == "" {
		respondError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	order, err := s.engine.GetOrder(orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// handleGetOrderBook handles GET /api/v1/orderbook/{symbol}
func (s *Server) handleGetOrderBook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	if symbol == "" {
		respondError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	// Get depth parameter (default 10)
	depthStr := r.URL.Query().Get("depth")
	depth := 10
	if depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			depth = d
		}
	}

	snapshot, err := s.engine.GetOrderBook(symbol, depth)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, snapshot)
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime).Seconds()

	response := map[string]interface{}{
		"status":           "healthy",
		"uptime_seconds":   int64(uptime),
		"orders_processed": s.ordersReceived.Load(),
	}

	respondJSON(w, http.StatusOK, response)
}

// handleMetrics handles GET /metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Calculate orders in book
	ordersInBook := s.ordersReceived.Load() - s.ordersMatched.Load() - s.ordersCancelled.Load()

	response := map[string]interface{}{
		"orders_received":  s.ordersReceived.Load(),
		"orders_matched":   s.ordersMatched.Load(),
		"orders_cancelled": s.ordersCancelled.Load(),
		"orders_in_book":   ordersInBook,
		"trades_executed":  s.tradesExecuted.Load(),
	}

	respondJSON(w, http.StatusOK, response)
}

// Helper functions

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]string{
		"error": message,
	}
	respondJSON(w, statusCode, response)
}

// Start starts the HTTP server
func (s *Server) Start(port string) error {
	return http.ListenAndServe(":"+port, s.router)
}