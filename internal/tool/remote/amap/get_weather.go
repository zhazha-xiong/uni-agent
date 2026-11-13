package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"uni-agent/internal/tool"
)

type weatherParams struct {
	Adcode   string `json:"adcode"`
	Forecast bool   `json:"forecast"`
}

type weatherResponse struct {
	Status    string     `json:"status"`
	Info      string     `json:"info"`
	Lives     []live     `json:"lives"`
	Forecasts []forecast `json:"forecasts"`
}

type live struct {
	City          string `json:"city"`
	Weather       string `json:"weather"`
	Temperature   string `json:"temperature"`
	WindDirection string `json:"winddirection"`
	WindPower     string `json:"windpower"`
	Humidity      string `json:"humidity"`
	ReportTime    string `json:"reporttime"`
}

type forecast struct {
	City       string `json:"city"`
	ReportTime string `json:"reporttime"`
	Casts      []cast `json:"casts"`
}

type cast struct {
	Date         string `json:"date"`
	Week         string `json:"week"`
	DayWeather   string `json:"dayweather"`
	NightWeather string `json:"nightweather"`
	DayTemp      string `json:"daytemp"`
	NightTemp    string `json:"nighttemp"`
	DayWind      string `json:"daywind"`
	DayPower     string `json:"daypower"`
}

// weatherResult 只回给模型用得上的字段，避免把整包响应灌进上下文
type weatherResult struct {
	City       string `json:"city"`
	ReportTime string `json:"reporttime"`
	Now        *live  `json:"now,omitempty"`
	Casts      []cast `json:"casts,omitempty"`
}

type weatherClient struct {
	key        string
	url        string
	httpclient *http.Client
}

// NewWeatherTool 用运行配置构造高德天气查询工具
func NewWeatherTool(key, endpoint string) tool.Tool {
	c := &weatherClient{
		key:        key,
		url:        endpoint,
		httpclient: &http.Client{},
	}
	return tool.Tool{
		Name:        "get_weather",
		Description: "查询指定城市的当前天气或未来 4 天预报，adcode 是城市编码，可先用 get_city_info 由城市名换得",
		Params: tool.Object(map[string]tool.Param{
			"adcode":   tool.String("城市编码，如北京市 110000；不确定时先调用 get_city_info 获取"),
			"forecast": tool.Boolean("false 查当前实况（默认），true 查未来 4 天预报"),
		}, "adcode"),
		Handler: c.GetWeather,
	}
}

// GetWeather 查询指定 adcode 的实况或预报天气
func (c *weatherClient) GetWeather(ctx context.Context, args json.RawMessage) (string, error) {
	p := &weatherParams{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, p); err != nil {
			return "", fmt.Errorf("反序列化失败: %w", err)
		}
	}
	if p.Adcode == "" {
		return "", fmt.Errorf("缺少必填参数 adcode")
	}

	extensions := "base"
	if p.Forecast {
		extensions = "all"
	}

	u, err := url.Parse(c.url)
	if err != nil {
		return "", fmt.Errorf("URL 解析失败: %w", err)
	}
	q := u.Query()
	q.Set("key", c.key)
	q.Set("city", p.Adcode)
	q.Set("extensions", extensions)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("新建请求失败: %w", err)
	}
	httpResp, err := c.httpclient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("发送 http 请求失败: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("请求出错,状态码: %s,错误信息: %s", httpResp.Status, body)
	}
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("获取 resp 失败, %w", err)
	}

	resp := &weatherResponse{}
	if err := json.Unmarshal(respBody, resp); err != nil {
		return "", fmt.Errorf("反序列化失败, %w", err)
	}
	if resp.Status != "1" {
		return "", fmt.Errorf("调用API出错，错误码:%s,原因:%s", resp.Status, resp.Info)
	}

	out := weatherResult{}
	if p.Forecast {
		if len(resp.Forecasts) == 0 || len(resp.Forecasts[0].Casts) == 0 {
			return "", fmt.Errorf("未查到该城市的预报，请确认 adcode 是否正确")
		}
		f := resp.Forecasts[0]
		out.City = f.City
		out.ReportTime = f.ReportTime
		out.Casts = f.Casts
	} else {
		if len(resp.Lives) == 0 {
			return "", fmt.Errorf("未查到该城市的实况天气，请确认 adcode 是否正确")
		}
		l := resp.Lives[0]
		out.City = l.City
		out.ReportTime = l.ReportTime
		out.Now = &l
	}

	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("序列化失败, %w", err)
	}
	return string(b), nil
}
