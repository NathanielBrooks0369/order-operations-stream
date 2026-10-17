package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/infrai-examples/order-operations-stream/internal/infrai"
	"github.com/infrai-examples/order-operations-stream/internal/orders"
)

const channel = "store-operations"

type server struct{ client *infrai.Client }

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.New(key)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.CreateChannel(ctx, channel); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", (&server{client: client}).recordOrder)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	log.Printf("order dashboard feed listening on :8080, channel=%s", channel)
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) recordOrder(w http.ResponseWriter, r *http.Request) {
	var update orders.Update
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	snapshot, err := orders.Decide(update)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if err := s.client.SendSnapshot(r.Context(), channel, snapshot); err != nil {
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message})
			return
		}
		log.Printf("send order snapshot: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dashboard handoff failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, snapshot)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Print(fmt.Errorf("encode response: %w", err))
	}
}
