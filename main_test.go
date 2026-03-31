package main

import (
	"testing"
)

func TestHandleMessage(t *testing.T) {
	msg := `{"m":"qsd","p":[null,{"n":"OANDA:XAUUSD","v":{"lp":100.5,"bid":100.1,"ask":100.9}}]}`

	handleMessage(msg)

	mu.RLock()
	defer mu.RUnlock()

	q, ok := data["OANDA:XAUUSD"]
	if !ok {
		t.Fatal("symbol not stored")
	}

	if !q.HasPrice || q.Price != 100.5 {
		t.Fatal("price not updated")
	}
	if !q.HasBid || q.Bid != 100.1 {
		t.Fatal("bid not updated")
	}
	if !q.HasAsk || q.Ask != 100.9 {
		t.Fatal("ask not updated")
	}
}

func TestFilterIncomplete(t *testing.T) {
	q := &Quote{}
	data["TEST"] = q

	if q.HasPrice || q.HasBid || q.HasAsk {
		t.Fatal("should be incomplete")
	}
}
