package routes

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// processInfo represents a running system process.
type processInfo struct {
	PID         int      `json:"pid"`
	PPID        int      `json:"ppid"`
	User        string   `json:"user"`
	Name        string   `json:"name"`
	State       string   `json:"state"`
	CPU         float64  `json:"cpu"`      // CPU 占用率（%），基于两次采样
	Memory      string   `json:"memory"`   // 内存占用（格式化）
	MemBytes    int64    `json:"memBytes"` // 内存占用（字节，用于排序）
	StartTime   string   `json:"startTime"`
	Command     string   `json:"command"`
	ExePath     string   `json:"exePath"`
	Ports       []int    `json:"ports"` // 监听端口
	Protect     bool     `json:"protect"` // 受保护进程，禁止结束
	System      bool     `json:"system"`  // 系统级进程（内核线程/无用户态命令行）
	IsDocker      bool   `json:"isDocker"` // 是否为 docker 容器进程
	ContainerID   string `json:"containerId,omitempty"`   // docker 容器短 ID（若可解析）
	ContainerName string `json:"containerName,omitempty"` // docker 容器名称（若可解析）
}

// RegisterProcessRoutes registers process-management routes.
// 进程管理（参考 Supervisor 的进程管理理念）：列出系统中运行的进程，并支持结束（kill）进程。
func RegisterProcessRoutes(rg *gin.RouterGroup) {
	rg.GET("/processes", middleware.ValidateToken, listProcessesHandler)
	// 列出进程打开的文件、读取进程日志、结束进程均涉及敏感系统信息，仅限管理员
	rg.GET("/processes/:pid/files", middleware.ValidateToken, middleware.RequireAdmin, listProcessFilesHandler)
	rg.GET("/processes/:pid/log", middleware.ValidateToken, middleware.RequireAdmin, readProcessLogHandler)
	rg.POST("/processes/kill", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 60000), killProcessHandler)
}

// protectedPIDs 是禁止结束的系统关键进程（PID 1 = init/systemd）。
// 同时禁止结束自身（logmanager 进程），避免管理员误杀导致应用崩溃。
var protectedPIDs = map[int]bool{
	1: true, // init / systemd
}

// isProtectedPID 判断进程是否受保护（关键系统进程或本应用自身）。
func isProtectedPID(pid int) bool {
	if pid <= 0 || protectedPIDs[pid] {
		return true
	}
	// 自身进程（当前二进制）不可结束
	if pid == os.Getpid() {
		return true
	}
	// 找到 logmanager 服务器二进制路径，若目标进程为其自身则受保护
	selfExe, err := os.Readlink("/proc/self/exe")
	if err == nil {
		targetExe, err2 := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err2 == nil && targetExe == selfExe {
			return true
		}
	}
	return false
}

// listProcessesHandler 列出系统中运行的进程。
// 支持参数：
//   scope: user(默认，仅用户态服务进程) / all(全部)
//   q:     按进程名/命令行/PID/端口关键字过滤
//   sort:  pid(默认) / name / cpu / mem
//   order: asc(默认) / desc
func listProcessesHandler(c *gin.Context) {
	procs, err := readProcesses()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取进程列表失败: " + err.Error()})
		return
	}

	// 过滤系统进程（默认只看用户进程/服务）
	scope := c.DefaultQuery("scope", "user")
	if scope != "all" {
		filtered := procs[:0]
		for _, p := range procs {
			if !p.System {
				filtered = append(filtered, p)
			}
		}
		procs = filtered
	}

	// 关键字过滤：进程名 / 命令行 / 可执行路径 / PID / 端口
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		query := strings.ToLower(q)
		queryInt := 0
		if n, err := strconv.Atoi(q); err == nil {
			queryInt = n
		}
		filtered := make([]processInfo, 0, len(procs))
		for _, p := range procs {
			if matchesProcessQuery(p, query, queryInt) {
				filtered = append(filtered, p)
			}
		}
		procs = filtered
	}

	// 排序
	sortProcesses(procs, c.DefaultQuery("sort", "pid"), c.DefaultQuery("order", "asc"))

	c.JSON(http.StatusOK, gin.H{"total": len(procs), "processes": procs})
}

// matchesProcessQuery 判断进程是否匹配搜索关键词（进程名/命令行/可执行路径/PID/端口）。
func matchesProcessQuery(p processInfo, query string, queryInt int) bool {
	if query == "" {
		return true
	}
	// 精确 PID 匹配（或 PID 子串）
	if p.PID == queryInt {
		return true
	}
	if strconv.Itoa(p.PID) == query || strings.Contains(strconv.Itoa(p.PID), query) {
		return true
	}
	// 端口匹配：query 为纯数字时匹配监听端口
	if queryInt > 0 {
		for _, port := range p.Ports {
			if port == queryInt {
				return true
			}
		}
	}
	// 进程名 / 命令行 / 可执行路径包含关键词
	return strings.Contains(strings.ToLower(p.Name), query) ||
		strings.Contains(strings.ToLower(p.Command), query) ||
		strings.Contains(strings.ToLower(p.ExePath), query)
}

// sortProcesses 按指定字段对进程排序（desc 时降序，相等时保持稳定顺序）。
func sortProcesses(procs []processInfo, sortKey, order string) {
	desc := order == "desc"
	less := func(i, j int) bool {
		a, b := procs[i], procs[j]
		var lt bool
		switch sortKey {
		case "name":
			lt = a.Name < b.Name
		case "cpu":
			lt = a.CPU < b.CPU
		case "mem":
			lt = a.MemBytes < b.MemBytes
		default: // pid
			lt = a.PID < b.PID
		}
		if lt {
			return !desc
		}
		if a.PID == b.PID && a.Name == b.Name && a.CPU == b.CPU && a.MemBytes == b.MemBytes {
			return false // 视为相等，保持稳定
		}
		return desc
	}
	sort.SliceStable(procs, less)
}

// killProcessHandler 结束指定 PID 的进程。
// 先发送 SIGTERM 请求优雅退出，超时后升级为 SIGKILL。
func killProcessHandler(c *gin.Context) {
	var req struct {
		PID int    `json:"pid"`
		Sig string `json:"signal"` // 可选：term(默认) / kill
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.PID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 PID"})
		return
	}

	// 受保护进程禁止结束
	if isProtectedPID(req.PID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "该进程受保护，不允许结束"})
		return
	}

	// 确认进程存在并记录其启动时间戳（startTicks）。启动时间用于后续
	// 检测 PID 是否被复用：/proc/PID 的"存在"本身不足以保证还是原来那个进程。
	// FIX(bug 19): 若在 SIGTERM→SIGKILL 升级期间该 PID 被操作系统回收并分配给
	// 另一个无关进程，直接按 PID 补 SIGKILL 会误杀新进程。
	origStat := readProcStatFields(req.PID)
	if origStat.startTicks == 0 {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", req.PID)); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "进程不存在或已退出"})
			return
		}
		// startTicks==0 但 /proc/PID 存在：极罕见的解析失败，保守地拒绝操作
		// 而非冒险按 PID 补杀，避免误杀。
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法确认进程启动信息，已中止操作"})
		return
	}

	sig := syscall.SIGTERM
	force := false
	switch req.Sig {
	case "kill":
		sig = syscall.SIGKILL
		force = true
	case "", "term":
		// default SIGTERM
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的信号类型，仅支持 term 或 kill"})
		return
	}

	cmdName := readProcessCommand(req.PID)

	// 发送信号（使用 os.FindProcess + Signal，保证 Linux 目标可用且可跨平台编译）
	if err := sendSignal(req.PID, sig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "结束进程失败: " + err.Error()})
		return
	}

	// 优雅退出：等待进程退出，超时后强制结束。等待期间持续用 startTicks
	// 校验仍是原来那个进程，绝不对被复用的 PID 补发 SIGKILL。
	terminated := false
	if !force {
		for i := 0; i < 20; i++ {
			time.Sleep(150 * time.Millisecond)
			if _, err := os.Stat(fmt.Sprintf("/proc/%d", req.PID)); os.IsNotExist(err) {
				terminated = true
				break
			}
			// PID 仍存在：确认它仍是原进程（startTicks 一致）。
			cur := readProcStatFields(req.PID)
			if cur.startTicks != origStat.startTicks {
				// PID 已被复用为别的进程——绝不能补杀，直接认定"已退出"。
				terminated = true
				break
			}
		}
		if !terminated {
			// 仍是原进程且未在超时时间内退出，升级为 SIGKILL。
			sendSignal(req.PID, syscall.SIGKILL)
		}
	} else {
		terminated = true
	}

	// 记录审计日志（结束进程是高危操作）
	services.AddSecurityAuditLog("PROCESS_KILL", map[string]interface{}{
		"pid":      req.PID,
		"command":  cmdName,
		"signal":   req.Sig,
		"force":    force,
		"outcome":  map[bool]string{true: "terminated", false: "killed"}[terminated],
	}, c)

	slog.Info("结束进程", "pid", req.PID, "command", cmdName, "signal", req.Sig)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"pid":        req.PID,
		"command":    cmdName,
		"signal":     req.Sig,
		"terminated": terminated,
	})
}

// ---------------------------------------------------------------------------
// 进程信息读取（解析 /proc）
// ---------------------------------------------------------------------------

func readProcesses() ([]processInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	// 一次性构建：监听端口映射、UID->用户名映射、系统 CPU 总 jiffies
	portByInode := readListenPorts()
	uidMap := loadUIDMap()
	sysJiffies := readSystemJiffies()
	// 容器 ID -> 名称 映射（用于给 docker 进程标注容器名）
	containerNameByID := loadDockerContainerNames()

	procs := make([]processInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}

		// 一次读取 /proc/pid/stat，解析出 PPID/state/utime/stime/starttime
		comm := readFileString(fmt.Sprintf("/proc/%d/comm", pid))
		st := readProcStatFields(pid)
		cmdline := readProcessCommand(pid)
		// 系统进程：内核线程（comm 以 [ 开头）或无用户态命令行（cmdline 为空）
		isKernelThread := strings.HasPrefix(comm, "[")
		isSystem := isKernelThread || (cmdline == "" && pid > 1)
		isDocker, containerID := detectDockerProcess(pid)

		p := processInfo{
			PID:       pid,
			PPID:      st.ppid,
			User:      lookupUserName(uidMap, pid),
			Name:      processName(comm, cmdline),
			State:     mapState(st.state),
			Command:   cmdline,
			StartTime: readProcStartTime(st.startTicks),
			CPU:       computeCPUUsage(pid, comm, st.utime, st.stime, sysJiffies),
			Protect:       isProtectedPID(pid),
			System:        isSystem,
			IsDocker:      isDocker,
			ContainerID:   containerID,
			ContainerName: containerNameByID[containerID],
			}

		p.Memory, p.MemBytes = readProcMemory(pid)

		// 仅对用户态进程解析 exe 与监听端口，内核线程无这些信息，跳过以省去系统调用
		if !isKernelThread {
			p.ExePath = readProcExe(pid)
			p.Ports = resolvePorts(pid, portByInode)
		}

		procs = append(procs, p)
	}

	return procs, nil
}

// 容器名映射缓存：进程列表接口可能被频繁刷新，每次执行 `docker ps` 会产生系统调用
// 开销并可能拖慢接口（docker daemon 无响应时）。用一个短 TTL 缓存复用结果。
var (
	containerNameCache     map[string]string
	containerNameCacheAt   time.Time
	containerNameCacheMu   sync.Mutex
	containerNameCacheTTL  = 30 * time.Second
)

// loadDockerContainerNames 通过 `docker ps` 构建 容器短ID -> 容器名 的映射。
// 供进程管理给 docker 容器进程标注容器名。失败时返回空映射（不影响进程列表本身）。
func loadDockerContainerNames() map[string]string {
	containerNameCacheMu.Lock()
	defer containerNameCacheMu.Unlock()

	// 缓存命中（TTL 内）直接复用
	if containerNameCache != nil && time.Since(containerNameCacheAt) < containerNameCacheTTL {
		return containerNameCache
	}

	// docker 命令参数固定（无用户输入），不会产生命令注入。
	// 使用固定的短超时与输出上限，避免异常时拖慢进程列表接口。
	stdout, err := execDocker([]string{"ps", "--format", "{{.ID}}\t{{.Names}}", "--no-trunc"}, 3000, 65536)
	if err != nil {
		// docker 不可用时返回空映射，进程列表仍正常返回
		return map[string]string{}
	}
	m := buildContainerNameMap(stdout)
	containerNameCache = m
	containerNameCacheAt = time.Now()
	return m
}

// buildContainerNameMap 解析 `docker ps --format '{{.ID}}\t{{.Names}}'` 的输出，
// 构建 完整ID -> 名称 与 短ID(前12位) -> 名称 两个键的映射，便于按短ID反查容器名。
func buildContainerNameMap(stdout string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		fullID := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		// 去掉容器名常见的 "/" 前缀（docker 命名约定）
		name = strings.TrimPrefix(name, "/")
		if len(fullID) >= 12 && name != "" {
			m[fullID] = name
			m[fullID[:12]] = name
		}
	}
	return m
}

// detectDockerProcess 通过 /proc/<pid>/cgroup 判断进程是否属于 docker 容器。
// 返回是否容器进程，以及解析出的容器短 ID（若非容器则 ID 为空）。
//
// 覆盖 fnOS / 主流 Docker 系统的常见 cgroup 命名方式：
//   - cgroup v2 + systemd 驱动:  /system.slice/docker-<64位ID>.scope   （fnOS 实测）
//   - cgroup v1 经典路径:        /docker/<64位ID>/...                 （Docker 经典）
//   - containerd runc:           /containerd/<ns>-<64位ID>.scope/...
//
// 容器 ID 取完整 64 位 ID 的前 12 位（与 docker ps 显示的短 ID 一致）。
func detectDockerProcess(pid int) (bool, string) {
	data := readFileString(fmt.Sprintf("/proc/%d/cgroup", pid))
	if data == "" {
		return false, ""
	}

	// 遍历 cgroup 的每一行（同一进程可能挂在多个子系统下）
	for _, line := range strings.Split(data, "\n") {
		if isD, id := detectDockerFromCgroupLine(line); isD {
			return true, id
		}
	}

	return false, ""
}

// detectDockerFromCgroupLine 对 cgroup 单行内容执行 docker 容器判定。
// 独立出来以便用真实 cgroup 内容做单元测试。
func detectDockerFromCgroupLine(line string) (bool, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return false, ""
	}

	// 模式 1: systemd 驱动（cgroup v2）: docker-<64位ID>.scope
	// 路径形如 /system.slice/docker-fce8...a873.scope，不含 /docker/ 分隔符。
	// 注意：宿主 dockerd 进程 cgroup 是 /system.slice/docker.service，其
	// "docker-" 后是 "service" 非 64 位 hex，extractDockerID 会返回空，不会误判。
	if idx := strings.Index(line, "docker-"); idx >= 0 {
		rest := line[idx+len("docker-"):]
		if containerID := extractDockerID(rest); containerID != "" {
			return true, containerID
		}
	}

	// 模式 2: .../docker/<64位ID>/... （Docker cgroup v1 经典路径）
	if idx := strings.Index(line, "/docker/"); idx >= 0 {
		rest := line[idx+len("/docker/"):]
		if containerID := extractDockerID(rest); containerID != "" {
			return true, containerID
		}
	}

	// 模式 3: .../containerd/<namespace>-<ID>.scope/... （containerd runc 运行时）
	if idx := strings.Index(line, "/containerd/"); idx >= 0 {
		rest := line[idx+len("/containerd/"):]
		if containerID := extractDockerID(rest); containerID != "" {
			return true, containerID
		}
	}

	return false, ""
}

// extractDockerID 从 cgroup 路径片段中提取容器 ID（64 位十六进制，取前 12 位）。
// 兼容 "1234567890ab..."（docker 直接路径）与 "docker-1234567890ab.scope" 等形式。
func extractDockerID(s string) string {
	s = strings.TrimPrefix(s, "/")
	// 去掉后缀 .scope / 后续路径
	if idx := strings.Index(s, ".scope"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	// 去掉可能的 "docker-" 前缀（systemd scope 形式）
	s = strings.TrimPrefix(s, "docker-")
	s = strings.TrimSpace(s)

	// 判断是否为 64 位十六进制 ID
	if len(s) >= 64 {
		valid := true
		for _, r := range s[:64] {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				valid = false
				break
			}
		}
		if valid {
			return s[:12]
		}
	}
	return ""
}

// procStatFields 为单次读取 /proc/pid/stat 的解析结果。
type procStatFields struct {
	ppid       int
	state      string
	utime      int64
	stime      int64
	startTicks int64
}

// readProcStatFields 一次性读取 /proc/pid/stat 并解析出 PPID、状态、CPU 时间、启动时间。
// 相比逐字段多次读取文件，大幅减少系统调用次数。
func readProcStatFields(pid int) procStatFields {
	var res procStatFields
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return res
	}
	s := string(data)
	start := strings.IndexByte(s, '(')
	end := strings.LastIndexByte(s, ')')
	if start < 0 || end < 0 || end >= len(s) {
		return res
	}
	// 括号后字段：state ppid pgrp session tty_nr tpgid flags minflt ...
	// index: state=0, ppid=1, ..., utime=11, stime=12, ..., starttime=21
	fields := strings.Fields(s[end+1:])
	if len(fields) > 0 {
		res.state = fields[0]
	}
	if len(fields) > 1 {
		res.ppid, _ = strconv.Atoi(fields[1])
	}
	if len(fields) > 12 {
		res.utime, _ = strconv.ParseInt(fields[11], 10, 64)
		res.stime, _ = strconv.ParseInt(fields[12], 10, 64)
	}
	if len(fields) > 21 {
		res.startTicks, _ = strconv.ParseInt(fields[21], 10, 64)
	}
	return res
}

// loadUIDMap 一次性读取 /etc/passwd，构建 UID -> 用户名 映射。
// 避免为每个进程都打开并扫描 /etc/passwd（这是进程列表性能瓶颈之一）。
func loadUIDMap() map[int]string {
	m := map[int]string{}
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return m
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 3 {
			if uid, err := strconv.Atoi(parts[2]); err == nil {
				m[uid] = parts[0]
			}
		}
	}
	return m
}

// lookupUserName 解析进程 UID 对应的用户名（使用一次性构建的 UID 映射）。
func lookupUserName(uidMap map[int]string, pid int) string {
	uid := -1
	scanner := bufio.NewScanner(strings.NewReader(readFileString(fmt.Sprintf("/proc/%d/status", pid))))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid, _ = strconv.Atoi(fields[1])
			}
			break
		}
	}
	if uid < 0 {
		return ""
	}
	if name, ok := uidMap[uid]; ok {
		return name
	}
	return strconv.Itoa(uid)
}

// processName 计算进程名：优先用 comm，若 comm 为空则用命令行第一个 token。
func processName(comm, cmdline string) string {
	name := strings.TrimSpace(comm)
	if name == "" {
		if idx := strings.IndexAny(cmdline, " \t"); idx > 0 {
			name = cmdline[:idx]
		} else if cmdline != "" {
			name = cmdline
		}
	}
	// 去掉路径前缀（取 basename）
	name = filepath.Base(name)
	return name
}

// ---------------------------------------------------------------------------
// CPU 占用率（两次采样）
// ---------------------------------------------------------------------------

type cpuSample struct {
	procJiffies int64 // 进程累计 CPU jiffies
	sysJiffies  int64 // 系统总 CPU jiffies
	valid       bool
	lastSeen    time.Time // 最近一次采样时间，用于清理过期条目（防止 map 无界增长）
}

var (
	cpuSampleMu sync.Mutex
	cpuSamples  = map[int]cpuSample{}
	// cpuSampleTTL 采样条目存活时间：超过该时长未再出现的 PID（多为已退出进程）
	// 将被周期性清理，避免短生命周期进程导致 cpuSamples 无界增长（内存泄漏）。
	cpuSampleTTL = 10 * time.Minute
)

// computeCPUUsage 计算进程 CPU 占用率。返回百分比（0-100+，可大于 100 多核）。
// 基于前后两次请求的采样增量，首次采样返回 0。
// utime/stime 已由调用方从单次 stat 读取传入，sysJiffies 为本次请求的系统总 jiffies。
func computeCPUUsage(pid int, comm string, utime, stime, sysJiffies int64) float64 {
	if strings.HasPrefix(comm, "[") {
		return 0
	}
	procJiffies := utime + stime
	if procJiffies < 0 || sysJiffies <= 0 {
		return 0
	}

	cpuSampleMu.Lock()
	defer cpuSampleMu.Unlock()

	// 周期性清理过期采样：每次调用以 1/100 概率触发一次全量清理，
	// 删除超过 cpuSampleTTL 未再采样的条目（已退出/短生命周期进程）。
	// 概率触发避免每次请求都遍历整个 map，同时保证清理最终必然发生。
	if rand.Intn(100) == 0 {
		now := time.Now()
		for pid, s := range cpuSamples {
			if now.Sub(s.lastSeen) > cpuSampleTTL {
				delete(cpuSamples, pid)
			}
		}
	}

	prev, ok := cpuSamples[pid]
	cpuSamples[pid] = cpuSample{procJiffies: procJiffies, sysJiffies: sysJiffies, valid: true, lastSeen: time.Now()}

	if !ok || !prev.valid {
		return 0 // 首次采样，无增量
	}
	dp := procJiffies - prev.procJiffies
	ds := sysJiffies - prev.sysJiffies
	if ds <= 0 || dp < 0 {
		return 0
	}
	// 占用率 = dp / ds * 100（ds 为总 CPU 时间增量，对应单核 100%）
	return float64(dp) / float64(ds) * 100.0
}

// readSystemJiffies 读取系统总 CPU jiffies（/proc/stat 所有 CPU 行求和）。
func readSystemJiffies() int64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	var total int64
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		// 跳过 "cpu " 汇总行（全核合计）：per-CPU 行（cpu0/cpu1...）之和 == 汇总行，
		// 若把汇总行也累加会导致 total ≈ 2× 真实值，进而使 CPU% 少报约一半。
		// 判断依据："cpu " 是 "cpu"+空格（第 4 个字符为空格），而 "cpu0" 第 4 个字符是数字。
		if len(line) > 3 && line[3] == ' ' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		for _, f := range fields[1:] {
			v, err := strconv.ParseInt(f, 10, 64)
			if err == nil {
				total += v
			}
		}
	}
	return total
}

// ---------------------------------------------------------------------------
// 内存占用
// ---------------------------------------------------------------------------

// readProcMemory 读取进程内存占用（VmRSS），返回格式化字符串和字节数。
func readProcMemory(pid int) (string, int64) {
	vmRSS := int64(0)
	scanner := bufio.NewScanner(strings.NewReader(readFileString(fmt.Sprintf("/proc/%d/status", pid))))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				vmRSS, _ = strconv.ParseInt(fields[1], 10, 64)
			}
			break
		}
	}
	bytes := vmRSS * 1024
	if bytes <= 0 {
		return "-", 0
	}
	return formatProcessBytes(bytes), bytes
}

// ---------------------------------------------------------------------------
// 监听端口解析
// ---------------------------------------------------------------------------

// 监听端口映射缓存（短 TTL）。/proc/net/tcp 可能很大，且监听状态变化不频繁，
// 短时间复用可避免每次请求都重复解析大文件。这不属于进程预加载，仅缓存系统级低频信息。
var (
	listenPortMu     sync.Mutex
	listenPortCache  map[string]int
	listenPortCached time.Time
	listenPortTTL    = 3 * time.Second
)

// readListenPorts 读取 /proc/net/tcp 和 tcp6，构建 "socket inode -> 端口" 映射（仅 LISTEN）。
// 带短 TTL 缓存，避免每次请求都重新解析系统网络表。
func readListenPorts() map[string]int {
	listenPortMu.Lock()
	defer listenPortMu.Unlock()
	if listenPortCache != nil && time.Since(listenPortCached) < listenPortTTL {
		return listenPortCache
	}
	ports := map[string]int{}
	parseTCPFile("/proc/net/tcp", ports)
	parseTCPFile("/proc/net/tcp6", ports)
	listenPortCache = ports
	listenPortCached = time.Now()
	return ports
}

func parseTCPFile(path string, ports map[string]int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue // 跳过表头
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// fields[1]=local_address(hex), fields[3]=st, fields[9]=inode
		if fields[3] != "0A" { // 0A = LISTEN
			continue
		}
		port := hexPort(fields[1])
		inode := fields[9]
		if port > 0 && inode != "" && inode != "0" {
			ports[inode] = port
		}
	}
}

// hexPort 从 "0100007F:1F90" 形式的 local_address 提取十进制端口。
func hexPort(localAddr string) int {
	idx := strings.IndexByte(localAddr, ':')
	if idx < 0 {
		return 0
	}
	hex := localAddr[idx+1:]
	v, err := strconv.ParseUint(hex, 16, 16)
	if err != nil {
		return 0
	}
	return int(v)
}

// resolvePorts 解析指定 PID 的监听端口：读取 /proc/pid/fd 中的 socket inode，与端口映射匹配。
func resolvePorts(pid int, portByInode map[string]int) []int {
	if len(portByInode) == 0 {
		return nil
	}
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}
	var ports []int
	seen := map[int]bool{}
	for _, e := range entries {
		link, err := os.Readlink(fdDir + "/" + e.Name())
		if err != nil {
			continue
		}
		// 形如 "socket:[12345]"
		if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
			continue
		}
		inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
		if port, ok := portByInode[inode]; ok && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	sort.Ints(ports)
	return ports
}

// ---------------------------------------------------------------------------
// 进程日志查看（借鉴 Supervisor 的子进程日志管理理念）
// ---------------------------------------------------------------------------

// processFileInfo 描述进程打开的一个文件。
type processFileInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	IsLog    bool   `json:"isLog"`    // 是否为日志文件
	Size     int64  `json:"size"`
	SizeText string `json:"sizeText"`
}

// listProcessFilesHandler 列出指定进程通过 fd 打开的常规文件（排除 socket/pipe/dev/proc/sys 等）。
func listProcessFilesHandler(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 PID"})
		return
	}

	files, err := readProcessFiles(pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pid": pid, "files": files})
}

// readProcessLogHandler 读取指定进程打开的日志文件内容。
// 安全模型：路径必须是该进程当前通过 fd 打开的常规文件，且为日志文件，
// 并通过符号链接与常规文件校验。仅在用户态进程中开放。
func readProcessLogHandler(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 PID"})
		return
	}
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 path 参数"})
		return
	}

	// 校验该路径确实是此进程打开的常规日志文件
	if err := validateProcessLogFile(pid, path); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	maxLines := 200
	if v := c.Query("maxLines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			maxLines = n
		}
	}
	tail := c.Query("tail") == "true"

	result, err := services.ReadLogFileAt(path, types.ReadLogOptions{
		MaxLines: maxLines,
		Tail:     tail,
	})
	if err != nil {
		slog.Error("failed to read process log", "pid", pid, "path", path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取进程日志失败: " + err.Error()})
		return
	}

	// 记录审计
	services.AddSecurityAuditLog("PROCESS_LOG_READ", map[string]interface{}{
		"pid":  pid,
		"path": path,
	}, c)

	c.JSON(http.StatusOK, result)
}

// readProcessFiles 读取进程通过 fd 打开的常规文件，过滤出日志相关文件。
func readProcessFiles(pid int) ([]processFileInfo, error) {
	if isKernelPID(pid) {
		return nil, fmt.Errorf("内核线程无用户态文件")
	}
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil, fmt.Errorf("无法读取进程文件描述符（进程可能不存在或无权限）")
	}

	var files []processFileInfo
	seen := map[string]bool{}
	for _, e := range entries {
		link, err := os.Readlink(fdDir + "/" + e.Name())
		if err != nil {
			continue
		}
		// 只保留常规文件路径
		if !isSafeRegularFileLink(link) {
			continue
		}
		path := link
		if seen[path] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		seen[path] = true
		files = append(files, processFileInfo{
			Path:     path,
			Name:     filepath.Base(path),
			IsLog:    isLogFileLike(path),
			Size:     info.Size(),
			SizeText: formatProcessBytes(info.Size()),
		})
	}

	// 日志文件优先，再按名称排序
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsLog != files[j].IsLog {
			return files[i].IsLog
		}
		return files[i].Path < files[j].Path
	})
	return files, nil
}

// isKernelPID 判断是否为内核线程 PID（无用户态 fd 信息）。
func isKernelPID(pid int) bool {
	comm := readFileString(fmt.Sprintf("/proc/%d/comm", pid))
	return strings.HasPrefix(comm, "[")
}

// isSafeRegularFileLink 判断 fd 链接是否为可安全访问的常规文件路径。
// 排除 socket、pipe、anon_inode、/proc、/dev、/sys、/run 等伪文件或敏感目录。
func isSafeRegularFileLink(link string) bool {
	if !strings.HasPrefix(link, "/") {
		return false // socket:[...], pipe:[...], anon_inode:[...] 等非路径链接
	}
	for _, prefix := range []string{"/proc/", "/dev/", "/sys/", "/run/", "/var/run/", "/tmp/", "/var/tmp/"} {
		if strings.HasPrefix(link, prefix) {
			return false
		}
	}
	return true
}

// validateProcessLogFile 校验 path 是否为指定进程当前打开的常规日志文件。
func validateProcessLogFile(pid int, path string) error {
	normalized := utils.SafePath(path)
	if normalized == "" {
		return fmt.Errorf("非法路径")
	}
	if !isLogFileLike(normalized) {
		return fmt.Errorf("仅允许读取日志文件")
	}
	// 必须是该进程 fd 打开的常规文件
	files, err := readProcessFiles(pid)
	if err != nil {
		return fmt.Errorf("无法验证进程文件: %v", err)
	}
	for _, f := range files {
		if f.Path == normalized {
			return nil
		}
	}
	return fmt.Errorf("该路径不是此进程当前打开的文件")
}

// isLogFileLike 判断路径是否像日志文件（.log 后缀或位于日志目录）。
func isLogFileLike(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range []string{".log", ".out", ".err"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	if strings.Contains(path, "/log/") || strings.Contains(path, "/logs/") || strings.HasSuffix(path, ".log") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 其他 /proc 读取辅助
// ---------------------------------------------------------------------------

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func mapState(code string) string {
	switch code {
	case "R":
		return "运行中"
	case "S":
		return "睡眠"
	case "D":
		return "磁盘等待"
	case "Z":
		return "僵尸"
	case "T":
		return "已停止"
	case "t":
		return "跟踪"
	case "X":
		return "已退出"
	default:
		return code
	}
}

// readProcessCommand 读取完整命令行（/proc/pid/cmdline）。
func readProcessCommand(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		// 回退到 comm
		return readFileString(fmt.Sprintf("/proc/%d/comm", pid))
	}
	// cmdline 以 \0 分隔参数
	parts := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	return strings.Join(parts, " ")
}

// readProcExe 读取进程可执行文件路径。
func readProcExe(pid int) string {
	path, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return ""
	}
	return path
}

// readProcStartTime 将进程启动时间（starttime，自系统启动的时钟周期数）格式化为时间字符串。
func readProcStartTime(startTicks int64) string {
	if startTicks <= 0 {
		return ""
	}
	// 系统启动时间
	uptimeSec := readProcUptime()
	if uptimeSec <= 0 {
		return ""
	}
	// 使用系统真实时钟频率（USER_HZ）而非硬编码 100：
	// 若系统 USER_HZ=1000（部分平台），硬编码 100 会导致启动时间计算错误。
	hz := readClockTicksPerSec()
	procStartOffsetSec := startTicks / hz
	bootTime := time.Now().Unix() - uptimeSec
	startTime := time.Unix(bootTime+procStartOffsetSec, 0)
	return startTime.Format("2006-01-02 15:04:05")
}

var (
	clockTicksOnce sync.Once
	clockTicks     int64
)

// readClockTicksPerSec 返回系统时钟频率（USER_HZ，每秒时钟周期数）。
// 优先从 /proc/self/auxv 的 AT_CLKTCK（entry type 17）读取真实值，
// 读取失败时回退到 100（常见 CLK_TCK）。结果缓存，避免每个进程重复读取。
func readClockTicksPerSec() int64 {
	clockTicksOnce.Do(func() {
		clockTicks = readClockTicksFromAuxv()
	})
	return clockTicks
}

// readClockTicksFromAuxv 从 /proc/self/auxv 解析 AT_CLKTCK。
// auxv 为 (type, value) 的 8 字节对（unsigned long），AT_CLKTCK = 17。
// 兼容小端/大端字节序；解析失败时回退到 100。
func readClockTicksFromAuxv() int64 {
	data, err := os.ReadFile("/proc/self/auxv")
	if err != nil {
		return 100
	}
	const (
		atClktck     = 17
		entrySize    = 16 // 8 字节 type + 8 字节 value
		maxPlausible = 1 << 20
	)
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for i := 0; i+entrySize <= len(data); i += entrySize {
			typ := order.Uint64(data[i : i+8])
			val := order.Uint64(data[i+8 : i+16])
			if typ == atClktck {
				if val > 0 && val < maxPlausible {
					return int64(val)
				}
				return 100
			}
		}
	}
	return 100
}

func readProcUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	sec, _ := strconv.ParseFloat(fields[0], 64)
	return int64(sec)
}

// sendSignal 向指定 PID 的进程发送信号。
// 使用 os.FindProcess + Process.Signal 实现，Linux 目标平台正常工作，且可在开发机跨平台编译。
func sendSignal(pid int, sig syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func formatProcessBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
