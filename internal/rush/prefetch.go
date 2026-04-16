package rush

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Prefetch implements a "Time Engine" that reads Git branch histories locally
// without actively running git checkout. It forecasts the identity hash for inactive branches
// and spins up the daemon silently to download the dependencies so they are warmed by 
// the time you finally check out.
func Prefetch() {
	out, _ := exec.Command("git", "branch", "--format=%(refname:short)").Output()
	branches := strings.Split(strings.TrimSpace(string(out)), "\n")

	nodeVer := getNodeVersion()
	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		// Peek at the lockfile on other branches
		content, err := exec.Command("git", "show", b+":package-lock.json").Output()
		if err != nil {
			continue
		}

		// Forecast the hash
		h := sha256.New()
		fmt.Fprintf(h, "%s:%s:%s:", runtime.GOOS, runtime.GOARCH, nodeVer)
		h.Write(content)
		targetHash := runtime.GOOS + "-" + hex.EncodeToString(h.Sum(nil))

		if !checkhash(targetHash) {
			fmt.Printf(" Prefetching branch '%s' (%s)...\n", b, targetHash)
			cmd := exec.Command(os.Args[0], "daemon", "warm", targetHash)
		
			_ = cmd.Start()
		}
	}
}
