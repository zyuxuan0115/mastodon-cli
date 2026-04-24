package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	Server string
	Token  string
	HTTP   *http.Client
}

func newClient(server, token string) *Client {
	return &Client{Server: server, Token: token, HTTP: http.DefaultClient}
}

type App struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Acct        string `json:"acct"`
	DisplayName string `json:"display_name"`
}

type Status struct {
	ID          string  `json:"id"`
	Content     string  `json:"content"`
	CreatedAt   string  `json:"created_at"`
	Visibility  string  `json:"visibility"`
	URL         string  `json:"url"`
	Account     Account `json:"account"`
	SpoilerText string  `json:"spoiler_text"`
	InReplyToID string  `json:"in_reply_to_id,omitempty"`
}

type PostParams struct {
	Status      string
	Visibility  string
	SpoilerText string
	InReplyToID string
	MediaIDs    []string
}

func (c *Client) do(method, path string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, c.Server+path, body)
	if err != nil {
		return err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) RegisterApp(name, scopes, redirect string) (*App, error) {
	form := url.Values{}
	form.Set("client_name", name)
	form.Set("redirect_uris", redirect)
	form.Set("scopes", scopes)
	var a App
	if err := c.do("POST", "/api/v1/apps", form, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (c *Client) ExchangeCode(clientID, clientSecret, code, redirect, scopes string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirect)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("scope", scopes)
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := c.do("POST", "/oauth/token", form, &out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func (c *Client) Post(p PostParams) (*Status, error) {
	form := url.Values{}
	form.Set("status", p.Status)
	if p.Visibility != "" {
		form.Set("visibility", p.Visibility)
	}
	if p.SpoilerText != "" {
		form.Set("spoiler_text", p.SpoilerText)
	}
	if p.InReplyToID != "" {
		form.Set("in_reply_to_id", p.InReplyToID)
	}
	for _, id := range p.MediaIDs {
		form.Add("media_ids[]", id)
	}
	var s Status
	if err := c.do("POST", "/api/v1/statuses", form, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) Timeline(kind string, limit int) ([]Status, error) {
	var path string
	switch kind {
	case "home":
		path = "/api/v1/timelines/home"
	case "public":
		path = "/api/v1/timelines/public"
	default:
		return nil, fmt.Errorf("unknown timeline kind: %s (use home or public)", kind)
	}
	if limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	var ss []Status
	if err := c.do("GET", path, nil, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func (c *Client) Delete(id string) error {
	return c.do("DELETE", "/api/v1/statuses/"+id, nil, nil)
}

func (c *Client) AccountStatuses(accountID string, limit int, excludeReplies, excludeReblogs bool) ([]Status, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if excludeReplies {
		q.Set("exclude_replies", "true")
	}
	if excludeReblogs {
		q.Set("exclude_reblogs", "true")
	}
	path := "/api/v1/accounts/" + accountID + "/statuses"
	if s := q.Encode(); s != "" {
		path += "?" + s
	}
	var ss []Status
	if err := c.do("GET", path, nil, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func (c *Client) UploadMedia(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.Server+"/api/v2/media", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("upload %s: %s: %s", filepath.Base(path), resp.Status, strings.TrimSpace(string(body)))
	}
	var m struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusAccepted || m.URL == "" {
		if err := c.waitForMedia(m.ID); err != nil {
			return "", err
		}
	}
	return m.ID, nil
}

func (c *Client) waitForMedia(id string) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		req, err := http.NewRequest("GET", c.Server+"/api/v1/media/"+id, nil)
		if err != nil {
			return err
		}
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode >= 400 {
			return fmt.Errorf("media %s: %s", id, resp.Status)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("media %s still processing after 60s", id)
		}
		time.Sleep(1 * time.Second)
	}
}

func (c *Client) VerifyCredentials() (*Account, error) {
	var a Account
	if err := c.do("GET", "/api/v1/accounts/verify_credentials", nil, &a); err != nil {
		return nil, err
	}
	return &a, nil
}
