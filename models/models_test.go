package models

import "testing"

func TestNewBalance(t *testing.T) {
	_ = NewBalance(0.1, 0.05)
}
