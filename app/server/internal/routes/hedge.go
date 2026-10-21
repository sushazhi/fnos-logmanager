package routes

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
)

// hedge.go —— "领先者直出 + 双源对冲"下载引擎
//
// 动机（参考 fnos-qbittorrent 的 gateway-proxy.py 实测结论）：小样本测速会骗人。
// 某个镜像窗口吞吐更高，但完整下载 20MB+ 时可能突然 499 / 0 字节超时；
// 另一个窗口吞吐略低却全程稳定。因此：
//   - 测速只决定"谁先试"（见 update_speedrank.go）；
//   - 真实胜负由真实字节流裁决：领先者跑满 leadDelay 后若速率不足，
//     立刻让备选源并行起跑；双方同时下载 evalWindow 后，明显更慢的一方退出。
//
// 不变量（改代码前请先读）：
//  1. 文件句柄只有 copyLeg 一个所有者（它负责关闭）；连接由 cancel 关闭。
//     引擎里的任何返回路径都不得直接关闭 leg 的文件，避免 Windows 重复关闭复用句柄。
//  2. 每个启动过的 goroutine 都必须在所有返回路径上被回收（读 finished 通道），
//     否则 globalUpdateState.updating 会永久为 true，之后所有更新请求都 409。
//  3. 续传必须真续传：服务器忽略 Range 时报错换源，绝不把整包拼到偏移之后
//     （否则 dest 变成 prefix+整包，长度翻倍但内容错位）。
//  4. dest 只在"确实要重新下载"时截断；一个已完整落盘的 dest 不能被后续失败路径抹掉。

// ---------------------------------------------------------------------------
// 传输抽象
// ---------------------------------------------------------------------------

// httpTransfer 是一次正在进行、可中途放弃的 HTTP 下载。
// Close 的契约：调用后 Body 的 Read 必须尽快返回错误（net/http 满足：
// 关闭 Response.Body 会让阻塞中的 Read 返回错误）。Close 绝不关闭文件。
type httpTransfer interface {
	URL() string
	Body() io.Reader
	Total() int64
	Close()
}

// transferOpener 打开一条下载支路。offset>0 时必须拿到 206 续传响应，
// 服务器若忽略 Range 返回 200，则视为该支路不可用于续传（见 openHTTPTransfer）。
type transferOpener func(url string, offset int64) (httpTransfer, error)

func openHTTPTransfer(rawURL string, offset int64) (httpTransfer, error) {
	if !isHTTPSURL(rawURL) {
		return nil, fmt.Errorf("仅允许HTTPS下载")
	}
	if !isAllowedHost(rawURL) {
		return nil, fmt.Errorf("下载地址不在允许列表")
	}
	client := &http.Client{
		Timeout: time.Duration(config.Get().Update.DownloadTimeoutMs) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > config.Get().Update.MaxRedirects {
				return fmt.Errorf("重定向次数过多")
			}
			if !isHTTPSURL(req.URL.String()) || !isAllowedHost(req.URL.String()) {
				return fmt.Errorf("重定向地址不安全")
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "logmanager-updater/1.0")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if offset > 0 {
			resp.Body.Close()
			return nil, fmt.Errorf("服务器忽略 Range（期望 206，实际 200）")
		}
	case http.StatusPartialContent:
		if offset <= 0 {
			resp.Body.Close()
			return nil, fmt.Errorf("意外返回 206")
		}
		want := fmt.Sprintf("bytes %d-", offset)
		if cr := resp.Header.Get("Content-Range"); cr != "" && !hasPrefixFold(cr, want) {
			resp.Body.Close()
			return nil, fmt.Errorf("续传起始位置不符: %s", cr)
		}
	case http.StatusRequestedRangeNotSatisfiable:
		resp.Body.Close()
		// 416 只在"续传起点已在文件末尾"时才是成功语义；offset==0 时报 416
		// 说明源本身异常，绝不能当成"0 字节下载成功"。
		if offset <= 0 {
			return nil, fmt.Errorf("源返回 HTTP 416")
		}
		return &emptyTransfer{url: rawURL, total: offset}, nil
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total >= 0 && offset > 0 {
		total += offset
	}
	return &httpTransferImpl{url: rawURL, body: resp.Body, total: total, closer: resp.Body}, nil
}

type httpTransferImpl struct {
	url    string
	body   io.Reader
	total  int64
	closer io.Closer
}

func (t *httpTransferImpl) URL() string     { return t.url }
func (t *httpTransferImpl) Body() io.Reader { return t.body }
func (t *httpTransferImpl) Total() int64    { return t.total }
func (t *httpTransferImpl) Close() {
	if t.closer != nil {
		t.closer.Close()
	}
}

// emptyTransfer 表示"续传起点已在文件末尾"：立即 EOF，但总量已知。
type emptyTransfer struct {
	url   string
	total int64
}

func (t *emptyTransfer) URL() string     { return t.url }
func (t *emptyTransfer) Body() io.Reader { return strings.NewReader("") }
func (t *emptyTransfer) Total() int64    { return t.total }
func (t *emptyTransfer) Close()          {}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && equalFoldASCII(s[:len(prefix)], prefix)
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'a' <= ca && ca <= 'z' {
			ca -= 32
		}
		if 'a' <= cb && cb <= 'z' {
			cb -= 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// 进度
// ---------------------------------------------------------------------------

// progress 记录"已确认写入 dest 的字节数"。多支路并发调用，自身带锁，
// 并在对象内部强制单调不减：迟到的较小值直接丢弃，调用方无需再做钳制。
type progress struct {
	mu        sync.Mutex
	committed int64
	total     int64
	reported  int64 // 已对外报告过的字节数下限
	onChange  func(float64)
}

func newProgress(onChange func(float64)) *progress {
	return &progress{onChange: onChange}
}

func (p *progress) setTotal(t int64) {
	if t <= 0 {
		return
	}
	p.mu.Lock()
	p.total = t
	p.mu.Unlock()
}

func (p *progress) add(n int64) {
	p.mu.Lock()
	p.committed += n
	cur := p.committed
	p.mu.Unlock()
	p.report(cur)
}

// set 直接把"已确认字节数"设为文件实际长度（拼接完成后校正用）。
func (p *progress) set(n int64) {
	p.mu.Lock()
	p.committed = n
	p.mu.Unlock()
	p.report(n)
}

func (p *progress) get() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.committed
}

// report 取快照后再回调（绝不持锁执行回调），保证对外进度单调不减。
func (p *progress) report(cur int64) {
	p.mu.Lock()
	if cur < p.reported {
		p.mu.Unlock()
		return
	}
	p.reported = cur
	total := p.total
	p.mu.Unlock()
	if p.onChange == nil || total <= 0 {
		return
	}
	if cur > total {
		cur = total
	}
	p.onChange(float64(cur) / float64(total))
}

// ---------------------------------------------------------------------------
// 引擎
// ---------------------------------------------------------------------------

type hedgeEngine struct {
	ordered    []string
	dest       string
	tune       transferTuning
	onProgress func(float64)
	open       transferOpener
	now        func() time.Time
}

// run 按顺序尝试各候选，任一源完整下完即返回 nil。
func (e *hedgeEngine) run(leadDelay time.Duration) error {
	next := 0
	var lastErr error
	for next < len(e.ordered) {
		idx, err := e.attempt(next, leadDelay)
		if err == nil {
			return nil
		}
		lastErr = err
		slog.Warn("该源不可用，换下一个源", "url", e.ordered[idx], "error", err)
		if idx < next {
			idx = next
		}
		next = idx + 1
		leadDelay = 0
		e.removeTemps()
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的下载地址")
	}
	return lastErr
}

func (e *hedgeEngine) removeTemps() {
	for i := range e.ordered {
		os.Remove(e.dest + fmt.Sprintf(".alt%d", i))
	}
}

// attempt 从 ordered[start] 开始：领先者独占 leadDelay，必要时对冲。
// 返回 (最终尝试到的索引, error)；err==nil 表示 dest 已完整落盘。
func (e *hedgeEngine) attempt(start int, leadDelay time.Duration) (int, error) {
	maxBytes := config.Get().Update.MaxDownloadBytes
	prog := newProgress(e.onProgress)

	mainURL := e.ordered[start]
	mainTr, err := e.open(mainURL, 0)
	if err != nil {
		return start, err
	}
	prog.setTotal(mainTr.Total())

	// 拿到第一个响应后才截断 dest：连不上的源不该毁掉可能已经完整的旧文件。
	destFile, err := os.OpenFile(e.dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		mainTr.Close()
		return start, err
	}

	mainLeg := newLeg(mainURL, mainTr, destFile, "", 0, e.now())
	go mainLeg.pump(e, prog, maxBytes)
	slog.Info("开始下载", "url", mainURL, "leadDelayMs", leadDelay.Milliseconds(), "total", mainTr.Total())

	// ---- 阶段 1：领先者独占 leadDelay ----
	// "先直连，超时再加速"：直连独占窗口内若一直没字节（首包超时）或明显跑不动，
	// 不必等满窗口，立刻进入阶段 2 让加速源并行对冲。
	if leadDelay > 0 {
		hasPeer := start+1 < len(e.ordered)
		t := time.NewTimer(leadDelay)
		var watch *time.Ticker
		var watchC <-chan time.Time
		if hasPeer {
			watch = time.NewTicker(500 * time.Millisecond)
			watchC = watch.C
		}
		leadDeadline := e.now().Add(leadDelay)
	waitLead:
		for {
			select {
			case err := <-mainLeg.finished:
				t.Stop()
				if watch != nil {
					watch.Stop()
				}
				return start, err
			case <-t.C:
				break waitLead
			case <-watchC:
				idle := e.now().Sub(time.Unix(0, mainLeg.lastRead.Load()))
				rate := mainLeg.meter.rateMBps(3 * time.Second)
				// 首包超时：连上了却一直 0 字节（跨境链路被丢弃时的典型表现）
				stalled := mainLeg.meter.bytes() == 0 && idle > e.tune.firstByteWait
				// 有字节但远低于可用下限，且窗口已过半：认定直连被限速
				slow := mainLeg.meter.bytes() > 0 && idle < e.tune.hedgeStall &&
					rate < e.tune.minRate && e.now().After(leadDeadline.Add(-leadDelay/2))
				if stalled || slow {
					slog.Info("直连未按时出数据/过慢，提前启用加速源", "url", mainURL,
						"idleMs", idle.Milliseconds(), "rateMBps", fmt.Sprintf("%.2f", rate),
						"stalled", stalled, "slow", slow)
					break waitLead
				}
			}
		}
		t.Stop()
		if watch != nil {
			watch.Stop()
		}
		// 计时器与"下载刚好完成"可能同时就绪：优先认完成，别把快下完的源当慢源换掉。
		select {
		case err := <-mainLeg.finished:
			return start, err
		default:
		}
	}

	// ---- 阶段 2：开启对冲（若有备选） ----
	peerIdx := start + 1
	if peerIdx >= len(e.ordered) {
		return start, <-mainLeg.finished
	}

	offset := prog.get()
	peerPath := e.dest + fmt.Sprintf(".alt%d", peerIdx)
	peerFile, ferr := os.OpenFile(peerPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	var peerTr httpTransfer
	var oerr error
	if ferr == nil {
		peerTr, oerr = e.open(e.ordered[peerIdx], offset)
	} else {
		oerr = ferr
	}
	if oerr != nil {
		if peerFile != nil {
			peerFile.Close()
		}
		os.Remove(peerPath)
		slog.Warn("备选源不可用，继续等待领先者", "url", e.ordered[peerIdx], "error", oerr)
		return start, <-mainLeg.finished
	}
	prog.setTotal(peerTr.Total())
	peerLeg := newLeg(e.ordered[peerIdx], peerTr, peerFile, peerPath, offset, e.now())
	// 对冲源进度不计入对外进度：只有真正拼进 dest 后，进度才由 appendTempToDest 校正。
	go peerLeg.pump(e, newProgress(nil), maxBytes)
	slog.Info("领先者速率不足，开启对冲源", "lead", mainURL,
		"leadRateMBps", fmt.Sprintf("%.2f", mainLeg.meter.rateMBps(3*time.Second)),
		"peer", peerLeg.url, "offset", offset)

	// ---- 阶段 3：双边同时跑 evalWindow，由真实速率裁决 ----
	return e.decide(start, peerIdx, mainLeg, peerLeg, prog, offset, peerPath)
}

// decide 在对冲阶段挑选胜者，且保证两条支路的 goroutine 都被回收。
func (e *hedgeEngine) decide(start, peerIdx int, mainLeg, peerLeg *leg, prog *progress, offset int64, peerPath string) (int, error) {
	evalTimer := time.NewTimer(e.tune.evalWindow)
	defer evalTimer.Stop()

	for {
		select {
		case mainErr := <-mainLeg.finished:
			// 领先者结束：对冲源可能已经下完剩余部分，能拼接就拼接。
			peerErr := <-peerLeg.finished
			if mainErr == nil {
				os.Remove(peerPath)
				return start, nil
			}
			if peerErr == nil && peerLeg.meter.bytes() > 0 {
				if err := appendTempToDest(e.dest, peerPath, offset, prog); err != nil {
					return peerIdx, err
				}
				os.Remove(peerPath) // 必须在对冲支路结束（fd 已关）之后删除
				slog.Info("领先者中断，已由对冲源补齐", "url", peerLeg.url, "leadError", mainErr)
				return peerIdx, nil
			}
			os.Remove(peerPath)
			return peerIdx, mainErr

		case peerErr := <-peerLeg.finished:
			if peerErr == nil && peerLeg.meter.bytes() > 0 {
				// 对冲源已完整下完（EOF）。它一定比仍在传输的领先者更接近完成：
				// 取消领先者（可能正卡在半死连接上）后拼接对冲结果。
				// 注意：这里绝不能再"裸等"领先者——它可能永远不结束。
				mainLeg.cancel()
				<-mainLeg.finished
				if err := appendTempToDest(e.dest, peerPath, offset, prog); err != nil {
					return peerIdx, err
				}
				os.Remove(peerPath)
				slog.Info("对冲源已完成，切换到对冲源", "peer", peerLeg.url,
					"peerBytes", peerLeg.meter.bytes())
				return peerIdx, nil
			}
			// 对冲源失败（或空）：以领先者为准，等它跑完
			mainErr := <-mainLeg.finished
			os.Remove(peerPath)
			if mainErr != nil {
				return peerIdx, fmt.Errorf("领先者: %v; 对冲源: %v", mainErr, peerErr)
			}
			return start, nil

		case <-evalTimer.C:
			mainRate := mainLeg.meter.rateMBps(e.tune.evalWindow)
			peerRate := peerLeg.meter.rateMBps(e.tune.evalWindow)
			leaderStalled := mainRate < e.tune.minRate && peerLeg.meter.bytes() > 0
			peerClearlyFaster := peerRate > 0 && (mainRate <= 0 || peerRate >= mainRate/e.tune.killRatio)

			if mainRate <= 0 && peerRate <= 0 {
				// 双边都无字节：都取消并换下一个候选，避免无限等待
				slog.Warn("双边均无进展，放弃该组合")
				mainLeg.cancel()
				peerLeg.cancel()
				mainErr := <-mainLeg.finished
				peerErr := <-peerLeg.finished
				os.Remove(peerPath)
				return peerIdx, fmt.Errorf("领先者与对冲源均无进展: %v / %v", mainErr, peerErr)
			}
			if leaderStalled || peerClearlyFaster {
				slog.Info("领先者过慢，切换到对冲源", "mainRateMBps", fmt.Sprintf("%.2f", mainRate),
					"peerRateMBps", fmt.Sprintf("%.2f", peerRate))
				mainLeg.cancel()
				<-mainLeg.finished
				if peerLeg.done() {
					peerErr := <-peerLeg.finished
					if peerErr != nil {
						os.Remove(peerPath)
						return peerIdx, peerErr
					}
				} else {
					// 对冲源还在跑：等它把剩余部分下完
					if peerErr := <-peerLeg.finished; peerErr != nil {
						os.Remove(peerPath)
						return peerIdx, peerErr
					}
				}
				if err := appendTempToDest(e.dest, peerPath, offset, prog); err != nil {
					return peerIdx, err
				}
				os.Remove(peerPath)
				return peerIdx, nil
			}

			slog.Info("领先者仍占优，关闭对冲源", "mainRateMBps", fmt.Sprintf("%.2f", mainRate),
				"peerRateMBps", fmt.Sprintf("%.2f", peerRate))
			peerLeg.cancel()
			<-peerLeg.finished
			os.Remove(peerPath)
			return start, <-mainLeg.finished
		}
	}
}

// leg 是一条下载支路。文件句柄只由 copy() 关闭；连接只由 cancel() 关闭。
type leg struct {
	url      string
	tr       httpTransfer
	file     *os.File
	tmpPath  string // 非空表示写入独立临时文件（对冲支路）
	base     int64  // 已确认前缀长度（续传起点），用于总量上限判断
	finished chan error
	meter    *speedMeter

	closed   atomic.Bool
	lastRead atomic.Int64 // UnixNano：最后一个字节到达时间（watchdog 独占读取）
}

func newLeg(url string, tr httpTransfer, file *os.File, tmpPath string, base int64, now time.Time) *leg {
	l := &leg{url: url, tr: tr, file: file, tmpPath: tmpPath, base: base,
		finished: make(chan error, 1), meter: newSpeedMeter()}
	l.lastRead.Store(now.UnixNano())
	return l
}

// done 报告该支路是否已结束（非阻塞）。
func (l *leg) done() bool {
	select {
	case err := <-l.finished:
		// 读完即说明已结束；把结果放回去给后续读取方
		l.finished <- err
		return true
	default:
		return false
	}
}

// cancel 请求中止该支路：置标志让 watchdog 退出，再关连接让阻塞中的 Read 返回。
func (l *leg) cancel() {
	if l.closed.Swap(true) {
		return
	}
	l.tr.Close()
}

// pump 运行支路；子 goroutine 里的 panic 不受上层 recover 保护，这里必须自兜。
func (l *leg) pump(e *hedgeEngine, prog *progress, maxBytes int64) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("下载支路 panic", "url", l.url, "recover", r)
			l.finished <- fmt.Errorf("下载支路异常: %v", r)
		}
	}()
	l.finished <- l.copy(e, prog, maxBytes)
}

// copy 把支路内容写入文件；返回 nil 表示完整读到 EOF。
func (l *leg) copy(e *hedgeEngine, prog *progress, maxBytes int64) error {
	defer func() {
		l.closed.Store(true) // 自行结束也算已关闭，让 watchdog 尽快退出
		if l.file != nil {
			l.file.Close() // 文件唯一所有者
		}
		l.tr.Close()
	}()

	// 停滞看门狗：Read 是阻塞的，必须由独立 goroutine 在超时后主动断连接，
	// 否则一个假死的 socket 会把整个下载拖到客户端超时（实测镜像会这样）。
	watchdogDone := make(chan struct{})
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		stall := e.tune.hedgeStall
		for {
			select {
			case <-watchdogDone:
				return
			case <-t.C:
				if l.closed.Load() {
					return
				}
				if e.now().Sub(time.Unix(0, l.lastRead.Load())) > stall {
					slog.Warn("下载停滞，断开该源", "url", l.url, "stall", stall)
					l.cancel()
					return
				}
			}
		}
	}()
	defer close(watchdogDone)

	buf := make([]byte, 32*1024)
	reader := l.tr.Body()
	var written int64
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			l.lastRead.Store(e.now().UnixNano())
			if _, werr := l.file.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			l.meter.add(int64(n))
			if l.tmpPath == "" {
				prog.add(int64(n))
			}
			if l.base+written > maxBytes {
				return fmt.Errorf("下载内容超过大小限制")
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// appendTempToDest 把临时文件内容拼接进 dest（先截断到 base），并按实际长度校正进度。
func appendTempToDest(dest, tmpPath string, base int64, prog *progress) error {
	src, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()
	if err := dst.Truncate(base); err != nil {
		return err
	}
	if _, err := dst.Seek(base, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	if fi, err := dst.Stat(); err == nil {
		prog.set(fi.Size())
	}
	return nil
}
