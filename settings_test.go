package main

import (
	"sync"
	"testing"
)

// TestSettingsConcurrentAccess exercises the mu lock discipline directly: many
// goroutines writing and marshalling settings at once, mirroring how Wails
// dispatches each bound call on its own net/http goroutine. The existing
// functional tests are single-goroutine, so `go test -race` never observes the
// write-vs-marshal path this guards. Run with -race for it to mean anything.
func TestSettingsConcurrentAccess(t *testing.T) {
	b := newTestBooking(t)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		scheme := "light"
		if i%2 == 0 {
			scheme = "dark"
		}
		go func() { defer wg.Done(); b.SetColorScheme(scheme) }() // locked mutate + persist (marshal to disk)
		go func() { defer wg.Done(); _ = b.GetSettings() }()      // locked whole-struct read
		go func() { defer wg.Done(); _ = b.GetServicePrices() }() // locked map read
	}
	wg.Wait()
}
