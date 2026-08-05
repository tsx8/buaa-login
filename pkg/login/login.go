package login

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tsx8/buaa-login/pkg/srun"
)

type Client struct {
	ID      string
	Pwd     string
	BaseURL string
	Client  *http.Client
	Header  http.Header
}

func New(id, pwd string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		ID:      id,
		Pwd:     pwd,
		BaseURL: "https://10.200.21.4",
		Client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		Header: http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0"},
		},
	}
}

func (c *Client) Run() (bool, map[string]interface{}, error) {
	if c.ID == "" || c.Pwd == "" {
		return false, nil, errors.New("id or password missing")
	}

	baseURL := strings.TrimRight(c.BaseURL, "/")
	bodyInit, err := c.get(baseURL, nil)
	if err != nil {
		return false, nil, fmt.Errorf("init request failed: %v", err)
	}

	acidRegex := regexp.MustCompile(`ac_id=(\d+)`)
	acidMatch := acidRegex.FindSubmatch(bodyInit)
	if len(acidMatch) < 2 {
		return false, nil, errors.New("failed to extract ac_id from redirect page")
	}
	acid := string(acidMatch[1])

	portalParams := url.Values{}
	portalParams.Set("ac_id", acid)
	portalParams.Set("theme", "buaa")
	bodyPortal, err := c.get(baseURL+"/srun_portal_pc", portalParams)
	if err != nil {
		return false, nil, fmt.Errorf("portal request failed: %v", err)
	}

	ipRegex := regexp.MustCompile(`ip\s*:\s*["']([^"']+)["']`)
	ipMatch := ipRegex.FindSubmatch(bodyPortal)
	if len(ipMatch) < 2 {
		return false, nil, errors.New("failed to extract ip from portal page")
	}
	ip := string(ipMatch[1])

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	params := url.Values{}
	params.Set("callback", "jQuery")
	params.Set("username", c.ID)
	params.Set("ip", ip)
	params.Set("_", timestamp)

	bodyChal, err := c.get(baseURL+"/cgi-bin/get_challenge", params)
	if err != nil {
		return false, nil, fmt.Errorf("get_challenge failed: %v", err)
	}

	challenge, err := parseJSONP(bodyChal)
	if err != nil {
		return false, nil, fmt.Errorf("invalid challenge response: %v", err)
	}
	token, ok := challenge["challenge"].(string)
	if !ok || token == "" {
		return false, nil, errors.New("failed to get challenge token")
	}

	infoData := map[string]string{
		"username": c.ID,
		"password": c.Pwd,
		"ip":       ip,
		"acid":     acid,
		"enc_ver":  "srun_bx1",
	}
	infoBytes, _ := json.Marshal(infoData)
	infoStr := string(infoBytes)

	encInfo := "{SRBX1}" + srun.GetBase64(srun.GetXEncode(infoStr, token))
	md5Pwd := srun.GetMD5(c.Pwd, token)

	chkstr := token + c.ID + token + md5Pwd + token + acid + token + ip + token + "200" + token + "1" + token + encInfo
	chksum := srun.GetSHA1(chkstr)

	loginParams := url.Values{}
	loginParams.Set("callback", "jQuery")
	loginParams.Set("action", "login")
	loginParams.Set("username", c.ID)
	loginParams.Set("password", "{MD5}"+md5Pwd)
	loginParams.Set("ac_id", acid)
	loginParams.Set("ip", ip)
	loginParams.Set("chksum", chksum)
	loginParams.Set("info", encInfo)
	loginParams.Set("n", "200")
	loginParams.Set("type", "1")
	loginParams.Set("os", "windows+10")
	loginParams.Set("name", "windows")
	loginParams.Set("double_stack", "0")
	loginParams.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))

	bodyLogin, err := c.get(baseURL+"/cgi-bin/srun_portal", loginParams)
	if err != nil {
		return false, nil, fmt.Errorf("login request failed: %v", err)
	}

	result, err := parseJSONP(bodyLogin)
	if err != nil {
		return false, nil, fmt.Errorf("invalid login response: %v", err)
	}

	if result["error"] == "ok" || result["res"] == "ok" {
		return true, result, nil
	}

	resCode, _ := result["res"].(string)
	if resCode == "sign_error" || resCode == "challenge_expire_error" {
		return false, result, nil
	}

	return false, result, errors.New("login returned non-ok status")
}

func (c *Client) get(rawURL string, params url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if params != nil {
		req.URL.RawQuery = params.Encode()
	}
	req.Header = c.Header.Clone()

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func parseJSONP(body []byte) (map[string]interface{}, error) {
	body = bytes.TrimSpace(body)
	open := bytes.IndexByte(body, '(')
	close := bytes.LastIndexByte(body, ')')
	if open < 0 || close <= open {
		return nil, errors.New("missing JSONP wrapper")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body[open+1:close], &result); err != nil {
		return nil, err
	}
	return result, nil
}
