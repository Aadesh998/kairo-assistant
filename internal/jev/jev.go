package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const endpoint = "https://api.typesafe.ai/v1/systemone"

type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Noul          float64            `json:"noul"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type Client struct {
	model string
	http  *http.Client
}

func New(model string) *Client {
	return &Client{model: model, http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) Ready() bool {
	return c != nil && os.Getenv("TYPESAFE_API_KEY") != ""
}

func Choice(instructions string, options map[string]string) Question {
	return Question{Type: "choice", Instructions: instructions, Criteria: options}
}

func Score(instructions string, levels []string) Question {
	return Question{Type: "score", Instructions: instructions, Criteria: levels}
}

func Noul(instructions string) Question {
	return Question{Type: "noul", Instructions: instructions}
}

func (c *Client) Decide(ctx context.Context, state string, questions map[string]Question) (map[string]Answer, error) {
	if !c.Ready() {
		return nil, errors.New("TYPESAFE_API_KEY is not set")
	}
	body, _ := json.Marshal(map[string]any{"state": state, "model": c.model, "questions": questions})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("TYPESAFE_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jev %d: %s", resp.StatusCode, msg)
	}
	var out struct {
		Answers map[string]Answer `json:"answers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Answers, nil
}
