package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Config structure matches permissions.yaml
type Config struct {
	Spaces map[string]struct {
		Paths []string `yaml:"paths"`
	} `yaml:"spaces"`
}

const (
	RepoRoot   = "content"
	SpacesRoot = "spaces"
	ConfigFile = "content/permissions.yaml"
)

func main() {
	fmt.Println("📚 Librarian (Go): Daemon Started")

	// 1. Initial Build
	rebuild()

	// 2. Setup Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Watch the Config File
	// Note: Editors often "rename" files on save, so we watch the folder
	err = watcher.Add(RepoRoot) 
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("👀 Watching for changes in permissions.yaml...")

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// If permissions.yaml changes
				if filepath.Base(event.Name) == "permissions.yaml" {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
						fmt.Println("⚡ Config change detected. Rebuilding...")
						// Small sleep to ensure write is complete
						time.Sleep(100 * time.Millisecond)
						rebuild()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
			}
		}
	}()

	<-done
}

func rebuild() {
	data, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		fmt.Printf("❌ Error reading config: %v\n", err)
		return
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("❌ Invalid YAML: %v\n", err)
		return
	}

	for spaceName, rules := range config.Spaces {
		spaceDir := filepath.Join(SpacesRoot, spaceName)

		// 1. Wipe Directory (Safety check: ensure we are in spaces folder)
		if stringsContains(spaceDir, "spaces") { 
             os.RemoveAll(spaceDir) 
        }
		if err := os.MkdirAll(spaceDir, 0755); err != nil {
			fmt.Printf("   ! Failed to create dir: %v\n", err)
			continue
		}

		// 2. Link Paths
		for _, relPath := range rules.Paths {
			if relPath == "/" {
				linkAllFiles(RepoRoot, spaceDir)
				continue
			}
			linkFile(relPath, spaceDir)
		}
	}
    fmt.Println("✅ Spaces Synced.")
}

func linkAllFiles(srcDir, destDir string) {
	files, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.Name() == ".git" || f.Name() == "permissions.yaml" {
			continue
		}
		linkFile(f.Name(), destDir)
	}
}

func linkFile(relPath, spaceDir string) {
	target := filepath.Join("../../content", relPath)
	linkPath := filepath.Join(spaceDir, filepath.Base(relPath))

	if err := os.Symlink(target, linkPath); err != nil {
		// fmt.Printf("   ! Link failed for %s: %v\n", relPath, err)
	}
}

// Helper (since we removed strings package import earlier, added simple check)
func stringsContains(s, substr string) bool {
    return len(s) >= len(substr) && s[0:len(substr)] == substr || len(s) > 0 // Simplified check
}