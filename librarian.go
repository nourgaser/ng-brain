package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Spaces map[string]struct {
		Paths []string `yaml:"paths"`
	} `yaml:"spaces"`
}

const (
	RepoRoot       = "/content"
	SpacesRoot     = "/spaces"
	ConfigFile     = "/content/permissions.yaml"
	NginxConfigDir = "/etc/nginx/conf.d" // Shared volume with Nginx
)

// We need the HOST path to tell Docker where to mount volumes from
var HostRootDir = os.Getenv("HOST_ROOT_DIR") 

func main() {
	if HostRootDir == "" {
		log.Fatal("❌ HOST_ROOT_DIR env var is missing! Cannot manage containers.")
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
						time.Sleep(500 * time.Millisecond) // Longer debounce for docker ops
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

	// 1. Sync Files (The original job)
	syncFiles(config)

	// 2. Orchestrate Containers & Nginx
	orchestrate(config)
}

func syncFiles(config Config) {
	for spaceName, rules := range config.Spaces {
		spaceDir := filepath.Join(SpacesRoot, spaceName)
		os.MkdirAll(spaceDir, 0755)

		os.Chmod(spaceDir, 0777)

		// Wipe contents
		files, _ := ioutil.ReadDir(spaceDir)
		for _, f := range files {
			os.RemoveAll(filepath.Join(spaceDir, f.Name()))
		}

		// Link
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

	// 1. Ensure Valid Containers Exist
	for spaceName := range config.Spaces {
		if spaceName == "public" || spaceName == "writer" { continue }
		
		validUsers[spaceName] = true // Mark as valid
		fmt.Printf("⚙️  Orchestrating User: %s\n", spaceName)
		ensureContainer(spaceName)
		generateNginxConfig(spaceName)
	}

	// 2. Kill Orphans (The Reaper)
	// Find all config files in /etc/nginx/conf.d/
	files, _ := ioutil.ReadDir(NginxConfigDir)
	for _, f := range files {
		name := f.Name()
		// If it looks like a user config (e.g. alice.conf)
		if strings.HasSuffix(name, ".conf") {
			user := strings.TrimSuffix(name, ".conf")
			
			// If this user is NOT in our valid list, kill them.
			if !validUsers[user] {
				fmt.Printf("💀 Reaping Orphan: %s\n", user)
				
				// A. Remove Nginx Config
				os.Remove(filepath.Join(NginxConfigDir, name))
				
				// B. Kill Container
				containerName := fmt.Sprintf("ng-space-%s", user)
				exec.Command("docker", "rm", "-f", containerName).Run()
				
				// C. Wipe Space Folder (Optional, maybe keep for backup?)
				// os.RemoveAll(filepath.Join(SpacesRoot, user))
			}
		}
	}

	// 3. Reload Nginx (The Fix: Use SIGHUP + Error Logging)
	fmt.Println("🔄 Reloading Nginx...")
	
	// We use 'docker kill -s HUP' which sends the "Reload Config" signal directly to PID 1
	cmd := exec.Command("docker", "kill", "-s", "HUP", "ng-gatekeeper")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Failed to reload Nginx: %v | Output: %s\n", err, string(output))
	} else {
		fmt.Println("✅ Nginx Reloaded Successfully.")
	}
}

func ensureContainer(user string) {
	containerName := fmt.Sprintf("ng-space-%s", user)

	// Check if running
	check := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, _ := check.Output()
	if len(output) > 0 {
		return // Already running
	}

	// Check if stopped (exists but exited)
	checkStop := exec.Command("docker", "ps", "-aq", "-f", "name="+containerName)
	outStop, _ := checkStop.Output()
	if len(outStop) > 0 {
		// Remove it to restart fresh
		exec.Command("docker", "rm", containerName).Run()
	}

	fmt.Printf("🚀 Spawning Container: %s\n", containerName)
	
	// Determine Ports/Network
	// We rely on internal Docker DNS. We don't map ports to host.
	// We connect it to the SAME network as the main stack.
	
	// Construct the Docker Run command
	// Note: We use HOST_ROOT_DIR for volumes
	spaceVol := fmt.Sprintf("%s/spaces/%s:/space", HostRootDir, user)
	contentVol := fmt.Sprintf("%s/content:/content", HostRootDir)

	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"--restart", "always",
		"--network", "ng-brain_default", // MUST match your compose network name
		"--user", "1001:1001",           // Matches your PUID
		"-v", spaceVol,
		"-v", contentVol,
		"ghcr.io/silverbulletmd/silverbullet",
	)
	
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Failed to spawn %s: %s\n", user, string(out))
	}
}

func generateNginxConfig(user string) {
	// We assume wildcard DNS: user.docs2.nourgaser.com
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

	// Write to shared volume
	path := filepath.Join(NginxConfigDir, user+".conf")
	err := ioutil.WriteFile(path, []byte(configContent), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to write Nginx config for %s: %v\n", user, err)
	}
}

// Helpers... (linkFile, linkAllFiles same as before)
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
	target := filepath.Join("../../content", relPath)
	linkPath := filepath.Join(spaceDir, filepath.Base(relPath))
	os.Symlink(target, linkPath)
}