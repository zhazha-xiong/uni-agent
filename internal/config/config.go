package config

import (
	"fmt"
	"os"
)

type Config struct {
	LLM  LLM
	AMAP AMAP
}

// LLM chat completions 客户端需要的参数
type LLM struct {
	APIKey  string
	BaseURL string
}

// AMAP 高德地图工具需要的参数
type AMAP struct {
	Key        string
	WeatherURL string
	ConfigURL  string
}

// Load 读取环境变量并校验，必填项缺失时返回错误而不是零值
func Load() (Config, error) {
	c := Config{
		LLM: LLM{
			APIKey:  os.Getenv("LLM_APIKEY"),
			BaseURL: os.Getenv("LLM_BASEURL"),
		},
		AMAP: AMAP{
			Key:        os.Getenv("AMAP_KEY"),
			WeatherURL: os.Getenv("AMAP_WEATHER_URL"),
			ConfigURL:  os.Getenv("AMAP_CONFIG_URL"),
		},
	}

	var missing []string
	if c.LLM.APIKey == "" {
		missing = append(missing, "LLM_APIKEY")
	}
	if c.LLM.BaseURL == "" {
		missing = append(missing, "LLM_BASEURL")
	}
	if c.AMAP.Key == "" {
		missing = append(missing, "AMAP_KEY")
	}
	if c.AMAP.WeatherURL == "" {
		missing = append(missing, "AMAP_WEATHER_URL")
	}
	if c.AMAP.ConfigURL == "" {
		missing = append(missing, "AMAP_CONFIG_URL")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("缺少必填环境变量: %v", missing)
	}

	return c, nil
}
