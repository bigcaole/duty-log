package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func SendFeishuText(webhookURL, title, content string) error {
	url := strings.TrimSpace(webhookURL)
	if url == "" {
		return fmt.Errorf("empty feishu webhook url")
	}

	payload := map[string]any{
		"msg_type": "post",
		"content": map[string]any{
			"post": map[string]any{
				"zh_cn": map[string]any{
					"title": strings.TrimSpace(title),
					"content": [][]map[string]string{
						{
							{
								"tag":  "text",
								"text": strings.TrimSpace(content),
							},
						},
					},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var parsed struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if parsed.Code != 0 {
		return fmt.Errorf("feishu webhook failed: code=%d msg=%s", parsed.Code, parsed.Msg)
	}
	return nil
}
