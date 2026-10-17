package orders

import (
	"fmt"
	"time"
)

type State string

const (
	CheckoutStarted State = "checkout_started"
	Paid            State = "paid"
	Fulfilled       State = "fulfilled"
)

type Update struct {
	OrderID    string    `json:"order_id"`
	Customer   string    `json:"customer_id"`
	State      State     `json:"state"`
	Total      float64   `json:"total"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Point struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp string            `json:"timestamp"`
	Tags      map[string]string `json:"tags"`
}

type Snapshot struct {
	OrderID              string  `json:"order_id"`
	CustomerID           string  `json:"customer_id"`
	OrderState           State   `json:"order_state"`
	CustomerStatus       string  `json:"customer_status"`
	CheckoutActive       float64 `json:"checkout_active"`
	FulfillmentCompleted float64 `json:"fulfillment_completed"`
	ReceiptTotal         float64 `json:"receipt_total"`
	OccurredAt           string  `json:"occurred_at"`
	Points               []Point `json:"-"`
}

func Decide(update Update) (Snapshot, error) {
	if update.OrderID == "" || update.Customer == "" || update.OccurredAt.IsZero() {
		return Snapshot{}, fmt.Errorf("order_id, customer_id, and occurred_at are required")
	}
	if update.Total < 0 {
		return Snapshot{}, fmt.Errorf("total must be non-negative")
	}

	status := "checking_out"
	values := map[string]float64{
		"commerce.checkout.started":      1,
		"commerce.fulfillment.completed": 0,
		"commerce.receipts.total":        0,
	}
	switch update.State {
	case CheckoutStarted:
	case Paid:
		status = "receipt_issued"
		values["commerce.checkout.started"] = 0
		values["commerce.receipts.total"] = update.Total
	case Fulfilled:
		status = "order_shipped"
		values["commerce.checkout.started"] = 0
		values["commerce.fulfillment.completed"] = 1
		values["commerce.receipts.total"] = update.Total
	default:
		return Snapshot{}, fmt.Errorf("unknown order state %q", update.State)
	}

	timestamp := update.OccurredAt.UTC().Format(time.RFC3339)
	tags := map[string]string{"order_id": update.OrderID, "state": string(update.State)}
	points := make([]Point, 0, 3)
	for _, name := range []string{"commerce.checkout.started", "commerce.fulfillment.completed", "commerce.receipts.total"} {
		points = append(points, Point{Name: name, Value: values[name], Timestamp: timestamp, Tags: tags})
	}
	return Snapshot{
		OrderID: update.OrderID, CustomerID: update.Customer, OrderState: update.State,
		CustomerStatus: status, CheckoutActive: values["commerce.checkout.started"],
		FulfillmentCompleted: values["commerce.fulfillment.completed"], ReceiptTotal: values["commerce.receipts.total"],
		OccurredAt: timestamp, Points: points,
	}, nil
}
