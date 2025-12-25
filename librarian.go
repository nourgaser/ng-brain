package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	RepoRoot   = "/content"
	SpacesRoot = "/spaces"
	ConfigFile = "/content/permissions.yaml"
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

	// Watch the Content Folder
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
				// Detect changes to permissions.yaml
				if filepath.Base(event.Name) == "permissions.yaml" {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
						fmt.Println("⚡ Config change detected. Rebuilding...")
						time.Sleep(100 * time.Millisecond) // Debounce
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

		// Safety Check: Ensure we are only deleting inside /spaces
		// This prevents config errors from wiping your hard drive
		if !strings.HasPrefix(spaceDir, "/spaces") {
			fmt.Printf("   ! Safety Block: Refusing to wipe %s\n", spaceDir)
			continue
		}

		// 1. Wipe Directory (Clean Slate)
		os.RemoveAll(spaceDir)
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
		name := f.Name()
		if name == ".git" || name == "permissions.yaml" {
			continue
		}
		linkFile(name, destDir)
	}
}

func linkFile(relPath, spaceDir string) {
	// Inside the container, Content is mapped to /content
	// Spaces are mapped to /spaces/xyz
	// So the relative link from /spaces/xyz/file -> /content/file is:
	target := filepath.Join("../../content", relPath)
	linkPath := filepath.Join(spaceDir, filepath.Base(relPath))

	if err := os.Symlink(target, linkPath); err != nil {
		fmt.Printf("   ! Link failed for %s: %v\n", relPath, err)
	}
}