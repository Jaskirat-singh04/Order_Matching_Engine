package engine

// import "time"

// OrderSide represents buy or sell
type OrderSide string

const (
	BUY  OrderSide = "BUY"
	SELL OrderSide = "SELL"
)

// OrderType represents limit or market
type OrderType string

const (
	LIMIT  OrderType = "LIMIT"
	MARKET OrderType = "MARKET"
)

// OrderStatus represents order state
type OrderStatus string

const (
	ACCEPTED     OrderStatus = "ACCEPTED"
	PARTIAL_FILL OrderStatus = "PARTIAL_FILL"
	FILLED       OrderStatus = "FILLED"
	CANCELLED    OrderStatus = "CANCELLED"
)

// Order represents a single order
type Order struct {
	ID             string      `json:"order_id"`
	Symbol         string      `json:"symbol"`
	Side           OrderSide   `json:"side"`
	Type           OrderType   `json:"type"`
	Price          int64       `json:"price"`           // in cents
	Quantity       int64       `json:"quantity"`
	FilledQuantity int64       `json:"filled_quantity"`
	Status         OrderStatus `json:"status"`
	Timestamp      int64       `json:"timestamp"` // Unix milliseconds
}

// Trade represents an executed trade
type Trade struct {
	ID        string `json:"trade_id"`
	Price     int64  `json:"price"`
	Quantity  int64  `json:"quantity"`
	Timestamp int64  `json:"timestamp"`
	BuyerID   string `json:"buyer_id"`
	SellerID  string `json:"seller_id"`
}

// PriceLevel represents all orders at a specific price
type PriceLevel struct {
	Price  int64
	Orders []*Order
}