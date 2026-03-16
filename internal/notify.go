package athanor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type hookPayload struct {
	Text string `json:"text"`
}

func Notify(ctx context.Context, hook, hostname, msg string) error {
	marshaledPayload, err := json.Marshal(hookPayload{fmt.Sprintf("%v: %v", hostname, msg)})
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook, bytes.NewBuffer(marshaledPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	return nil
}
