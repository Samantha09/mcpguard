// 探针客户端 — 连接平台、注册、拉取规则、上报日志
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/gorilla/websocket"
)

// Client 探针客户端
type Client struct {
	platformAddr string
	token        string
	probeID      string
	httpClient   *http.Client
	wsConn       *websocket.Conn
}

// NewClient 创建探针客户端
func NewClient(platformAddr, token string) *Client {
	return &Client{
		platformAddr: platformAddr,
		token:        token,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Register 首次注册探针，返回 token
func (c *Client) Register(ctx context.Context, name, hostname, ip string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{
		"name":     name,
		"hostname": hostname,
		"ip":       ip,
	})
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.platformAddr+"/api/v1/probes/register", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("register failed: %s", resp.Status)
	}

	var result struct {
		ProbeID string `json:"probe_id"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	c.probeID = result.ProbeID
	c.token = result.Token
	return result.ProbeID, result.Token, nil
}

// PullRules 从平台拉取最新规则
func (c *Client) PullRules(ctx context.Context) ([]models.Rule, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.platformAddr+"/api/v1/rules", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pull rules failed: %s", resp.Status)
	}

	var rules []models.Rule
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// ConnectWebSocket 建立 WebSocket 连接
func (c *Client) ConnectWebSocket(ctx context.Context) error {
	u, err := url.Parse(c.platformAddr)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/api/v1/ws"
	u.RawQuery = "probe_id=" + c.probeID + "&token=" + c.token

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	c.wsConn = conn
	return nil
}

// SendLog 通过 WebSocket 发送日志
func (c *Client) SendLog(entry *models.LogEntry) error {
	if c.wsConn == nil {
		return fmt.Errorf("websocket not connected")
	}
	msg := map[string]any{
		"type": "log",
		"data": entry,
	}
	return c.wsConn.WriteJSON(msg)
}

// Close 关闭连接
func (c *Client) Close() {
	if c.wsConn != nil {
		c.wsConn.Close()
	}
}

// ProbeID 返回探针 ID
func (c *Client) ProbeID() string {
	return c.probeID
}

// Token 返回 token
func (c *Client) Token() string {
	return c.token
}
