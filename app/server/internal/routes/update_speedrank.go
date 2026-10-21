// 更新下载"测速加速"：参考资料 app/bin/gateway-proxy.py 的镜像选型思路
// （gh-proxy.com 最稳；v4.gh-proxy.org 与其同后端只能作节点；ghfast.top 抖动大）。
//
// 本文件只做"候选排序"，不改动任何安全校验：
//   - 候选仍由 apply-safe 的 HTTPS + 宿主白名单校验后才会被真正请求；
//   - 某条候选失败（连接失败 / HTTP 非 200 / 首包超时 / 速度过低）时立即换下一条，
//     因此"慢节点排前面"最多损失一个首包超时（几秒），不会拖垮整个更新。
package routes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type mirrorTier int

const (
	// tierMulti 多轮实测最稳，排最前。
	tierMulti mirrorTier = iota
	// tierNode 与某个 tierMulti 同后端（x-github-request-id / etag / x-cache-hits 一致），
	// 只能作同一后端的节点切换，不是灾备。
	tierNode
	// tierVolatile 独立运营但抖动大（实测 1.8 MB/s ~ 100s 超时）。
	tierVolatile
	// tierUnknown 未测过的镜像：谨慎排在后面，但实测测速可将其提前。
	tierUnknown
	// tierDirect：asset 自身地址，实测常为 0 字节超时，排最后兜底。
	tierDirect
)

type mirrorRank struct {
	tier mirrorTier
}

// mirrorRanks 按域名（必要时按域名+路径）登记实测档次。
// 新增镜像时在这里登记一个档次，未登记的默认 tierUnknown。
var mirrorRanks = map[string]mirrorRank{
	"gh-proxy.com":       {tier: tierMulti},
	"v4.gh-proxy.org":    {tier: tierNode},
	"ghfast.top":         {tier: tierVolatile},
	"ghproxy.imciel.com": {tier: tierVolatile},
}

// rankedMirrorTiers 定义了对外可见的档次顺序，用于日志与诊断。
var rankedMirrorTiers = []struct {
	tier mirrorTier
	name string
}{
	{tierMulti, "multi-round-stable"},
	{tierNode, "same-backend-node"},
	{tierVolatile, "volatile"},
	{tierUnknown, "unknown"},
	{tierDirect, "direct"},
}

func tierName(t mirrorTier) string {
	for _, r := range rankedMirrorTiers {
		if r.tier == t {
			return r.name
		}
	}
	return "unknown"
}

// classifyMirrorURL 返回候选地址的优先级（越小越先试）。
func classifyMirrorURL(rawURL string) mirrorTier {
	u, err := url.Parse(rawURL)
	if err != nil {
		return tierUnknown
	}
	host := strings.ToLower(u.Hostname())
	if rank, ok := mirrorRanks[host]; ok {
		return rank.tier
	}
	// 形如 https://gh-proxy.com/<原始 GitHub 地址>：任何与镜像同域名的子域按父域登记处理
	for domain, rank := range mirrorRanks {
		if strings.HasSuffix(host, "."+domain) {
			return rank.tier
		}
	}
	// 直连 github.com / *.githubusercontent.com 一类的原始地址
	switch {
	case host == "github.com" || host == "api.github.com",
		strings.HasSuffix(host, ".githubusercontent.com"):
		return tierDirect
	}
	return tierUnknown
}

// orderCandidatesForSpeed 按「实测档次 → 原始顺序」稳定排序，同时去重。
// 直连放最后：实测 github.com 可握手但返回 0 字节并超时，放在最前只会白等一个超时。
func orderCandidatesForSpeed(urls []string) []string {
	seen := make(map[string]struct{}, len(urls))
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			continue
		}
		if _, dup := seen[u]; dup {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return classifyMirrorURL(out[i]) < classifyMirrorURL(out[j])
	})
	return out
}

// probeMirror 是"加速探测"的超时参数。取值原则：宁可快速放弃也不让用户干等，
// 失败后立刻换下一条候选（下一条通常仍在同一后端）。
//
// 阈值按"连上之后"的吞吐判断：4G / 跨境链路稳定吞吐常在 0.2~0.5 MB/s，
// 单看绝对速度会误杀可用慢链路，所以过滤条件只有"首包超时 / 低于硬下限 / 采样不足"，
// 真正的快慢比较交给排序（快的排前面，慢的自动靠后而不是被剔除）。
type probeMirror struct {
	connectTimeout time.Duration
	firstByteWait  time.Duration // 首包（含 header）超时
	sampleBytes    int64         // 采样字节数，用于估算 Mbps / 首包耗时
	sampleDeadline time.Duration // 采样最长时间
	minThroughput  float64       // 硬下限（MB/s）：约 64 Mbit/s 的 1/10，明显不可用的源才剔除
}

var defaultProbe = probeMirror{
	connectTimeout: 6000 * time.Millisecond,
	firstByteWait:  6000 * time.Millisecond,
	sampleBytes:    128 * 1024,
	sampleDeadline: 8 * time.Second,
	minThroughput:  0.05,
}

// probeWinner 是探测阶段选出的优等候选：调用方应优先实测下载它，其余按顺序兜底。
type probeWinner struct {
	URL string
	// Ok 为 false 表示"连得上且能出数据，但采样窗口内没测准"（极慢链路）。仍然可用：
	// 此时不叠加首包等待，直接进入真实下载，让真实字节流决定要不要换源。
	Ok         bool
	Throughput float64 // MB/s（Ok=false 时为 0，未知）
	TTFB       time.Duration
}

// rankCandidatesByProbe 并发向各候选发起 Range 探测，返回"探测优等候选"；其余候选由
// 调用方按静态档次顺序兜底。返回 nil 表示所有探测都失败——调用方应直接用静态顺序下载，
// 绝不能因为"测速失败"就判定无法更新（探测只是加速手段，不是可用性前提）。
func rankCandidatesByProbe(ctx context.Context, urls []string) *probeWinner {
	if len(urls) == 0 {
		return nil
	}
	type result struct {
		url        string
		throughput float64
		ttfb       time.Duration
		ok         bool
	}
	results := make([]result, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			mbps, ttfb, err := measureCandidateThroughput(ctx, u)
			if err != nil {
				slog.Info("更新源测速失败", "url", u, "error", err)
				return
			}
			results[i] = result{url: u, throughput: mbps, ttfb: ttfb, ok: true}
		}(i, u)
	}
	wg.Wait()

	best := -1
	for i, r := range results {
		if !r.ok {
			continue
		}
		if r.throughput <= 0 {
			// 未测准：只作为"有可测源"时被淘汰的兜底，不参与打分
			continue
		}
		if best < 0 || probeScore(r.throughput, r.ttfb) < probeScore(results[best].throughput, results[best].ttfb) {
			best = i
		}
	}
	if best >= 0 {
		w := results[best]
		slog.Info("更新源测速结果", "winner", w.url, "throughputMBps", fmt.Sprintf("%.2f", w.throughput),
			"ttfbMs", w.ttfb.Milliseconds(), "score", fmt.Sprintf("%.3f", probeScore(w.throughput, w.ttfb)))
		return &probeWinner{URL: w.url, Ok: true, Throughput: w.throughput, TTFB: w.ttfb}
	}
	// 没有可打分的源：退而求其次，挑第一个"连得上且出过数据"的候选
	for _, r := range results {
		if r.ok {
			slog.Info("更新源测速未测准，改用可用兜底源", "url", r.url, "ttfbMs", r.ttfb.Milliseconds())
			return &probeWinner{URL: r.url, Ok: false, TTFB: r.ttfb}
		}
	}
	return nil
}

// probeScore 是"估计下载 1MB 所需秒数"：首包耗时 + 1MB/吞吐。
// 只用吞吐排序会偏爱"一上来很快、随后限速"的镜像：它们在小样本窗口里吞吐很高，
// 完整下载 20MB+ 时却可能比稳态源还慢；首包慢的源同样要先付出握手与排队延迟。
// 阈值只用于剔除明显不可用的源（见 minThroughput），不做"慢节点降级"：
// 实测四家镜像吞吐区间互相交叉，固定比例阈值会随网络抖动误杀。
func probeScore(throughput float64, ttfb time.Duration) float64 {
	if throughput <= 0 {
		return 1e9
	}
	return ttfb.Seconds() + 1.0/throughput
}

// measureCandidateThroughput 探测单个候选：返回 (吞吐 MB/s, nil) 表示可用；
// 返回 0 且 err==nil 表示"连得上但没测准"，保留为队尾兜底；返回 err 表示剔除。
func measureCandidateThroughput(ctx context.Context, rawURL string) (float64, time.Duration, error) {
	var ttfb time.Duration
	if !isHTTPSURL(rawURL) || !isAllowedHost(rawURL) {
		return 0, ttfb, fmt.Errorf("地址不安全")
	}
	p := defaultProbe
	// 探测不做证书豁免：代理同样走 HTTPS，失败就换下一个候选。
	client := &http.Client{
		Timeout: p.connectTimeout + p.firstByteWait + p.sampleDeadline,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 2 {
				return fmt.Errorf("重定向次数过多")
			}
			if !isHTTPSURL(req.URL.String()) || !isAllowedHost(req.URL.String()) {
				return fmt.Errorf("重定向地址不安全")
			}
			return nil
		},
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.connectTimeout+p.firstByteWait+p.sampleDeadline)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, ttfb, err
	}
	req.Header.Set("User-Agent", "logmanager-updater/1.0")
	// 只取头部若干字节，不把整包拉下来
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", p.sampleBytes-1))

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, ttfb, err
	}
	defer resp.Body.Close()

	ttfb = time.Since(start)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, ttfb, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if ttfb > p.firstByteWait {
		return 0, ttfb, fmt.Errorf("首包超时 %v", ttfb)
	}

	// 吞吐按"首包之后的字节/耗时"估算：握手与重定向耗时不计入，
	// 避免首包慢但带宽足的节点被误判成慢节点。
	br := bufio.NewReaderSize(resp.Body, 32*1024)
	buf := make([]byte, 32*1024)
	var read int64
	var readStart time.Time
	deadline := time.Now().Add(p.sampleDeadline)
	for read < p.sampleBytes && time.Now().Before(deadline) {
		n, readErr := br.Read(buf)
		if n > 0 {
			if readStart.IsZero() {
				readStart = time.Now()
			}
			read += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				break
			}
			return 0, ttfb, readErr
		}
	}
	if read == 0 {
		return 0, ttfb, fmt.Errorf("无数据")
	}
	if readStart.IsZero() {
		return 0, ttfb, nil
	}
	elapsed := time.Since(readStart).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	mbps := float64(read) / elapsed / (1024 * 1024)
	if read < p.sampleBytes {
		// 采样不足：按实际速度判断是否"明显不可用"，可用则返回 0（排到队尾作兜底）
		if mbps >= p.minThroughput {
			slog.Info("更新源采样不足但可用，作为兜底", "url", rawURL, "bytes", read, "throughputMBps", fmt.Sprintf("%.2f", mbps))
			return 0, ttfb, nil
		}
		return 0, ttfb, fmt.Errorf("速度过低 %.2f MB/s（%d 字节）", mbps, read)
	}
	if mbps < p.minThroughput {
		return 0, ttfb, fmt.Errorf("速度过低 %.2f MB/s", mbps)
	}
	return mbps, ttfb, nil
}

// ---------------------------------------------------------------------------
// 实际下载阶段：领先者直出 + 双源对冲
// ---------------------------------------------------------------------------
//
// 参考资料里最有说服力的一条实测是"按吞吐排序会骗人"：ghfast.top 在小样本窗口
// 0.10 MB/s 领先，但大包实测抖成 HTTP 499 / 0 字节超时；gh-proxy.com 窗口 0.09 MB/s
// 却全程 0.95~6.4 MB/s 最稳。所以探测只能决定"谁先试"，真正的胜负必须由真实字节流裁决。
//
// 策略：
//   1) 领先候选先跑 hedgeLeadDelay，期间若吞吐低于 hedgeMinRate 或停滞，
//      就认为它"起步快后限速"，转入对冲（此时备选连接已提前建立，省掉握手/排队）。
//   2) 对冲进行 hedgeEvalWindow 后比较双方速率，明显更慢的一方退出；
//      领先者被换下后，其已下载前缀保留，备选下载完成后截断到该长度再拼接，
//      因此总进度从不回退，用户看到的是"持续前进"。
//   3) 所有源都失败才报错，与改造前的失败语义一致（只是更快失败、更快切换）。

// transferTuning 汇总下载阶段的时间参数（单测可直接覆盖）。
type transferTuning struct {
	leadDelay     time.Duration // 领先者独占窗口
	minRate       float64       // 领先者窗口内低于此速率（MB/s）即提前对冲
	evalWindow    time.Duration // 双边同时下载的判定窗口
	killRatio     float64       // 落后方速率 < 领先方 × killRatio 即退出
	hedgeStall    time.Duration // 任一路连续无进展即视为失败
	maxHedgePeers int
}

var defaultTransfer = transferTuning{
	leadDelay:     4 * time.Second,
	minRate:       0.20,
	evalWindow:    6 * time.Second,
	killRatio:     0.7,
	hedgeStall:    30 * time.Second,
	maxHedgePeers: 1,
}

// speedMeter 是下载过程中的速率计量（滑动窗口，避免用"平均速率"误判先快后慢）。
type speedMeter struct {
	mu       sync.Mutex
	samples  []speedSample
	lastByte time.Time
	total    int64
}

type speedSample struct {
	at   time.Time
	byte int64
}

func newSpeedMeter() *speedMeter {
	return &speedMeter{lastByte: time.Now()}
}

func (m *speedMeter) add(n int64) {
	m.mu.Lock()
	m.total += n
	m.lastByte = time.Now()
	m.samples = append(m.samples, speedSample{at: m.lastByte, byte: m.total})
	// 只保留最近 10s 的样本
	cut := m.lastByte.Add(-10 * time.Second)
	for len(m.samples) > 2 && m.samples[0].at.Before(cut) {
		m.samples = m.samples[1:]
	}
	m.mu.Unlock()
}

func (m *speedMeter) bytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.total
}

func (m *speedMeter) since() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.lastByte)
}

// rateMBps 返回滑动窗口内的瞬时吞吐（MB/s）。
func (m *speedMeter) rateMBps(window time.Duration) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.samples) < 2 {
		return 0
	}
	last := m.samples[len(m.samples)-1]
	first := m.samples[0]
	for _, s := range m.samples {
		if last.at.Sub(s.at) <= window {
			first = s
			break
		}
	}
	span := last.at.Sub(first.at).Seconds()
	if span <= 0 {
		return 0
	}
	return float64(last.byte-first.byte) / span / (1024 * 1024)
}

// probeSpeed 做一次默认参数的并发测速，仅用于日志/单测；下载阶段不依赖它。
func probeSpeed(ctx context.Context, urls []string) *probeWinner {
	return rankCandidatesByProbe(ctx, urls)
}
