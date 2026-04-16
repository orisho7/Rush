package rush

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var wg sync.WaitGroup

var filenames []string = []string{
	"package.json",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
}

// getCurrentDir gets the application current working directory path securely.
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v\n", err)
	}
	return dir
}

// checkhash verifies if a directory with the respective identity hash exists 
// locally inside the permanent L1 `.rush-cache` folder. 
func checkhash(hash string) bool {
	_, err := os.Stat(filepath.Join(".rush-cache", hash))
	if err != nil {

		return false
	}
	return true
}

// FindProjectRoot navigates upwards through the application directory tree 
// until it locks onto a supported JS lock file layout, identifying the 
// root project folder. It returns the absolute path so that Rush can execute 
// securely regardless of nested CWD origin.
func FindProjectRoot() (string, error) {
	dir, _ := os.Getwd()
	for {
		for _, lock := range filenames {
			if _, err := os.Stat(filepath.Join(dir, lock)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("fatal: Could not find project root (no lockfiles found in parents)")
}
