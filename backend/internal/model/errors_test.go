package model

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
)

type sqlState string

func (s sqlState) Error() string    { return "pg " + string(s) }
func (s sqlState) SQLState() string { return string(s) }

func TestIsUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"sentinel", fmt.Errorf("x: %w", ErrUnavailable), true},
		{"dial refused", fmt.Errorf("connect: %w", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}), true},
		{"bare ECONNRESET", fmt.Errorf("read: %w", syscall.ECONNRESET), true},
		{"connection exception 08006", fmt.Errorf("q: %w", sqlState("08006")), true},
		{"cannot connect now 57P03", sqlState("57P03"), true},
		{"undefined table 42P01", sqlState("42P01"), false},
		{"unique violation 23505", sqlState("23505"), false},
		{"not found", ErrNotFound, false},
		{"context canceled", context.Canceled, false},
		{"plain", errors.New("boom"), false},
	}
	for _, tt := range tests {
		if got := IsUnavailable(tt.err); got != tt.want {
			t.Errorf("%s: IsUnavailable = %v, want %v", tt.name, got, tt.want)
		}
	}
}
