package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"uni-agent/internal/tool"
)

type cityParams struct {
	Keywords string `json:"keywords"`
	Page     int    `json:"page"`
}

type cityResponse struct {
	Status    string     `json:"status"`
	Info      string     `json:"info"`
	Districts []district `json:"districts"`
}

type district struct {
	CityCode string `json:"citycode"`
	ADCode   string `json:"adcode"`
	Name     string `json:"name"`
}

type cityClient struct {
	key        string
	url        string
	httpclient *http.Client
}

// NewCityTool 用运行配置构造高德城市查询工具
func NewCityTool(key, endpoint string) tool.Tool {
	c := &cityClient{
		key:        key,
		url:        endpoint,
		httpclient: &http.Client{},
	}
	return tool.Tool{
		Name:        "get_city_info",
		Description: "根据城市/区名称获取对应的 citycode（城市编码） 和 adcode（区域编码）",
		Params: tool.Object(map[string]tool.Param{
			"keywords": tool.String("查询关键字:接受单个关键词语,如城市名称、区,即需要查询的目标"),
			"page":     tool.Integer("分页序号,单页最多20个结果,初始值为1,缺省值为1"),
		}),
		Handler: c.GetCityInfo,
	}
}

// GetCityInfo 根据城市/区名称获取对应的 citycode（城市编码） 和 adcode（区域编码）
func (c *cityClient) GetCityInfo(ctx context.Context, args json.RawMessage) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("请求体参数列表为空")
	}
	p := &cityParams{}
	if err := json.Unmarshal(args, p); err != nil {
		return "", fmt.Errorf("反序列化失败: %w", err)
	}

	page := p.Page
	if page <= 0 {
		page = 1
	}

	u, err := url.Parse(c.url)
	if err != nil {
		return "", fmt.Errorf("URL 解析失败: %w", err)
	}
	q := u.Query()
	q.Set("key", c.key)
	q.Set("keywords", p.Keywords)
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("新建请求失败: %w", err)
	}
	httpResp, err := c.httpclient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("发送 http 请求失败, 请检查参数列表: %w", err)
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
	resp := &cityResponse{}

	if err = json.Unmarshal(respBody, resp); err != nil {
		return "", fmt.Errorf("反序列化失败, %w", err)
	}

	if resp.Status != "1" {
		return "", fmt.Errorf("调用API出错，错误码:%s,原因:%s", resp.Status, resp.Info)
	}
	districts, err := json.Marshal(resp.Districts)
	if err != nil {
		return "", fmt.Errorf("序列化失败, %w", err)
	}
	if len(districts) == 0 {
		return "", fmt.Errorf("未查到结果, %w", err)
	}

	return string(districts), nil
}
