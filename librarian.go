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
	RepoRoot   = "/content"
	SpacesRoot = "/spaces"
	ConfigFile = "/content/permissions.yaml"
)

func main() {
	fmt.Println("📚 Librarian (Go): Daemon Started")
	rebuild()

	watcher, err := fsnotify.NewWatcher()
	if err != nil { log.Fatal(err) }
	defer watcher.Close()

	if err := watcher.Add(RepoRoot); err != nil { log.Fatal(err) }

	fmt.Println("👀 Watching for changes...")
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok { return }
				if filepath.Base(event.Name) == "permissions.yaml" {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
						fmt.Println("⚡ Config change detected.")
						time.Sleep(100 * time.Millisecond)
						rebuild()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok { return }
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

		// 1. Ensure Directory Exists (Do NOT delete it)
		if err := os.MkdirAll(spaceDir, 0755); err != nil {
			fmt.Printf("   ! Failed to create dir: %v\n", err)
			continue
		}

		// 2. Wipe CONTENTS only (The Fix)
		files, err := ioutil.ReadDir(spaceDir)
		if err == nil {
			for _, f := range files {
				os.RemoveAll(filepath.Join(spaceDir, f.Name()))
			}
		}

		// 3. Link Paths
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
	if err != nil { return }
	for _, f := range files {
		name := f.Name()
		if name == ".git" || name == "permissions.yaml" { continue }
		linkFile(name, destDir)
	}
}

func linkFile(relPath, spaceDir string) {
	// Standard relative symlink
	target := filepath.Join("../../content", relPath)
	linkPath := filepath.Join(spaceDir, filepath.Base(relPath))
	
	// Try to link, ignore error if it fails (e.g. exists)
	os.Symlink(target, linkPath)
}