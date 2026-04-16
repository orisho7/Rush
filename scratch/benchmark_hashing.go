package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run benchmark_hashing.go <path-to-project-root>")
		return
	}
	root := os.Args[1]
	
	filenames := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
	
	start := time.Now()
	
	h := sha256.New()
	nodeVersion := "v18.16.0" // Mock node version for benchmark consistency
	
	fmt.Fprintf(h, "%s:%s:%s:", runtime.GOOS, runtime.GOARCH, nodeVersion)
	
	var foundAnyLock bool
	for _, filename := range filenames {
		f, err := os.Open(root + "/" + filename)
		if err != nil {
			continue
		}
		foundAnyLock = true
		_, _ = io.Copy(h, f)
		f.Close()
	}
	
	if !foundAnyLock {
		fmt.Println("No lock files found in", root)
		return
	}
	
	hash := hex.EncodeToString(h.Sum(nil))
	duration := time.Since(start)
	
	fmt.Printf("Hash: %s\n", hash)
	fmt.Printf("Duration: %v\n", duration)
}
