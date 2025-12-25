package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Updated Config Struct with Password
type Config struct {
	Spaces map[string]struct {
		Password string   `yaml:"password"` // <--- Captured here
		Paths    []string `yaml:"paths"`
	} `yaml:"spaces"`
}

const (
	RepoRoot       = "/content"
	SpacesRoot     = "/spaces"
	ConfigFile     = "/content/permissions.yaml"
	NginxConfigDir = "/etc/nginx/conf.d"
)

var HostRootDir = os.Getenv("HOST_ROOT_DIR")

func main() {
	if HostRootDir == "" {
		log.Fatal("❌ HOST_ROOT_DIR env var is missing!")
	}
	fmt.Println("📚 Librarian Orchestrator: Started")
	rebuild()

	watcher, err := fsnotify.NewWatcher()
	if err != nil { log.Fatal(err) }
	defer watcher.Close()
	if err := watcher.Add(RepoRoot); err != nil { log.Fatal(err) }

	fmt.Println("👀 Watching permissions.yaml...")
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok { return }
				if filepath.Base(event.Name) == "permissions.yaml" {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
						fmt.Println("⚡ Config change detected.")
						time.Sleep(500 * time.Millisecond)
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

	syncFiles(config)
	orchestrate(config)
}

func syncFiles(config Config) {
	// 1. Ensure Shared Storage Exists in CONTENT (The Source of Truth)
	sharedPlugDir := filepath.Join(RepoRoot, "_plug")
	sharedLibDir := filepath.Join(RepoRoot, "Library")
	
	// Create them in Content if they don't exist
	os.MkdirAll(sharedPlugDir, 0777)
	os.MkdirAll(sharedLibDir, 0777)
	os.Chmod(sharedPlugDir, 0777)
	os.Chmod(sharedLibDir, 0777)

	for spaceName, rules := range config.Spaces {
		spaceDir := filepath.Join(SpacesRoot, spaceName)
		
		os.MkdirAll(spaceDir, 0755)
		os.Chmod(spaceDir, 0777)

		// 2. Wipe Space (Clean Slate)
		files, _ := ioutil.ReadDir(spaceDir)
		for _, f := range files {
			os.RemoveAll(filepath.Join(spaceDir, f.Name()))
		}

		// 3. FORCE SYSTEM LINKS (The Fix)
		// We link the FOLDERS, not the contents.
		linkFile("_plug", spaceDir)
		linkFile("Library", spaceDir)
		
		// 4. User Links (from permissions.yaml)
		for _, relPath := range rules.Paths {
			if relPath == "/" {
				linkAllFiles(RepoRoot, spaceDir)
				continue
			}
			linkFile(relPath, spaceDir)
		}
	}
}

func orchestrate(config Config) {
	validUsers := make(map[string]bool)

	for spaceName, details := range config.Spaces {
		if spaceName == "public" || spaceName == "writer" { continue }
		
		validUsers[spaceName] = true
		fmt.Printf("⚙️  Orchestrating User: %s\n", spaceName)
		
		// Pass the password to the launcher
		ensureContainer(spaceName, details.Password)
		generateNginxConfig(spaceName)
	}

	// Reaper Logic
	files, _ := ioutil.ReadDir(NginxConfigDir)
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".conf") {
			user := strings.TrimSuffix(name, ".conf")
			if !validUsers[user] {
				fmt.Printf("💀 Reaping Orphan: %s\n", user)
				os.Remove(filepath.Join(NginxConfigDir, name))
				exec.Command("docker", "rm", "-f", fmt.Sprintf("ng-space-%s", user)).Run()
			}
		}
	}

	fmt.Println("🔄 Reloading Nginx...")
	cmd := exec.Command("docker", "kill", "-s", "HUP", "ng-gatekeeper")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ Reload Failed: %s\n", string(out))
	} else {
		fmt.Println("✅ Nginx Reloaded.")
	}
}

func ensureContainer(user string, password string) {
	containerName := fmt.Sprintf("ng-space-%s", user)

	// STRATEGY CHANGE: Always remove and recreate.
	// This ensures that password changes and mount updates (permissions.yaml)
	// are always applied immediately.
	// It causes a 1-2s downtime for the user on config change, which is acceptable.
	exec.Command("docker", "rm", "-f", containerName).Run()

	fmt.Printf("🚀 Spawning Container: %s\n", containerName)

	spaceVol := fmt.Sprintf("%s/spaces/%s:/space", HostRootDir, user)
	contentVol := fmt.Sprintf("%s/content:/content", HostRootDir)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "always",
		"--network", "ng-brain_default",
		"--user", "1001:1001",
		"-v", spaceVol,
		"-v", contentVol,
	}

	// Inject Password if provided
	if password != "" {
		// Format: username:password
		authEnv := fmt.Sprintf("SB_USER=%s:%s", user, password)
		args = append(args, "-e", authEnv)
	}

	args = append(args, "ghcr.io/silverbulletmd/silverbullet")

	cmd := exec.Command("docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ Failed to spawn %s: %s\n", user, string(out))
	}
}

func generateNginxConfig(user string) {
	domain := fmt.Sprintf("%s.docs2.nourgaser.com", user)
	container := fmt.Sprintf("ng-space-%s", user)

	configContent := fmt.Sprintf(`
server {
    listen 80;
    server_name %s;
    location / {
        proxy_pass http://%s:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }
}
`, domain, container)

	ioutil.WriteFile(filepath.Join(NginxConfigDir, user+".conf"), []byte(configContent), 0644)
}

func linkAllFiles(srcDir, destDir string) {
	files, err := ioutil.ReadDir(srcDir)
	if err != nil { return }
	
	// Check if this is the Writer space (safer check)
	isWriter := filepath.Base(destDir) == "writer"

	for _, f := range files {
		name := f.Name()
		
		// Skip config
		if name == "permissions.yaml" { continue }

		// Handle .git explicitly
		if name == ".git" {
			if isWriter {
				// Force link .git for writer
				linkFile(name, destDir)
				fmt.Println("   🔗 Linked .git repository to writer")
			}
			continue
		}

		linkFile(name, destDir)
	}
}

func linkFile(relPath, spaceDir string) {
	target := filepath.Join("../../content", relPath)
	linkPath := filepath.Join(spaceDir, filepath.Base(relPath))
	os.Symlink(target, linkPath)
}