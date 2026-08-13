#!/usr/bin/env python3
"""
build.py - fnOS LogManager 跨平台打包脚本（参考 fnos-qbittorrent/build.py）

用法:
    python build.py [--version 0.8.0] [--skip-vue] [--force]

特性:
    - 自动检测操作系统 (Windows/Linux/macOS)，选择对应的 fnpack 构建工具
    - 复用 .local-build 缓存，避免重复下载
    - 交叉编译 Go 服务端 (amd64 + arm64)
    - 构建 Vue 前端并注入版本号
    - 参数与 build.ps1 兼容
"""
import argparse
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import urllib.request

PROJECT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_DIR = os.path.join(PROJECT_DIR, ".local-build")
MANIFEST_FILE = os.path.join(PROJECT_DIR, "manifest")
VERSION_FILE = os.path.join(BUILD_DIR, "versions.json")

# fnpack 构建工具
FNPACK_BASE = "https://static2.fnnas.com/fnpack/fnpack-1.2.3"
FNPACK_VER = "1.2.3"  # 用于版本缓存判断


def log(msg, color="cyan"):
    """简单日志输出，不做 ANSI 颜色处理，避免跨平台乱码。"""
    sys.stdout.write(msg + "\n")
    sys.stdout.flush()


# 官方 fnpack 支持的平台映射（实测，参考 https://developer.fnnas.com/docs/cli/fnpack/）
#   Windows  : windows-amd64
#   Linux    : linux-amd64 / linux-arm
#   macOS    : darwin-amd64 / darwin-arm64
# 注意：Linux arm64 的官方文件名是 "linux-arm"（非 linux-arm64），已实测验证
def get_platform():
    """返回 'windows' / 'linux' / 'darwin'（macOS）。"""
    s = platform.system().lower()
    if s.startswith("win"):
        return "windows"
    if s.startswith("darwin"):
        return "darwin"
    return "linux"


def get_platform_arch():
    """返回当前机器的 CPU 架构标识（amd64 / arm64）。"""
    m = platform.machine().lower()
    if m in ("aarch64", "arm64", "armv8l", "arm"):
        return "arm64"
    return "amd64"


def get_fnpack_url():
    """根据平台和架构返回 fnpack 下载地址，覆盖 Windows/Linux/macOS。

    - 构建工具 fnpack 必须用【当前开发机】的平台，而非目标应用平台
    - 因此这里用 get_platform() + get_platform_arch() 自动检测开发机
    """
    plat = get_platform()
    if plat == "windows":
        fnpack_arch = "amd64"
    elif plat == "darwin":
        # macOS Apple Silicon 用 arm64，Intel 用 amd64
        fnpack_arch = get_platform_arch()
    else:  # linux
        # Linux arm64 官方文件名为 linux-arm
        fnpack_arch = "arm" if get_platform_arch() == "arm64" else "amd64"
    return f"{FNPACK_BASE}-{plat}-{fnpack_arch}"


def load_versions():
    if os.path.exists(VERSION_FILE):
        try:
            with open(VERSION_FILE, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception:
            return {}
    return {}


def save_version(component, version):
    os.makedirs(BUILD_DIR, exist_ok=True)
    versions = load_versions()
    versions[component] = version
    with open(VERSION_FILE, "w", encoding="utf-8") as f:
        json.dump(versions, f, ensure_ascii=False, indent=2)


def version_match(component, expected):
    return load_versions().get(component) == expected


def download(url, out_file, description, component, version, force):
    """带缓存的下载：命中缓存直接返回，否则下载。"""
    if not force and os.path.exists(out_file) and os.path.getsize(out_file) > 0:
        if version_match(component, version):
            log(f"  Using cached {description} (version {version})", "green")
            return True
        log(f"  Version mismatch for {description}, re-downloading...", "yellow")

    log(f"  Downloading {description}...", "yellow")
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=300) as resp:
            data = resp.read()
        if data and len(data) > 0:
            with open(out_file, "wb") as f:
                f.write(data)
            save_version(component, version)
            log(f"  Downloaded {description}", "green")
            return True
    except Exception as e:
        log(f"  ERROR: Failed to download {description}: {e}", "red")
        return False
    return False


def copy_tree(src, dst):
    if os.path.exists(src):
        if os.path.isdir(src):
            shutil.copytree(src, dst, dirs_exist_ok=True)
        else:
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            shutil.copy2(src, dst)


def read_manifest_version():
    with open(MANIFEST_FILE, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line.startswith("version"):
                return line.split("=", 1)[1].strip()
    return ""


def update_manifest(build_dir, version):
    """复制 manifest 并更新版本号。"""
    dest = os.path.join(build_dir, "manifest")
    with open(MANIFEST_FILE, "r", encoding="utf-8") as f:
        content = f.read()
    content = re.sub(r"(?m)^version\s*=.*", f"version = {version}", content)
    with open(dest, "w", encoding="utf-8") as f:
        f.write(content)


def run_command(args, cwd=None, check=True, capture=False):
    """跨平台运行命令。check 为 True 时失败抛异常。

    Windows 下 npm/go 等是 .cmd/.exe，需特殊处理：
    - npm 需使用 npm.cmd（subprocess 无法直接定位 .cmd）
    """
    # Windows 上 .cmd 命令必须显式加后缀，否则 subprocess 找不到可执行文件
    if get_platform() == "windows" and args and args[0].lower() in ("npm", "npx", "pnpm", "yarn"):
        args = list(args)
        args[0] = args[0] + ".cmd"
    log(f"  Running: {' '.join(args)}", "gray")
    result = subprocess.run(args, cwd=cwd, capture_output=capture)
    if check and result.returncode != 0:
        err = result.stderr.decode("utf-8", "replace") if capture and result.stderr else ""
        raise RuntimeError(f"Command failed: {' '.join(args)} {err}")
    return result


def dir_snapshot(root):
    """返回目录内所有文件的 (相对路径, mtime) 快照，用于判断源码是否变更。"""
    snap = {}
    if not os.path.isdir(root):
        return snap
    for base, _dirs, files in os.walk(root):
        # 忽略依赖与构建产物目录，避免缓存失效
        if "node_modules" in base or os.sep + "dist" in base + os.sep:
            continue
        for f in files:
            p = os.path.join(base, f)
            try:
                snap[os.path.relpath(p, root)] = os.path.getmtime(p)
            except OSError:
                pass
    return snap


def main():
    parser = argparse.ArgumentParser(description="fnOS LogManager 跨平台打包脚本")
    parser.add_argument("--version", "-v", default="", help="打包版本号（默认读 manifest）")
    parser.add_argument("--force", "-f", action="store_true", help="强制重新下载所有依赖")
    parser.add_argument("--skip-vue", action="store_true", help="跳过 Vue 前端构建")
    parser.add_argument("--clean", action="store_true", help="强制全量构建（忽略所有缓存）")
    args = parser.parse_args()

    # 版本号：命令行优先，否则读 manifest
    app_version = args.version.strip()
    if not app_version:
        app_version = read_manifest_version()
        if not app_version:
            log("ERROR: 无法从 manifest 读取版本号", "red")
            sys.exit(1)
        log(f"Using version from manifest: {app_version}", "cyan")
    else:
        log(f"Using version from parameter: {app_version}", "cyan")

    log(f"Platform: {get_platform()}/{get_platform_arch()}", "cyan")

    log("========================================", "cyan")
    log(f"  fnOS LogManager - Build", "cyan")
    log(f"  Version: {app_version}", "cyan")
    log("========================================", "cyan")

    # [1/5] 构建目录
    log("[1/5] Setting up build directory...", "yellow")
    clean_dirs = ["app/ui/assets", "app/ui/images", "app/server", "cmd", "config", "wizard"]
    for d in clean_dirs:
        p = os.path.join(BUILD_DIR, d)
        if os.path.exists(p):
            shutil.rmtree(p, ignore_errors=True)
    for d in ["app/server", "app/ui", "cmd", "config", "wizard"]:
        os.makedirs(os.path.join(BUILD_DIR, d), exist_ok=True)
    log("  Build directory ready", "green")

    # [2/5] 构建 Vue 前端
    log("[2/5] Building Vue frontend...", "yellow")
    ui_dir = os.path.join(PROJECT_DIR, "app", "ui")
    ui_dist = os.path.join(ui_dir, "dist")

    # Vue 源码快照：用于判断 src 是否变更（忽略 node_modules/dist）
    # 注：顶部栏版本号由后端 TRIM_APPVER 环境变量运行时提供，前端无需构建时注入
    vue_src = os.path.join(ui_dir, "src")
    vue_snapshot_key = f"vue_src_{app_version}"
    current_vue_snap = dir_snapshot(vue_src)
    last_vue_snap = load_versions().get(vue_snapshot_key)

    # 缓存命中：源码未变 + 版本未变 + dist 已存在 且 非强制全量
    vue_cached = (
        (not args.force)
        and (not args.clean)
        and (not args.skip_vue)
        and last_vue_snap == current_vue_snap
        and os.path.exists(os.path.join(ui_dist, "index.html"))
    )

    if args.skip_vue:
        log("  Skipping Vue build", "yellow")
    elif not os.path.exists(os.path.join(ui_dir, "package.json")):
        log("  Skipping Vue build (no package.json)", "yellow")
    elif vue_cached:
        log("  Vue source unchanged, reusing cached dist", "green")
    else:
        if not os.path.exists(os.path.join(ui_dir, "node_modules")):
            log("  Installing npm dependencies...", "yellow")
            run_command(["npm", "install"], cwd=ui_dir)
        try:
            log("  Building Vue app...", "yellow")
            run_command(["npm", "run", "build"], cwd=ui_dir)
            if not os.path.exists(ui_dist):
                raise RuntimeError("Vue build output not found")
            log("  Vue build complete", "green")
            save_version(vue_snapshot_key, current_vue_snap)
        except Exception as e:
            log(f"  ERROR: Vue build failed - {e}", "red")
            sys.exit(1)

    # [3/5] 复制项目文件
    log("[3/5] Copying project files...", "yellow")
    for sub in ["cmd", "config", "wizard"]:
        src = os.path.join(PROJECT_DIR, sub)
        if os.path.isdir(src):
            copy_tree(src, os.path.join(BUILD_DIR, sub))
    update_manifest(BUILD_DIR, app_version)
    for icon in ["ICON.PNG", "ICON_256.PNG"]:
        p = os.path.join(PROJECT_DIR, icon)
        if os.path.exists(p):
            shutil.copy2(p, BUILD_DIR)

    if os.path.exists(ui_dist):
        copy_tree(ui_dist, os.path.join(BUILD_DIR, "app", "ui"))
        for sub in ["config", "images"]:
            src = os.path.join(ui_dir, sub)
            if os.path.exists(src):
                copy_tree(src, os.path.join(BUILD_DIR, "app", "ui", sub))
        log("  Vue dist files copied", "green")
    else:
        # 无 dist 时拷贝静态文件（config/images/index.html）
        for sub in ["config", "images", "index.html"]:
            src = os.path.join(ui_dir, sub)
            if os.path.exists(src):
                copy_tree(src, os.path.join(BUILD_DIR, "app", "ui", sub))
        log("  Static UI files copied", "green")

    # [4/5] 交叉编译 Go server 二进制
    log("[4/5] Building Go server binaries...", "yellow")
    server_src = os.path.join(PROJECT_DIR, "app", "server")
    server_dir = os.path.join(BUILD_DIR, "app", "server")
    os.makedirs(server_dir, exist_ok=True)

    architectures = [("amd64", "x86_64"), ("arm64", "ARM64")]
    for goarch, label in architectures:
        binary_name = f"logmanager-server-{goarch}"
        binary_path = os.path.join(server_dir, binary_name)
        log(f"  Cross-compiling for linux/{goarch} ({label})...", "yellow")

        env = os.environ.copy()
        env["GOOS"] = "linux"
        env["GOARCH"] = goarch
        env["CGO_ENABLED"] = "0"
        try:
            result = subprocess.run(
                ["go", "build", "-o", binary_path, "-ldflags=-s -w", "./cmd/server"],
                cwd=server_src, env=env, capture_output=True,
            )
        except FileNotFoundError:
            log("  ERROR: Go 未安装，请先安装 Go 并加入 PATH", "red")
            sys.exit(1)

        if result.returncode != 0:
            log(f"  ERROR: Go build for {label} failed", "red")
            if result.stderr:
                log("  " + result.stderr.decode("utf-8", "replace")[:2000], "red")
            sys.exit(1)

        if os.path.exists(binary_path):
            size_mb = os.path.getsize(binary_path) / (1024 * 1024)
            log(f"  {label} binary: {size_mb:.2f} MB", "green")
        else:
            log(f"  ERROR: {label} binary not found after build", "red")
            sys.exit(1)

    # [5/5] 构建 fpk 包
    log("[5/5] Building package...", "yellow")
    fnpack_url = get_fnpack_url()
    fnpack_name = fnpack_url.rsplit("/", 1)[-1]
    fnpack_path = os.path.join(BUILD_DIR, fnpack_name)
    log(f"  Using fnpack for {get_platform()}/{get_platform_arch()}: {fnpack_url}", "gray")
    if not args.force and os.path.exists(fnpack_path) and os.path.getsize(fnpack_path) > 0 and version_match("fnpack", FNPACK_VER):
        log(f"  Using cached fnpack {FNPACK_VER}", "green")
    else:
        if not download(fnpack_url, fnpack_path, "fnpack", "fnpack", FNPACK_VER, args.force):
            sys.exit(1)

    # Linux/macOS 需要赋予可执行权限
    if get_platform() != "windows":
        os.chmod(fnpack_path, 0o755)

    fpk_out = os.path.join(BUILD_DIR, "logmanager.fpk")
    if os.path.exists(fpk_out):
        os.remove(fpk_out)

    log("  Running fnpack build...", "gray")
    old_cwd = os.getcwd()
    os.chdir(BUILD_DIR)
    try:
        if get_platform() == "windows":
            proc = subprocess.run([fnpack_path, "build", "."], capture_output=True)
        else:
            proc = subprocess.run(["./" + fnpack_name, "build", "."], capture_output=True)
    finally:
        os.chdir(old_cwd)

    if not os.path.exists(fpk_out):
        log("  ERROR: fnpack build failed", "red")
        if proc.stderr:
            log("  " + proc.stderr.decode("utf-8", "replace")[:2000], "red")
        sys.exit(1)

    final_name = f"logmanager-{app_version}.fpk"
    shutil.move(fpk_out, os.path.join(PROJECT_DIR, final_name))
    log("  Build successful!", "green")

    log("", "cyan")
    log("========================================", "green")
    log("  Build Complete!", "green")
    log(f"  Output: {final_name}", "green")
    log("========================================", "green")


if __name__ == "__main__":
    main()
