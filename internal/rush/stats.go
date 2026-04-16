package rush

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// recordBaseline stores the time taken for a full, cold build execution natively via package
// manager. It commits this duration to the disk to provide relative analytics on subsequent runs.
func recordBaseline(d time.Duration) {
	os.MkdirAll(".rush-cache", os.ModePerm)
	_ = os.WriteFile(filepath.Join(".rush-cache", "baseline.ms"), []byte(fmt.Sprintf("%d", d.Milliseconds())), 0644)
}

// getBaseline fetches the previously recorded baseline from the cache file format.
// If missing, returns 0.
func getBaseline() time.Duration {
	data, err := os.ReadFile(filepath.Join(".rush-cache", "baseline.ms"))
	if err != nil {
		return 0
	}
	ms, _ := strconv.ParseInt(string(data), 10, 64)
	return time.Duration(ms) * time.Millisecond
}
