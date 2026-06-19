package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PathScanner caches PATH binaries for fast batch detection
type PathScanner struct {
	binaries map[string]string // binary name -> full path
}

func NewPathScanner() *PathScanner {
	s := &PathScanner{binaries: make(map[string]string)}
	s.scan()
	return s
}

func (s *PathScanner) scan() {
	pathEnv := os.Getenv("PATH")
	dirs := filepath.SplitList(pathEnv)
	seen := make(map[string]bool)

	for _, dir := range dirs {
		if seen[dir] {
			continue
		}
		seen[dir] = true

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if !e.IsDir() && (info.Mode()&0111) != 0 {
				name := e.Name()
				if _, exists := s.binaries[name]; !exists {
					s.binaries[name] = filepath.Join(dir, name)
				}
			}
		}
	}
}

func (s *PathScanner) Find(name string) (string, bool) {
	path, ok := s.binaries[name]
	return path, ok
}

var defaultScanner *PathScanner

func getScanner() *PathScanner {
	if defaultScanner == nil {
		defaultScanner = NewPathScanner()
	}
	return defaultScanner
}

// DetectTool checks if a tool is installed on the system (with version check)
func DetectTool(tool ToolDefinition) InstallStatus {
	return detectToolWithScanner(tool, getScanner(), true)
}

func detectToolWithScanner(tool ToolDefinition, scanner *PathScanner, checkVersion bool) InstallStatus {
	status := InstallStatus{
		ToolName: tool.Name,
	}

	// 1. Check PATH for the tool binary
	binaryName := guessBinaryName(tool)
	if binaryName != "" {
		if path, ok := scanner.Find(binaryName); ok {
			status.OnPath = true
			status.PathLocation = path
			if checkVersion {
				status.Version = getVersion(binaryName)
			}
		}
	}

	// 2. Check install methods
	if !status.OnPath && len(tool.Install) > 0 {
		for _, inst := range tool.Install {
			switch inst.Method {
			case "docker":
				if checkDockerImage(inst.PackageName) {
					status.DockerImage = true
				}
			case "apt", "brew", "pip", "go", "cargo", "npm", "gem":
				if pkg, ver := checkPackageManager(inst); pkg != "" {
					status.PackageManager = pkg
					if checkVersion && ver != "" {
						status.Version = ver
					}
				}
			}
			if status.DockerImage || status.PackageManager != "" {
				break
			}
		}
	}

	return status
}

// DetectAllTools checks all tools in one batch (skips subprocesses for speed)
func DetectAllTools(tools []ToolDefinition) map[string]InstallStatus {
	scanner := NewPathScanner()
	results := make(map[string]InstallStatus)
	for _, t := range tools {
		results[t.ID] = detectToolWithScanner(t, scanner, false)
	}
	return results
}

func guessBinaryName(tool ToolDefinition) string {
	// Try the tool name first
	if tool.Name != "" {
		name := tool.Name
		// Handle spaces in names
		if idx := strings.Index(name, " "); idx > 0 {
			name = name[:idx]
		}
		if _, err := exec.LookPath(strings.ToLower(name)); err == nil {
			return strings.ToLower(name)
		}
	}

	// Try the ID as the binary name (with dashes and without)
	candidate := strings.ReplaceAll(tool.ID, "-", "")
	if _, err := exec.LookPath(candidate); err == nil {
		return candidate
	}

	// Try package name from install methods as binary name
	if len(tool.Install) > 0 {
		for _, inst := range tool.Install {
			if inst.PackageName != "" {
				if _, err := exec.LookPath(inst.PackageName); err == nil {
					return inst.PackageName
				}
			}
		}
	}

	return ""
}

func getVersion(binary string) string {
	// Try --version, -v, version subcommand
	for _, arg := range []string{"--version", "-v", "version"} {
		cmd := exec.Command(binary, arg)
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 0 {
				line := strings.TrimSpace(lines[0])
				return line
			}
		}
	}
	return ""
}

func checkDockerImage(image string) bool {
	if image == "" {
		return false
	}
	cmd := exec.Command("docker", "images", "-q", image)
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func checkPackageManager(inst Install) (string, string) {
	if inst.PackageName == "" {
		return "", ""
	}

	switch inst.Method {
	case "apt":
		return checkAPT(inst.PackageName)
	case "brew":
		return checkBrew(inst.PackageName)
	case "pip":
		return checkPip(inst.PackageName)
	case "go":
		return checkGo(inst.PackageName)
	case "cargo":
		return checkCargo(inst.PackageName)
	case "npm":
		return checkNpm(inst.PackageName)
	case "gem":
		return checkGem(inst.PackageName)
	}
	return "", ""
}

func checkAPT(pkg string) (string, string) {
	if runtime.GOOS != "linux" {
		return "", ""
	}
	cmd := exec.Command("dpkg", "-s", pkg)
	if err := cmd.Run(); err == nil {
		out, _ := exec.Command("dpkg", "-s", pkg).Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Version:") {
				return "apt", strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			}
		}
		return "apt", ""
	}
	cmd = exec.Command("dpkg-query", "-W", "-f=${Version}", pkg)
	if out, err := cmd.Output(); err == nil {
		return "apt", strings.TrimSpace(string(out))
	}
	return "", ""
}

func checkBrew(pkg string) (string, string) {
	if runtime.GOOS != "darwin" {
		return "", ""
	}
	cmd := exec.Command("brew", "list", pkg)
	if err := cmd.Run(); err == nil {
		out, _ := exec.Command("brew", "info", pkg).Output()
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			return "brew", strings.TrimSpace(lines[0])
		}
		return "brew", ""
	}
	return "", ""
}

func checkPip(pkg string) (string, string) {
	cmd := exec.Command("pip", "show", pkg)
	out, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Version:") {
				return "pip", strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			}
		}
		return "pip", ""
	}
	return "", ""
}

func checkGo(pkg string) (string, string) {
	if _, err := exec.LookPath("go"); err != nil {
		return "", ""
	}
	cmd := exec.Command("go", "version", "-m", pkg)
	if err := cmd.Run(); err == nil {
		return "go", ""
	}
	return "", ""
}

func checkCargo(pkg string) (string, string) {
	cmd := exec.Command("cargo", "install", "--list")
	out, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, pkg) {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) == 2 {
					return "cargo", strings.TrimSpace(parts[1])
				}
				return "cargo", strings.TrimSpace(line)
			}
		}
	}
	return "", ""
}

func checkNpm(pkg string) (string, string) {
	cmd := exec.Command("npm", "list", "-g", pkg)
	if err := cmd.Run(); err == nil {
		return "npm", ""
	}
	return "", ""
}

func checkGem(pkg string) (string, string) {
	cmd := exec.Command("gem", "list", pkg)
	out, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, pkg) {
				return "gem", strings.TrimSpace(line)
			}
		}
	}
	return "", ""
}
