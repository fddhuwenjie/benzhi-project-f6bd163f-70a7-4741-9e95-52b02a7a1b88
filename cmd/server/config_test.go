package main

import "testing"

func TestValidateAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:19081", "localhost:19082", "[::1]:19083"} {
		if _, err := validateAddress(address); err != nil {
			t.Errorf("valid address %s: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:19081", ":19081", "127.0.0.1:0", "127.0.0.1:70000"} {
		if _, err := validateAddress(address); err == nil {
			t.Errorf("unsafe address accepted: %s", address)
		}
	}
}
