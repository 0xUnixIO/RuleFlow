package app

import (
	"context"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchSubscriptionContent 从订阅地址获取原始内容和响应头。
func FetchSubscriptionContent(ctx context.Context, subURL string) (string, http.Header, error) {
	if subURL == "" {
		return "", nil, fmt.Errorf("订阅地址不能为空")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "clash.meta/v1.19.16")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("获取订阅失败（网络错误）: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 尝试解压错误响应体（部分服务器对非 200 响应也启用 gzip）
		errorBody, _ := readResponseBody(resp)
		errorHint := ""
		if len(errorBody) > 0 {
			preview := string(errorBody)
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			errorHint = fmt.Sprintf("，响应内容: %s", preview)
		}
		return "", nil, fmt.Errorf("订阅服务器返回错误（HTTP %d）%s", resp.StatusCode, errorHint)
	}

	content, err := readResponseBody(resp)
	if err != nil {
		return "", nil, fmt.Errorf("读取订阅内容失败: %w", err)
	}

	if len(content) == 0 {
		return "", nil, fmt.Errorf("订阅服务器返回了空内容")
	}

	return string(content), resp.Header, nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	reader := io.Reader(resp.Body)
	var closeFn func() error

	switch encoding {
	case "", "identity":
		// 无压缩
	case "gzip", "x-gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("解析 gzip 响应失败: %w", err)
		}
		reader = gr
		closeFn = gr.Close
	case "deflate":
		zr, err := zlib.NewReader(resp.Body)
		if err == nil {
			reader = zr
			closeFn = zr.Close
			break
		}
		fr := flate.NewReader(resp.Body)
		reader = fr
		closeFn = fr.Close
	default:
		return nil, fmt.Errorf("不支持的响应压缩格式: %s", encoding)
	}

	if closeFn != nil {
		defer closeFn()
	}

	return io.ReadAll(reader)
}
