package routes

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeSource 模拟一个镜像：可控制"起步快后限速""中途假死""连不上""忽略 Range"等真实故障。
type fakeSource struct {
	payload []byte
	// delayFor 返回每块数据之前的等待时长，用于模拟限速 / 假死
	delayFor func(chunk int) time.Duration
	failAt   int64 // >0 时读到该字节数后返回错误（模拟连接被掐断）
	openErr  error
	// ignoreRange 为 true 时无视 Range 返回整包（模拟劣质镜像），用于验证不会被拼坏
	ignoreRange bool
	opened      atomic.Int32
}

type fakeTransfer struct {
	src          *fakeSource
	reader       io.Reader
	url          string
	stop         chan struct{}
	closed       atomic.Bool
	servedOffset int64
}

func (t *fakeTransfer) URL() string     { return t.url }
func (t *fakeTransfer) Body() io.Reader { return t.reader }
func (t *fakeTransfer) Total() int64    { return int64(len(t.src.payload)) }
func (t *fakeTransfer) Close() {
	if t.closed.Swap(true) {
		return
	}
	close(t.stop) // 让阻塞中的 Read 立刻返回错误（等价于 net/http 关闭 Body）
}

func (t *fakeTransfer) stopped() bool {
	select {
	case <-t.stop:
		return true
	default:
		return false
	}
}

// fakeReader 按 fakeSource 描述吐数据，支持可打断的等待与连接关闭。
type fakeReader struct {
	tr    *fakeTransfer
	src   *fakeSource
	data  []byte
	off   int
	chunk int
}

func (r *fakeReader) Read(p []byte) (int, error) {
	if r.tr.stopped() {
		return 0, errors.New("use of closed connection")
	}
	if int64(r.off) >= int64(len(r.data)) {
		return 0, io.EOF
	}
	if r.src.failAt > 0 && int64(r.off) >= r.src.failAt {
		return 0, errors.New("connection reset")
	}
	if r.src.delayFor != nil {
		if d := r.src.delayFor(r.chunk); d > 0 {
			deadline := time.Now().Add(d)
			for time.Now().Before(deadline) {
				if r.tr.stopped() {
					return 0, errors.New("use of closed connection")
				}
				time.Sleep(2 * time.Millisecond)
			}
		}
		r.chunk++
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}

func newOpener(t *testing.T, sources map[string]*fakeSource) transferOpener {
	t.Helper()
	return func(url string, offset int64) (httpTransfer, error) {
		src, ok := sources[url]
		if !ok {
			return nil, errors.New("unknown url " + url)
		}
		if src.openErr != nil {
			return nil, src.openErr
		}
		// 与 openHTTPTransfer 同语义：offset>0 且服务器忽略 Range → 不可用于续传
		if offset > 0 && src.ignoreRange {
			return nil, errors.New("服务器忽略 Range（期望 206，实际 200）")
		}
		src.opened.Add(1)
		if offset < 0 {
			offset = 0
		}
		if offset > int64(len(src.payload)) {
			offset = int64(len(src.payload))
		}
		tr := &fakeTransfer{src: src, url: url, stop: make(chan struct{}), servedOffset: offset}
		tr.reader = &fakeReader{tr: tr, src: src, data: src.payload[offset:]}
		return tr, nil
	}
}

func testPayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + 3)
	}
	return b
}

func newEngine(t *testing.T, ordered []string, dest string, open transferOpener) (*hedgeEngine, *[]float64) {
	t.Helper()
	progress := &[]float64{}
	e := &hedgeEngine{
		ordered: ordered,
		dest:    dest,
		tune: transferTuning{
			leadDelay:     80 * time.Millisecond,
			minRate:       0.05,
			evalWindow:    120 * time.Millisecond,
			killRatio:     0.7,
			hedgeStall:    2 * time.Second,
			firstByteWait: 500 * time.Millisecond,
			maxHedgePeers: 1,
		},
		onProgress: func(p float64) { *progress = append(*progress, p) },
		open:       open,
		now:        time.Now,
	}
	return e, progress
}

func assertFileEquals(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("size mismatch: got %d want %d", len(got), len(want))
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch")
	}
}

func assertNoTempFiles(t *testing.T, dest string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Dir(dest))
	for _, ent := range entries {
		if strings.Contains(ent.Name(), ".alt") {
			t.Fatalf("leftover temp file: %s", ent.Name())
		}
	}
}

func TestHedgeSingleSourceCompletes(t *testing.T) {
	payload := testPayload(256 * 1024)
	src := &fakeSource{payload: payload}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, prog := newEngine(t, []string{"https://a"}, dest, newOpener(t, map[string]*fakeSource{"https://a": src}))
	if err := e.run(e.tune.leadDelay); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileEquals(t, dest, payload)
	last := -1.0
	for _, p := range *prog {
		if p < last-1e-9 {
			t.Fatalf("progress went backwards: %v", *prog)
		}
		last = p
	}
	if last < 0.99 {
		t.Fatalf("progress never reached 1.0: %v", *prog)
	}
}

func TestHedgeLeadOpenFailureFailsOver(t *testing.T) {
	payload := testPayload(200 * 1024)
	dead := &fakeSource{payload: payload, openErr: errors.New("TLS handshake timeout")}
	good := &fakeSource{payload: payload}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, _ := newEngine(t, []string{"https://dead", "https://good"}, dest,
		newOpener(t, map[string]*fakeSource{"https://dead": dead, "https://good": good}))
	if err := e.run(e.tune.leadDelay); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileEquals(t, dest, payload)
}

func TestHedgeLeadStallsAfterPartialTransfer(t *testing.T) {
	payload := testPayload(400 * 1024)
	// 领先者吐两块后彻底停住（连接在、但不再出字节）——实测镜像的典型故障形态。
	lead := &fakeSource{payload: payload, delayFor: func(chunk int) time.Duration {
		if chunk > 1 {
			return time.Hour
		}
		return 0
	}}
	peer := &fakeSource{payload: payload}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, prog := newEngine(t, []string{"https://lead", "https://peer"}, dest,
		newOpener(t, map[string]*fakeSource{"https://lead": lead, "https://peer": peer}))
	e.tune.hedgeStall = 3 * time.Second
	e.tune.evalWindow = 150 * time.Millisecond

	done := make(chan error, 1)
	go func() { done <- e.run(e.tune.leadDelay) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("engine hung on stalled source (watchdog or goroutine reaping broken)")
	}
	assertFileEquals(t, dest, payload)
	assertNoTempFiles(t, dest)
	for i := 1; i < len(*prog); i++ {
		if (*prog)[i] < (*prog)[i-1]-1e-9 {
			t.Fatalf("progress went backwards: %v", *prog)
		}
	}
}

// 关键回归：对冲源先 EOF（领先者半死但永不结束）时必须能自救，不得"裸等"领先者，
// 也不得在领先者仍在写公共文件时拼接——否则 dest 长度翻倍或内容错位。
func TestHedgePeerFinishesFirstDoesNotHangOrCorrupt(t *testing.T) {
	payload := testPayload(300 * 1024)
	lead := &fakeSource{payload: payload, delayFor: func(chunk int) time.Duration {
		if chunk > 1 {
			return 5 * time.Second
		}
		return 0
	}}
	peer := &fakeSource{payload: payload}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, _ := newEngine(t, []string{"https://lead", "https://peer"}, dest,
		newOpener(t, map[string]*fakeSource{"https://lead": lead, "https://peer": peer}))
	e.tune.hedgeStall = time.Hour
	e.tune.evalWindow = time.Hour // 评估窗口永不到达：必须靠 peer-first 分支自救
	done := make(chan error, 1)
	go func() { done <- e.run(e.tune.leadDelay) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("peer-first path hung: lead source is never cancelled")
	}
	assertFileEquals(t, dest, payload)
	assertNoTempFiles(t, dest)
}

// 服务器忽略 Range 时不得把整包拼到偏移之后（否则 dest = prefix + 整包，长度翻倍）。
func TestHedgeRangeIgnoredIsRejectedNotCorrupted(t *testing.T) {
	payload := testPayload(256 * 1024)
	lead := &fakeSource{payload: payload, delayFor: func(chunk int) time.Duration {
		if chunk > 1 {
			return time.Hour
		}
		return 0
	}}
	peer := &fakeSource{payload: payload, ignoreRange: true}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, _ := newEngine(t, []string{"https://lead", "https://peer"}, dest,
		newOpener(t, map[string]*fakeSource{"https://lead": lead, "https://peer": peer}))
	e.tune.hedgeStall = 2 * time.Second
	e.tune.evalWindow = 100 * time.Millisecond
	_ = e.run(e.tune.leadDelay)
	if fi, err := os.Stat(dest); err == nil && fi.Size() > int64(len(payload)) {
		t.Fatalf("dest grew beyond payload: %d > %d (Range ignored not handled)", fi.Size(), len(payload))
	}
}

func TestHedgeAllSourcesFailReturnsError(t *testing.T) {
	payload := testPayload(64 * 1024)
	a := &fakeSource{payload: payload, openErr: errors.New("boom")}
	b := &fakeSource{payload: payload, failAt: 1024}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, _ := newEngine(t, []string{"https://a", "https://b"}, dest,
		newOpener(t, map[string]*fakeSource{"https://a": a, "https://b": b}))
	if err := e.run(e.tune.leadDelay); err == nil {
		t.Fatal("expected error when every source fails")
	}
	assertNoTempFiles(t, dest)
}

// "先直连，超时再加速"的核心行为：直连连上了但一直不出首包（跨境被丢包的典型形态），
// 必须在 firstByteWait 后立刻启用加速源，而不是苦等整个 leadDelay。
func TestHedgeDirectSilentFailsOverAfterFirstByteWait(t *testing.T) {
	payload := testPayload(256 * 1024)
	// 直连：连接建立成功，但一个字节都不吐（模拟被丢弃/黑洞的连接）
	silent := &fakeSource{payload: payload, delayFor: func(chunk int) time.Duration { return time.Hour }}
	mirror := &fakeSource{payload: payload}
	dest := filepath.Join(t.TempDir(), "out.bin")
	e, _ := newEngine(t, []string{"https://direct", "https://mirror"}, dest,
		newOpener(t, map[string]*fakeSource{"https://direct": silent, "https://mirror": mirror}))
	// 直连独占窗口故意给得很长：切换必须由 firstByteWait 触发，而不是等满窗口
	leadDelay := 10 * time.Second
	e.tune.firstByteWait = 300 * time.Millisecond

	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- e.run(leadDelay) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("direct silent source was not abandoned within firstByteWait (still waiting full leadDelay)")
	}
	if elapsed := time.Since(start); elapsed >= 5*time.Second {
		t.Fatalf("failover took %v, expected ~firstByteWait", elapsed)
	}
	assertFileEquals(t, dest, payload)
	assertNoTempFiles(t, dest)
}

func TestProbeScorePrefersLowerTTFBAtEqualThroughput(t *testing.T) {
	// 同吞吐下首包更快者优先（同后端多节点的典型差异）
	slowStart := probeScore(0.11, 1800*time.Millisecond)
	fastStart := probeScore(0.11, 700*time.Millisecond)
	if fastStart >= slowStart {
		t.Fatalf("expected lower TTFB to score better: fast=%.3f slow=%.3f", fastStart, slowStart)
	}
}
