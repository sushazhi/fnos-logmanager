package routes

import "testing"

func TestParseManifestVersion(t *testing.T) {
	// 取自线上真实 manifest（经 gh.dpik.top 拉取），用于固定实际格式：key = value、LF 行尾
	live := "appname = logmanager\nversion = 0.8.2\ndisplay_name = 飞牛日志管理器\nplatform = all\n"
	if got := parseManifestVersion(live); got != "0.8.2" {
		t.Fatalf("线上内容解析错误: got %q", got)
	}

	cases := map[string]string{
		"version=1.2.3":                  "1.2.3",
		"version = 1.2.3\n":              "1.2.3",
		"# comment\nversion = 9.9.9\r\n": "9.9.9",
		"appname = logmanager\n":         "",
		"":                               "",
		"myversion = 1.0.0":              "",
		"display_name = version":         "",
	}
	for in, want := range cases {
		if got := parseManifestVersion(in); got != want {
			t.Errorf("输入 %q: got %q, want %q", in, got, want)
		}
	}
}
