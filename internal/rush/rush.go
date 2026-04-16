package rush

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

// Execute is the primary entrypoint for the Rush CLI.
// It automatically detects the current operating context:
// It branches into the background daemon, the pre-fetcher engine,
// or the primary build orchestrator.
func Execute() {
	// Hand-off logic for Background Daemon
	if len(os.Args) >= 3 && os.Args[1] == "daemon" {
		logFile, _ := os.OpenFile("daemon.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		log.SetOutput(logFile)
		log.Printf("[Daemon] Task Init: %v\n", os.Args[2:])

		SetupS3()

		if os.Args[2] == "warm" && len(os.Args) >= 4 {
			hash := os.Args[3]
			log.Printf("[Daemon] Warming hash: %s\n", hash)
			file, checksum, err := DownloadFromS3(hash + ".tar.zst")
			if err != nil {
				log.Printf("[Daemon] Warm failed (S3): %v\n", err)
				return
			}
			err = decompress(hash, file.Body, checksum)
			file.Body.Close()
			if err != nil {
				log.Printf("[Daemon] Warm failed (Decompress): %v\n", err)
			} else {
				log.Printf("[Daemon] Warm success: %s\n", hash)
			}
			return
		}

		// If a second argument is provided, do a one-shot upload first
		if os.Args[2] != "serve" {
			wg.Add(1)
			_ = StoreinS3(os.Args[2])
		}

		// Transition to persistent Peer Server
		ServePeerCache()
		return
	}

	// Logic for Predictive Prefetching Git Hooks
	if len(os.Args) >= 2 && os.Args[1] == "prefetch" {
		Prefetch()
		return
	}

	// Logic for explicitly running the P2P Cache Server
	if len(os.Args) >= 2 && os.Args[1] == "serve" {
		ServePeerCache()
		return
	}

	// Zero-Config Root Discovery (Finds the nearest package.json)
	root, err := FindProjectRoot()
	if err != nil {
		fmt.Printf("\033[31m%v\033[0m\n", err)
		return
	}
	os.Chdir(root)

	// Load Cloud Environment settings
	SetupS3()

	const logo = `
 ____  _   _ ____  _   _
|  _ \| | | / ___|| | | |
| |_) | | | \___ \| |_| |
|  _ <| |_| |___) |  _  |
|_| \_\\___/|____/|_| |_|`

	fmt.Println("\033[1;93m" + logo + "\033[0m")
	startTime := time.Now()
	fmt.Println("\nSearching for project identity...")

	_, err = GetProjectIdentity()
	if err != nil {
		fmt.Println(err)
		return
	}

	totalTime := time.Since(startTime)
	fmt.Printf("Total execution time: %v\n", totalTime)

	// Comparative Performance Stats calculation
	baseline := getBaseline()
	if baseline > 0 && totalTime < baseline {
		saved := baseline - totalTime
		percent := (float64(saved) / float64(baseline)) * 100
		fmt.Printf("\033[1;32m Rush saved you %v (%.1f%% faster)\033[0m\n", saved.Round(time.Millisecond), percent)
	}

	wg.Wait()
}

// GetProjectIdentity orchestrates the cascading cache restoration logic.
// It generates the deterministic hash, checks the L1 SSD Vault, actively queries
// LAN peers via mDNS, coordinates with global team members via S3 metadata locks,
// and finally falls back to native JS installations if the cache is entirely cold.
func GetProjectIdentity() (string, error) {
	nodeVer := getNodeVersion()
	foundAny, tool, hashHex := generateIdentityHash(nodeVer)
	var downloaded bool = false

	if !foundAny {
		return "", fmt.Errorf("no lock files found")
	}
	stringhex := runtime.GOOS + "-" + hashHex

	// L1 Cache Check
	if checkhash(stringhex) == false {
		fmt.Println("L1 Cache Miss")

		os.MkdirAll(".rush-cache", os.ModePerm)
		lockPath := filepath.Join(".rush-cache", stringhex+".lock")
		lock := flock.New(lockPath)

		fmt.Printf("Waiting for build lock for %s...\n", stringhex)
		if err := lock.Lock(); err != nil {
			return stringhex, fmt.Errorf("failed to acquire build lock: %w", err)
		}
		defer func() {
			lock.Unlock()
			os.Remove(lockPath) // fails gracefully on Windows if locked
		}()

		// Double-checked locking
		if checkhash(stringhex) {
			fmt.Println("Another process finished the build. Pre-populated cache found.")
			if Checknode() == false {
				CopyfromVault(stringhex)
			}
			return stringhex, nil
		}

		// S3 Metadata Coordination (Preventing Duplicate Install Stampedes)
		for {
			status, err := CheckS3Status(stringhex)
			if err == nil && status != nil && status.State == "building" {
				fmt.Printf("Team member is currently building this hash on another machine. Waiting...\n")
				time.Sleep(5 * time.Second)
				continue
			}
			break
		}

		// 1. Try LAN Peer
		p2pStream, p2pChecksum, err := FindPeerCache(stringhex)
		if err == nil {
			fmt.Println(" LAN Peer Hit! Pulling...")
			downloaded = true
			decompressStart := time.Now()
			err = decompress(stringhex, p2pStream, p2pChecksum)
			p2pStream.Close()
			if err == nil {
				fmt.Printf(" Peer Restore completed in: %v\n", time.Since(decompressStart))
			} else {
				fmt.Printf(" Peer checksum failed: %v\n", err)
				downloaded = false
			}
		} else {
			fmt.Println("LAN Cache Miss")
		}

		// 2. Try S3 Cloud
		if !downloaded {
			downloadStart := time.Now()
			file, expectedChecksum, err := DownloadFromS3(stringhex + ".tar.zst")
			if err != nil {
				fmt.Println("L2 Cache Miss")
			} else {
				downloaded = true
				fmt.Printf(" Cloud Download: %v\n", time.Since(downloadStart))
				defer file.Body.Close()

				decompressStart := time.Now()
				fmt.Println("Decompressing & Verifying...")
				err = decompress(stringhex, file.Body, expectedChecksum)
				if err != nil {
					fmt.Printf(" %v\n", err)
					downloaded = false
				} else {
					fmt.Printf(" Local Decompression & Verify: %v\n", time.Since(decompressStart))
				}
			}
		}

		// 3. Fallback: Native Native Install
		if downloaded == false {
			// Signal intention to build to L2 cache to save team members' time
			_ = UpdateS3Status(stringhex, "building")

			fmt.Println("Running install tool")
			err = RunInstall(tool)
			if err != nil {
				_ = UpdateS3Status(stringhex, "failed")
				return stringhex, err
			}
			err = StoreInVault(stringhex)
			if err != nil {
				_ = UpdateS3Status(stringhex, "failed")
				return stringhex, err
			}

			fmt.Println("Handing off S3 upload to background daemon...")
			cmd := exec.Command(os.Args[0], "daemon", stringhex)
			if runtime.GOOS == "windows" {
				cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
			}

			err = cmd.Start()
			if err != nil {
				fmt.Printf("Failed to start daemon: %v\n", err)
				_ = UpdateS3Status(stringhex, "failed")
			}
		}

	}

	if Checknode() == false {
		CopyfromVault(stringhex)
	}
	return stringhex, nil
}
