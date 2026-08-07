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

const (
	defaultGatewayURL = "https://10.200.21.4"
	gatewayTLSName    = "gw.buaa.edu.cn"
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
		BaseURL: defaultGatewayURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					ServerName: gatewayTLSName,
				},
			},
		},
		Header: http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0"},
		},
	}
}

func (c *Client) Run() (bool, map[string]interface{}, error) {
	if c.ID == "" || c.Pwd == "" {
		return false, nil, &Error{
			Kind:      ErrorConfiguration,
			Operation: "validate credentials",
			Message:   "student ID and password must both be non-empty",
		}
	}

	baseURL := strings.TrimRight(c.BaseURL, "/")
	bodyInit, err := c.get(baseURL, nil)
	if err != nil {
		return false, nil, transientError("initialize gateway session", err)
	}

	acidRegex := regexp.MustCompile(`ac_id=(\d+)`)
	acidMatch := acidRegex.FindSubmatch(bodyInit)
	if len(acidMatch) < 2 {
		return false, nil, transientError("discover gateway access controller", errors.New("ac_id is missing"))
	}
	acid := string(acidMatch[1])

	portalParams := url.Values{}
	portalParams.Set("ac_id", acid)
	portalParams.Set("theme", "buaa")
	bodyPortal, err := c.get(baseURL+"/srun_portal_pc", portalParams)
	if err != nil {
		return false, nil, transientError("load gateway portal", err)
	}

	ipRegex := regexp.MustCompile(`ip\s*:\s*["']([^"']+)["']`)
	ipMatch := ipRegex.FindSubmatch(bodyPortal)
	if len(ipMatch) < 2 {
		return false, nil, transientError("discover client address", errors.New("IP address is missing"))
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
		return false, nil, transientError("request login challenge", err)
	}

	challenge, err := parseJSONP(bodyChal)
	if err != nil {
		return false, nil, transientError("parse login challenge", err)
	}
	token, ok := challenge["challenge"].(string)
	if !ok || token == "" {
		return false, nil, transientError("parse login challenge", errors.New("challenge token is missing"))
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
		return false, nil, transientError("submit login request", err)
	}

	result, err := parseJSONP(bodyLogin)
	if err != nil {
		return false, nil, transientError("parse login response", err)
	}

	if result["error"] == "ok" || result["res"] == "ok" {
		return true, result, nil
	}

	resCode := gatewayResultCode(result)
	if resCode == "sign_error" || resCode == "challenge_expire_error" {
		return false, result, &Error{
			Kind:      ErrorTransient,
			Operation: "submit login request",
			Code:      resCode,
			Message:   "gateway challenge expired",
		}
	}

	return false, result, &Error{
		Kind:      ErrorAuthentication,
		Operation: "submit login request",
		Code:      resCode,
		Message:   "gateway rejected the login",
	}
}

func transientError(operation string, err error) error {
	return &Error{
		Kind:      ErrorTransient,
		Operation: operation,
		Message:   "temporary failure",
		Err:       err,
	}
}

func gatewayResultCode(result map[string]interface{}) string {
	for _, key := range []string{"error", "res"} {
		if value, ok := result[key].(string); ok && value != "" && value != "ok" {
			return safeGatewayCode(value)
		}
	}
	return "unknown"
}

func safeGatewayCode(code string) string {
	if len(code) > 64 {
		return "unknown"
	}
	for _, char := range code {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '_' && char != '-' && char != '.' {
			return "unknown"
		}
	}
	return code
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
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err
		}
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
