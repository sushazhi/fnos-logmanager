// Package main implements a cross-platform build tool for fnOS LogManager.
//
// It replaces the legacy build.py with a single native binary compiled from Go:
//   - Cross-platform by design (os/exec, filepath, net/http, os.Chmod)
//   - No Python / bash dependency at build time
//   - Same feature set: Vue build, Go cross-compile (linux/amd64+arm64), fnpack pack
//
// It lives in its own module (build/go.mod) and only uses the standard library,
// so it stays completely decoupled from the server module.
//
// Usage (from the project root):
//
//	go run ./build -version 0.8.0
//
// Or compile once and reuse:
//
//	go build -o .local-build/buildtool ./build
//	.local-build/buildtool -version 0.8.0
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	fnpackBase = "https://static2.fnnas.com/fnpack/fnpack-1.2.3"
	fnpackVer  = "1.2.3"
)

var (
	projectDir string
	buildDir   string
	appVersion string
	versions   map[string]string
)

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}

func log(msg string) { fmt.Println(msg) }

// platform returns "os/arch" of the development machine.
func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// fnpackSuffix maps the dev machine to the official fnpack download suffix.
func fnpackSuffix() string {
	switch runtime.GOOS {
	case "windows":
		return "windows-amd64"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "darwin-arm64"
		}
		return "darwin-amd64"
	default:
		if runtime.GOARCH == "arm64" {
			return "linux-arm"
		}
		return "linux-amd64"
	}
}

// findProjectRoot walks up from cwd until a directory containing "manifest" is found.
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "manifest")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fatal(fmt.Errorf("找不到项目根目录（未发现 manifest），请从项目目录或其子目录运行"))
		}
		dir = parent
	}
}

// runCmd runs a command in dir, streaming stdout/stderr to the console.
// On Windows, Go's exec.LookPath already resolves npm.cmd etc. via PATHEXT.
func runCmd(dir, name string, args ...string) error {
	log("  Running: " + name + " " + strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("命令失败: %s: %v", name, err)
	}
	return nil
}

// --- version cache (.local-build/versions.json) ---

func loadVersions() {
	versions = map[string]string{}
	b, err := os.ReadFile(filepath.Join(buildDir, "versions.json"))
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &versions)
}

func saveVersion(key, val string) {
	versions[key] = val
	_ = os.MkdirAll(buildDir, 0o755)
	b, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(buildDir, "versions.json"), b, 0o644)
}

// --- download with cache ---

func download(url, outFile, description, component, version string, force bool) {
	if !force {
		if info, err := os.Stat(outFile); err == nil && info.Size() > 0 && versions[component] == version {
			log("  Using cached " + description + " (version " + version + ")")
			return
		}
	}
	log("  Downloading " + description + "...")
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fatal(err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal(fmt.Errorf("下载 %s 失败: %v", description, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fatal(fmt.Errorf("下载 %s 失败: HTTP %d", description, resp.StatusCode))
	}
	out, err := os.Create(outFile)
	if err != nil {
		fatal(err)
	}
	n, err := io.Copy(out, resp.Body)
	out.Close()
	if err != nil || n == 0 {
		fatal(fmt.Errorf("下载 %s 失败: %v", description, err))
	}
	saveVersion(component, version)
	log("  Downloaded " + description)
}

// --- file helpers ---

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// copyTree recursively copies a file or directory (preserving permissions).
func copyTree(src, dst string) {
	info, err := os.Stat(src)
	if err != nil {
		return
	}
	if info.IsDir() {
		_ = os.MkdirAll(dst, 0o755)
		entries, err := os.ReadDir(src)
		if err != nil {
			fatal(err)
		}
		for _, e := range entries {
			copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
		}
		return
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	in, err := os.Open(src)
	if err != nil {
		fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		fatal(err)
	}
	out.Close()
	_ = os.Chmod(dst, info.Mode().Perm())
}

// dirSnapshot returns a stable JSON snapshot of (relpath -> mtime) used for
// build caching. node_modules / dist are ignored.
func dirSnapshot(root string) string {
	snap := map[string]int64{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if info, err := d.Info(); err == nil {
			snap[rel] = info.ModTime().UnixNano()
		}
		return nil
	})
	b, _ := json.Marshal(snap)
	return string(b)
}

// --- manifest handling ---

func readManifestVersion() string {
	b, err := os.ReadFile(filepath.Join(projectDir, "manifest"))
	if err != nil {
		fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version") {
			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	fatal(fmt.Errorf("无法从 manifest 读取版本号"))
	return "" // unreachable, keeps the compiler happy
}

// updateManifest rewrites the version line in the staged manifest.
func updateManifest() {
	path := filepath.Join(buildDir, "manifest")
	b, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	re := regexp.MustCompile(`(?m)^version\s*=.*`)
	out := re.ReplaceAllString(string(b), "version = "+appVersion)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		fatal(err)
	}
}

// --- build steps ---

func setupBuildDir() {
	log("[1/5] Setting up build directory...")
	for _, d := range []string{"app/ui/assets", "app/ui/images", "app/server", "cmd", "config", "wizard"} {
		_ = os.RemoveAll(filepath.Join(buildDir, d))
	}
	for _, d := range []string{"app/server", "app/ui", "cmd", "config", "wizard"} {
		_ = os.MkdirAll(filepath.Join(buildDir, d), 0o755)
	}
	log("  Build directory ready")
}

func buildVue(skipVue, force, clean bool) {
	log("[2/5] Building Vue frontend...")
	uiDir := filepath.Join(projectDir, "app", "ui")
	uiDist := filepath.Join(uiDir, "dist")
	vueKey := "vue_src_" + appVersion
	cur := dirSnapshot(filepath.Join(uiDir, "src"))
	cached := !force && !clean && !skipVue &&
		versions[vueKey] == cur && fileExists(filepath.Join(uiDist, "index.html"))

	switch {
	case skipVue:
		log("  Skipping Vue build")
	case !fileExists(filepath.Join(uiDir, "package.json")):
		log("  Skipping Vue build (no package.json)")
	case cached:
		log("  Vue source unchanged, reusing cached dist")
	default:
		if !fileExists(filepath.Join(uiDir, "node_modules")) {
			log("  Installing npm dependencies...")
			if err := runCmd(uiDir, "npm", "install"); err != nil {
				fatal(err)
			}
		}
		log("  Building Vue app...")
		if err := runCmd(uiDir, "npm", "run", "build"); err != nil {
			fatal(fmt.Errorf("Vue build failed: %v", err))
		}
		if !fileExists(filepath.Join(uiDist, "index.html")) {
			fatal(fmt.Errorf("Vue build output not found"))
		}
		saveVersion(vueKey, cur)
		log("  Vue build complete")
	}
}

func copyProjectFiles() {
	log("[3/5] Copying project files...")
	for _, sub := range []string{"cmd", "config", "wizard"} {
		copyTree(filepath.Join(projectDir, sub), filepath.Join(buildDir, sub))
	}
	copyTree(filepath.Join(projectDir, "manifest"), filepath.Join(buildDir, "manifest"))
	updateManifest()
	for _, icon := range []string{"ICON.PNG", "ICON_256.PNG"} {
		copyTree(filepath.Join(projectDir, icon), filepath.Join(buildDir, icon))
	}

	uiDir := filepath.Join(projectDir, "app", "ui")
	uiDist := filepath.Join(uiDir, "dist")
	if fileExists(filepath.Join(uiDist, "index.html")) {
		copyTree(uiDist, filepath.Join(buildDir, "app", "ui"))
		for _, sub := range []string{"config", "images"} {
			copyTree(filepath.Join(uiDir, sub), filepath.Join(buildDir, "app", "ui", sub))
		}
		log("  Vue dist files copied")
	} else {
		for _, sub := range []string{"config", "images", "index.html"} {
			copyTree(filepath.Join(uiDir, sub), filepath.Join(buildDir, "app", "ui", sub))
		}
		log("  Static UI files copied")
	}
}

func buildServer() {
	log("[4/5] Building Go server binaries...")
	serverSrc := filepath.Join(projectDir, "app", "server")
	serverDir := filepath.Join(buildDir, "app", "server")
	_ = os.MkdirAll(serverDir, 0o755)

	archs := []struct{ goarch, label string }{
		{"amd64", "x86_64"},
		{"arm64", "ARM64"},
	}
	for _, a := range archs {
		binaryPath := filepath.Join(serverDir, "logmanager-server-"+a.goarch)
		log("  Cross-compiling for linux/" + a.goarch + " (" + a.label + ")...")
		cmd := exec.Command("go", "build", "-o", binaryPath, "-ldflags=-s -w", "./cmd/server")
		cmd.Dir = serverSrc
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+a.goarch, "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log("  ERROR: Go build for " + a.label + " failed")
			if len(out) > 0 {
				fmt.Print(string(out))
			}
			fatal(err)
		}
		if info, err := os.Stat(binaryPath); err == nil {
			log(fmt.Sprintf("  %s binary: %.2f MB", a.label, float64(info.Size())/(1024*1024)))
		} else {
			fatal(fmt.Errorf("%s binary not found after build", a.label))
		}
	}
}

func buildPackage(force bool) {
	log("[5/5] Building package...")
	fnpackURL := fnpackBase + "-" + fnpackSuffix()
	fnpackPath := filepath.Join(buildDir, filepath.Base(fnpackURL))
	log("  Using fnpack for " + platform() + ": " + fnpackURL)
	if info, err := os.Stat(fnpackPath); err == nil && info.Size() > 0 && versions["fnpack"] == fnpackVer && !force {
		log("  Using cached fnpack " + fnpackVer)
	} else {
		download(fnpackURL, fnpackPath, "fnpack", "fnpack", fnpackVer, force)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(fnpackPath, 0o755); err != nil {
			fatal(err)
		}
	}

	fpkOut := filepath.Join(buildDir, "logmanager.fpk")
	_ = os.Remove(fpkOut)
	log("  Running fnpack build...")
	if err := runCmd(buildDir, fnpackPath, "build", "."); err != nil {
		fatal(fmt.Errorf("fnpack build failed: %v", err))
	}
	if !fileExists(fpkOut) {
		fatal(fmt.Errorf("fnpack build failed: logmanager.fpk not found"))
	}
	finalName := "logmanager-" + appVersion + ".fpk"
	if err := os.Rename(fpkOut, filepath.Join(projectDir, finalName)); err != nil {
		fatal(err)
	}
	log("  Build successful!")
	log("")
	log("========================================")
	log("  Build Complete!")
	log("  Output: " + finalName)
	log("========================================")
}

func main() {
	var (
		versionFlag = flag.String("version", "", "打包版本号（默认读 manifest）")
		vFlag       = flag.String("v", "", "打包版本号（默认读 manifest，-version 的短别名）")
		forceFlag   = flag.Bool("force", false, "强制重新下载所有依赖")
		fFlag       = flag.Bool("f", false, "强制重新下载所有依赖（-force 的短别名）")
		skipVueFlag = flag.Bool("skip-vue", false, "跳过 Vue 前端构建")
		cleanFlag   = flag.Bool("clean", false, "强制全量构建（忽略所有缓存）")
	)
	flag.Parse()

	projectDir = findProjectRoot()
	buildDir = filepath.Join(projectDir, ".local-build")

	appVersion = strings.TrimSpace(*versionFlag)
	if appVersion == "" {
		appVersion = strings.TrimSpace(*vFlag)
	}
	if appVersion == "" {
		appVersion = readManifestVersion()
		log("Using version from manifest: " + appVersion)
	} else {
		log("Using version from parameter: " + appVersion)
	}
	force := *forceFlag || *fFlag
	log("Platform: " + platform())
	log("========================================")
	log("  fnOS LogManager - Build")
	log("  Version: " + appVersion)
	log("========================================")

	loadVersions()
	setupBuildDir()
	buildVue(*skipVueFlag, *forceFlag, *cleanFlag)
	copyProjectFiles()
	buildServer()
	buildPackage(force)
}
