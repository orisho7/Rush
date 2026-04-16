package rush

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// RunInstall executes the detected JS package manager natively as a fallback
// when all cache layers (L1, P2P, S3) are utterly missed.
func RunInstall(tool string) error {
	start := time.Now()
	fmt.Printf("Running %s install...\n", tool)

	cmd := exec.Command(tool, "install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	duration := time.Since(start)
	fmt.Printf(" %s install completed in: %v\n", tool, duration)

	if err == nil {
		fmt.Printf("\033[1;36m[Stats] Performance baseline established: %v\033[0m\n", duration)
		recordBaseline(duration)
	}
	return err
}

// Checknode checks if the node_modules directory currently exists in the 
// local directory structure representing the active project.
func Checknode() bool {
	_, err := os.Stat("node_modules")
	if err != nil {
		fmt.Println("node_modules does not exist, Creating...")
		return false

	}
	return true
}
