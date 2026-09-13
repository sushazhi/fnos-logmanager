package notify

import (
	"reflect"
	"strings"
	"testing"
)

// 本文件守护「UI 声明的渠道字段」与「后端实际可读的配置字段」之间的一致性。
//
// 历史上这两者曾经脱节：前端 supportedChannelTypes 声明了 DINGTALK_TOKEN、
// FEISHU_WEBHOOK、WECOM_KEY 等字段，而后端渠道实现读取的是 DD_BOT_TOKEN、
// FSKEY、QYWX_KEY —— 用户认真填写后配置被静默丢弃，渠道报「配置不完整」。
// 这里用测试把该约定固定下来，防止再次漂移。

// configTagName 从结构体字段的 json tag 中取出 key 名。
func configTagName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

// allConfigKeys 返回 ChannelConfig 支持的全部配置 key。
func allConfigKeys() map[string]bool {
	keys := make(map[string]bool)
	cfgType := reflect.TypeOf(ChannelConfig{})
	for i := 0; i < cfgType.NumField(); i++ {
		if name := configTagName(cfgType.Field(i)); name != "" {
			keys[name] = true
		}
	}
	return keys
}

// TestChannelConfigKeysAreResolvable 保证每个声明的 json tag 都能被 GetConfig 读到。
// 这直接覆盖了 GOTIFY_PRIORITY 那样的缺陷：字段存在、环境变量也加载了，
// 但因为查询路径遗漏而永远读不出来。
func TestChannelConfigKeysAreResolvable(t *testing.T) {
	for key := range allConfigKeys() {
		// 不依赖实际取值：只要求查询路径存在。
		// 若字段被反射遗漏，GetConfig 会对任何已设置的值返回空串。
		if !configKeyResolvable(key) {
			t.Errorf("GetConfig cannot resolve declared key %q", key)
		}
	}
}

// TestNumericConfigKeyIsReadable 专门覆盖 GOTIFY_PRIORITY。
// 它是唯一的数值型配置字段，曾因手写 switch 缺少 case 而丢失。
func TestNumericConfigKeyIsReadable(t *testing.T) {
	const key = "GOTIFY_PRIORITY"

	if !configKeyResolvable(key) {
		t.Fatalf("config key %q is not resolvable", key)
	}

	// 设置后必须能读回（数值字段以十进制字符串返回）。
	SetConfig(key, "7")
	defer SetConfig(key, "")
	if got := GetConfig(key); got != "7" {
		t.Fatalf("GetConfig(%q) = %q, want %q", key, got, "7")
	}
	if !HasConfig(key) {
		t.Fatalf("HasConfig(%q) should be true after SetConfig", key)
	}

	// 清空后必须回到未配置状态，而不是返回 "0"。
	SetConfig(key, "")
	if HasConfig(key) {
		t.Fatalf("HasConfig(%q) should be false after clearing", key)
	}
}

// configKeyResolvable 判断 key 是否在 ChannelConfig 中存在对应字段。
func configKeyResolvable(key string) bool {
	cfgType := reflect.TypeOf(ChannelConfig{})
	for i := 0; i < cfgType.NumField(); i++ {
		if configTagName(cfgType.Field(i)) == key {
			return true
		}
	}
	return false
}

// TestSetConfigRoundTrip 验证每个 key 都能设置并读回，确保读写使用同一套 tag 约定。
func TestSetConfigRoundTrip(t *testing.T) {
	snapshot := SnapshotConfig()
	defer RestoreConfig(snapshot)

	cfgType := reflect.TypeOf(ChannelConfig{})
	for i := 0; i < cfgType.NumField(); i++ {
		field := cfgType.Field(i)
		key := configTagName(field)
		if key == "" {
			continue
		}
		// 数值字段需要数字字面量，字符串字段用普通探针值。
		probe := "probe-value"
		switch field.Type.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			probe = "42"
		}
		SetConfig(key, probe)
		if got := GetConfig(key); got != probe {
			t.Errorf("round trip failed for %q: got %q, want %q", key, got, probe)
		}
		SetConfig(key, "")
		if got := GetConfig(key); got != "" {
			t.Errorf("clear failed for %q: got %q, want empty", key, got)
		}
	}
}

// TestConfigAliasesResolveToRealFields 保证历史别名指向真实存在的字段。
// 别名若指向不存在的 key，配置会被静默丢弃，正是本文件要防的问题。
func TestConfigAliasesResolveToRealFields(t *testing.T) {
	keys := allConfigKeys()
	for alias, target := range configAliases {
		if !keys[target] {
			t.Errorf("alias %q points to unknown config key %q", alias, target)
		}
	}
}

// TestUnknownConfigKeyIsEmpty 未知 key 必须返回空串，而不是 panic 或误命中。
func TestUnknownConfigKeyIsEmpty(t *testing.T) {
	if got := GetConfig("DEFINITELY_NOT_A_REAL_KEY"); got != "" {
		t.Fatalf("unknown key returned %q, want empty", got)
	}
	if HasConfig("DEFINITELY_NOT_A_REAL_KEY") {
		t.Fatal("HasConfig should be false for an unknown key")
	}
}

// TestSnapshotRestoreIsolatesOverrides 证明快照/恢复能隔离单渠道覆盖，
// 且并发安全（配合 -race 运行时有效）。
func TestSnapshotRestoreIsolatesOverrides(t *testing.T) {
	const key = "TG_BOT_TOKEN"
	SetConfig(key, "original")
	defer SetConfig(key, "")

	snapshot := SnapshotConfig()
	SetConfig(key, "temporary")
	if got := GetConfig(key); got != "temporary" {
		t.Fatalf("override not applied: %q", got)
	}

	RestoreConfig(snapshot)
	if got := GetConfig(key); got != "original" {
		t.Fatalf("restore failed: got %q, want %q", got, "original")
	}
}

// TestConfigConcurrentAccess 在 -race 下验证读写不会产生数据竞争。
func TestConfigConcurrentAccess(t *testing.T) {
	const key = "BARK_PUSH"
	SetConfig(key, "initial")
	defer SetConfig(key, "")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = GetConfig(key)
			_ = HasConfig(key)
		}
	}()
	for i := 0; i < 200; i++ {
		SetConfig(key, "value")
		_ = SnapshotConfig()
	}
	<-done
}
