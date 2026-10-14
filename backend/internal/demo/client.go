package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal JSON client for the public API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// APIError is a non-2xx response.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body) }

// Do sends method path with body (nil for none) and decodes the response
// into out (nil to discard). Non-2xx responses return *APIError.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// VehicleIDs maps vehicle_number → id from GET /vehicles.
func (c *Client) VehicleIDs(ctx context.Context) (map[string]string, error) {
	var resp struct {
		Vehicles []struct {
			ID            string `json:"id"`
			VehicleNumber string `json:"vehicle_number"`
		} `json:"vehicles"`
	}
	if err := c.Do(ctx, http.MethodGet, "/vehicles", nil, &resp); err != nil {
		return nil, err
	}
	ids := make(map[string]string, len(resp.Vehicles))
	for _, v := range resp.Vehicles {
		ids[v.VehicleNumber] = v.ID
	}
	return ids, nil
}

// GeofenceIDs maps name → id from GET /geofences.
func (c *Client) GeofenceIDs(ctx context.Context) (map[string]string, error) {
	var resp struct {
		Geofences []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"geofences"`
	}
	if err := c.Do(ctx, http.MethodGet, "/geofences", nil, &resp); err != nil {
		return nil, err
	}
	ids := make(map[string]string, len(resp.Geofences))
	for _, g := range resp.Geofences {
		ids[g.Name] = g.ID
	}
	return ids, nil
}
