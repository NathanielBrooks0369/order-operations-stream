package orders

import (
	"testing"
	"time"
)

func TestDecideOrderUpdate(t *testing.T) {
	at := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		state       State
		wantStatus  string
		wantReceipt float64
		wantShipped float64
	}{
		{name: "checkout stays active", state: CheckoutStarted, wantStatus: "checking_out"},
		{name: "payment issues receipt", state: Paid, wantStatus: "receipt_issued", wantReceipt: 84.50},
		{name: "fulfillment updates customer", state: Fulfilled, wantStatus: "order_shipped", wantReceipt: 84.50, wantShipped: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := Decide(Update{OrderID: "ord-1042", Customer: "cus-17", State: tt.state, Total: 84.50, OccurredAt: at})
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.CustomerStatus != tt.wantStatus || snapshot.ReceiptTotal != tt.wantReceipt {
				t.Fatalf("got status=%q receipt=%.2f", snapshot.CustomerStatus, snapshot.ReceiptTotal)
			}
			if got := snapshot.Points[1].Value; got != tt.wantShipped {
				t.Fatalf("fulfillment metric = %.0f, want %.0f", got, tt.wantShipped)
			}
		})
	}
}
