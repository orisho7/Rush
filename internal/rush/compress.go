package rush

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/klauspost/compress/zstd"
)

// compress pipes a tar stream of the fully built L1 cache directory
// into a highly concurrent zstd stream. It runs the sha256 hashing inline
// via io.MultiWriter to avoid needing a secondary read pass.
func compress(hash string) (string, error) {
	fmt.Println("Compressing...")
	hasher := sha256.New()
	archivePath := filepath.Join(".rush-cache", hash+".tar.zst")
	outFile, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Create an io.MultiWriter to write to both the file and the hasher
	mw := io.MultiWriter(outFile, hasher)

	// Create fast zstd writer
	zw, err := zstd.NewWriter(mw,
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(runtime.NumCPU()),
	)
	if err != nil {
		return "", err
	}

	root := filepath.Join(".rush-cache", hash)

	// Use system tar.exe for scanning (much faster on Windows)
	// -c: create, -f -: to stdout, -C: from this directory
	cmd := exec.Command("tar", "-C", root, "-cf", "-", ".")
	cmd.Stdout = zw
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		zw.Close()
		return "", err
	}

	// CRITICAL: Close the zstd writer to flush final buffers BEFORE hashing
	zw.Close()

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// decompress utilizes a decoupled producer-consumer pipeline via io.Pipe
// to stream decompression from S3/P2P directly into untar via a 1MB buffer.
// It verifies the SHA-256 signature inline before atomic cache swapping.
func decompress(hash string, r io.Reader, expectedChecksum string) error {
	tempRoot := filepath.Join(".rush-cache", hash+".tmp")
	finalRoot := filepath.Join(".rush-cache", hash)

	// Ensure clean staging area
	os.RemoveAll(tempRoot)
	os.MkdirAll(tempRoot, os.ModePerm)

	// Create an asynchronous pipe to decouple decompression from extraction
	pr, pw := io.Pipe()

	// 1. Producer: Decompress the S3/Peer stream, hash it, and feed the pipe
	go func() {
		hasher := sha256.New()
		tr := io.TeeReader(r, hasher)

		reader, err := zstd.NewReader(tr, zstd.WithDecoderConcurrency(runtime.NumCPU()))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		defer reader.Close()

		// Use a large 1MB buffer for high-speed I/O
		buf := make([]byte, 1024*1024)
		_, err = io.CopyBuffer(pw, reader, buf)

		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if err == nil && expectedChecksum != "" && actualChecksum != expectedChecksum {
			err = fmt.Errorf("CHECKSUM MISMATCH: expected %s, got %s", expectedChecksum, actualChecksum)
		}

		pw.CloseWithError(err)
	}()

	// 2. Consumer: Feed the pipe into system tar.exe
	cmd := exec.Command("tar", "-C", tempRoot, "-xf", "-")
	cmd.Stdin = pr
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		os.RemoveAll(tempRoot)
		return err
	}

	// Atomic Swap: Stage-then-Swap to finalize
	var renameErr error
	for i := 0; i < 5; i++ {
		os.RemoveAll(finalRoot)
		renameErr = os.Rename(tempRoot, finalRoot)
		if renameErr == nil {
			break
		}
		time.Sleep(200 * time.Millisecond) // Give Windows a moment to release file handles
	}

	if renameErr != nil {
		return fmt.Errorf("failed to finalize cache: %w", renameErr)
	}

	return nil
}
