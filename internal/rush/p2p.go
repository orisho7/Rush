package rush

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/klauspost/compress/zstd"
)

// ServePeerCache binds to an available TCP port and advertises this local machine
// via mDNS (zeroconf). It provides a simple HTTP server to allow peers on the
// same Local Area Network to pull compiled caches at gigabit speeds without hitting S3.
func ServePeerCache() {
	// Attempt to find a free port starting at 1997
	port := 1997
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		listener, err = net.Listen("tcp", ":0") // fallback to OS-assigned
		if err != nil {
			return
		}
		port = listener.Addr().(*net.TCPAddr).Port
	}

	fmt.Printf("[P2P] Server starting on port %d...\n", port)

	// Register with mDNS
	server, _ := zeroconf.Register("Rush-Node-"+runtime.GOOS, "_rush._tcp", "local.", port, []string{"ver=1"}, nil)
	defer server.Shutdown()

	http.HandleFunc("/p2p/", func(w http.ResponseWriter, r *http.Request) {
		hash := strings.TrimPrefix(r.URL.Path, "/p2p/")
		path := filepath.Join(".rush-cache", hash)

		if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
			http.Error(w, "Not found", 404)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")

		// Create a zstd stream writing directly to the HTTP response
		zw, _ := zstd.NewWriter(w,
			zstd.WithEncoderLevel(zstd.SpeedFastest),
			zstd.WithEncoderConcurrency(runtime.NumCPU()),
		)
		defer zw.Close()

		// Stream the uncompressed cache folder directly into the zstd network pipe
		cmd := exec.Command("tar", "-C", path, "-cf", "-", ".")
		cmd.Stdout = zw
		cmd.Stderr = os.Stderr

		_ = cmd.Run()
	})

	_ = http.Serve(listener, nil)
}

// FindPeerCache performs an ultra-fast mDNS scan for other Rush daemons on the LAN.
// If a peer is found advertising the requested hash, it returns a 200 stream, saving time.
func FindPeerCache(hash string) (io.ReadCloser, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, "", err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		_ = resolver.Browse(ctx, "_rush._tcp", "local.", entries)
	}()

	for {
		select {
		case entry := <-entries:
			if entry == nil {
				continue
			}
			for _, ip := range entry.AddrIPv4 {
				url := fmt.Sprintf("http://%s:%d/p2p/%s", ip, entry.Port, hash)
				resp, err := http.Get(url)
				if err == nil && resp.StatusCode == 200 {
					return resp.Body, "", nil // Found a hit
				}
			}
		case <-ctx.Done():
			return nil, "", fmt.Errorf("searched LAN but no peer has match")
		}
	}
}
