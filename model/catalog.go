package model

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// ListModels fetches the remote catalog without modifying the manually configured
// Models list. Manual IDs remain usable even when catalog retrieval is unsupported.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	ids := map[string]bool{}
	cursors := map[string]bool{}
	endpoint := "/models"
	catalogBytes := 0
	for page := 0; page < 100; page++ {
		data, err := c.get(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		catalogBytes += len(data)
		if catalogBytes > bodyLimit {
			return nil, errors.New("模型目录总计超过 8 MiB")
		}
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool            `json:"has_more"`
			LastID  string          `json:"last_id"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, errors.New("模型目录不是合法 JSON")
		}
		if len(result.Error) > 0 && string(result.Error) != "null" {
			return nil, errors.New("模型目录接口返回了错误")
		}
		if result.Data == nil {
			return nil, errors.New("模型目录缺少 data 字段")
		}
		for _, entry := range result.Data {
			if strings.TrimSpace(entry.ID) != "" {
				ids[entry.ID] = true
			}
		}
		if !result.HasMore {
			models := make([]string, 0, len(ids))
			for id := range ids {
				models = append(models, id)
			}
			sort.Strings(models)
			return models, nil
		}
		if c.provider.Protocol != ProtocolAnthropic || result.LastID == "" || cursors[result.LastID] {
			return nil, errors.New("模型目录的分页方式不受支持或无效")
		}
		cursors[result.LastID] = true
		endpoint = "/models?after_id=" + url.QueryEscape(result.LastID)
	}
	return nil, errors.New("模型目录超过 100 页")
}

// get reads one catalog page, authenticated as the protocol expects.
func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.provider.BaseURL, "/")+endpoint, nil)
	if err != nil {
		return nil, errors.New("无法创建模型请求")
	}
	req.Header.Set("Accept", "application/json")
	if c.provider.Protocol == ProtocolAnthropic {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	// guard adds the credential.
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, safeError(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, statusError(resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	if err != nil {
		return nil, safeError(ctx, err)
	}
	if len(data) > bodyLimit {
		return nil, errors.New("模型目录超过 8 MiB")
	}
	return data, nil
}
