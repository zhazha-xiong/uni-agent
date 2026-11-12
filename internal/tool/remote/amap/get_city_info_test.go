package amap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newFakeServer 起一个假的高德服务
func newFakeServer(t *testing.T, statusCode int, body string) (*httptest.Server, func() url.Values) {
	t.Helper()
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, func() url.Values { return got }
}

const okBody = `{"status":1,"info":"OK","districts":[{"citycode":"0411","adcode":"210200","name":"大连市"}]}`

func TestNewCityTool_Metadata(t *testing.T) {
	tl := NewCityTool("fake-key", "http://example.com")

	if tl.Name != "get_city_info" {
		t.Errorf("Name = %q", tl.Name)
	}
	if tl.Description == "" {
		t.Error("Description 不应为空")
	}
	if tl.Handler == nil {
		t.Fatal("Handler 不应为 nil")
	}
	if tl.Params.Type != "object" {
		t.Errorf("Params.Type = %q, 期望 object", tl.Params.Type)
	}
	if _, ok := tl.Params.Properties["keywords"]; !ok {
		t.Error("Params 缺少 keywords 字段")
	}
	// cityParams.Page 是 int，schema 必须声明 integer，否则模型会传字符串导致解析失败
	if p := tl.Params.Properties["page"]; p.Type != "integer" {
		t.Errorf("page 的 schema 类型 = %q, 期望 integer", p.Type)
	}
}

func TestGetCityInfo_Success(t *testing.T) {
	srv, query := newFakeServer(t, http.StatusOK, okBody)
	tl := NewCityTool("fake-key", srv.URL)

	out, err := tl.Handler(context.Background(), json.RawMessage(`{"keywords":"大连","page":2}`))
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}

	q := query()
	if got := q.Get("key"); got != "fake-key" {
		t.Errorf("key 参数 = %q, 期望 fake-key", got)
	}
	if got := q.Get("keywords"); got != "大连" {
		t.Errorf("keywords 参数 = %q, 期望 大连", got)
	}
	if got := q.Get("page"); got != "2" {
		t.Errorf("page 参数 = %q, 期望 2", got)
	}

	if !strings.Contains(out, "210200") {
		t.Errorf("返回结果应包含 adcode，实际: %s", out)
	}
	if !strings.Contains(out, "大连市") {
		t.Errorf("返回结果应包含城市名，实际: %s", out)
	}
}

func TestGetCityInfo_DefaultPage(t *testing.T) {
	srv, query := newFakeServer(t, http.StatusOK, okBody)
	tl := NewCityTool("fake-key", srv.URL)

	if _, err := tl.Handler(context.Background(), json.RawMessage(`{"keywords":"大连"}`)); err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}
	if got := query().Get("page"); got != "1" {
		t.Errorf("page 缺省时应为 1，实际 %q", got)
	}
}

func TestGetCityInfo_EmptyArgs(t *testing.T) {
	tl := NewCityTool("fake-key", "http://127.0.0.1:0")

	_, err := tl.Handler(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "参数列表为空") {
		t.Fatalf("空参数应报错，实际: %v", err)
	}
}

func TestGetCityInfo_BadJSON(t *testing.T) {
	tl := NewCityTool("fake-key", "http://127.0.0.1:0")

	_, err := tl.Handler(context.Background(), json.RawMessage(`{`))
	if err == nil || !strings.Contains(err.Error(), "反序列化失败") {
		t.Fatalf("非法 JSON 应报错，实际: %v", err)
	}
}

func TestGetCityInfo_HTTPError(t *testing.T) {
	srv, _ := newFakeServer(t, http.StatusInternalServerError, `boom`)
	tl := NewCityTool("fake-key", srv.URL)

	_, err := tl.Handler(context.Background(), json.RawMessage(`{"keywords":"大连"}`))
	if err == nil {
		t.Fatal("500 应返回错误")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("错误信息应包含状态码，实际: %v", err)
	}
}

func TestGetCityInfo_APIStatusZero(t *testing.T) {
	srv, _ := newFakeServer(t, http.StatusOK, `{"status":0,"info":"INVALID_USER_KEY","districts":[]}`)
	tl := NewCityTool("fake-key", srv.URL)

	_, err := tl.Handler(context.Background(), json.RawMessage(`{"keywords":"大连"}`))
	if err == nil {
		t.Fatal("status=0 应返回错误")
	}
	if !strings.Contains(err.Error(), "INVALID_USER_KEY") {
		t.Errorf("错误信息应包含高德给的原因，实际: %v", err)
	}
}
