package config

import (
	"strings"
	"testing"
)

// setValidEnv 把所有必填项设成合法值，各用例在此基础上覆盖自己关心的那个
func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("LLM_APIKEY", "sk-test")
	t.Setenv("LLM_BASEURL", "https://api.deepseek.com")
	t.Setenv("AMAP_KEY", "amap-key")
	t.Setenv("AMAP_WEATHER_URL", "https://restapi.amap.com/v3/weather/weatherInfo")
	t.Setenv("AMAP_CONFIG_URL", "https://restapi.amap.com/v3/config/district")
}

func TestLoad_OK(t *testing.T) {
	setValidEnv(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("期望加载成功，实际报错: %v", err)
	}
	if c.LLM.APIKey != "sk-test" {
		t.Errorf("LLM.APIKey = %q, 期望 sk-test", c.LLM.APIKey)
	}
	if c.AMAP.Key != "amap-key" {
		t.Errorf("AMAP.Key = %q, 期望 amap-key", c.AMAP.Key)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	setValidEnv(t)
	t.Setenv("LLM_APIKEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("缺少 LLM_APIKEY 时应返回错误")
	}
	if !strings.Contains(err.Error(), "LLM_APIKEY") {
		t.Errorf("错误信息应包含变量名，实际: %v", err)
	}
}

func TestLoad_MissingBaseURL(t *testing.T) {
	setValidEnv(t)
	t.Setenv("LLM_BASEURL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("缺少 LLM_BASEURL 时应返回错误")
	}
	if !strings.Contains(err.Error(), "LLM_BASEURL") {
		t.Errorf("错误信息应包含变量名，实际: %v", err)
	}
}

func TestLoad_MissingAMAPKey(t *testing.T) {
	setValidEnv(t)
	t.Setenv("AMAP_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("缺少 AMAP_KEY 时应返回错误")
	}
	if !strings.Contains(err.Error(), "AMAP_KEY") {
		t.Errorf("错误信息应包含变量名，实际: %v", err)
	}
}

func TestLoad_ReportsAllMissing(t *testing.T) {
	t.Setenv("LLM_APIKEY", "")
	t.Setenv("LLM_BASEURL", "")
	t.Setenv("AMAP_KEY", "")
	t.Setenv("AMAP_WEATHER_URL", "")
	t.Setenv("AMAP_CONFIG_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("全部缺失时应返回错误")
	}
	want := []string{"LLM_APIKEY", "LLM_BASEURL", "AMAP_KEY", "AMAP_WEATHER_URL", "AMAP_CONFIG_URL"}
	for _, name := range want {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("错误信息应包含 %s，实际: %v", name, err)
		}
	}
}
