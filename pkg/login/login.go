package login

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tsx8/buaa-login/v2/pkg/srun"
)

const (
	defaultGatewayURL = "https://10.200.21.4"
	gatewayTLSName    = "gw.buaa.edu.cn"
)

// Config contains the credentials and optional outbound interface for a login client.
type Config struct {
	StudentID string
	Password  string
	Interface string
}

type Client struct {
	id         string
	pwd        string
	baseURL    string
	httpClient *http.Client
	header     http.Header
}

// New validates config and constructs a client whose requests use one transport.
func New(config Config) (*Client, error) {
	if config.StudentID == "" || config.Password == "" {
		return nil, &Error{
			Kind:      ErrorConfiguration,
			Operation: "validate credentials",
			Message:   "student ID and password must both be non-empty",
		}
	}

	dialer := &net.Dialer{}
	if config.Interface != "" {
		control, err := newInterfaceControl(config.Interface)
		if err != nil {
			return nil, err
		}
		dialer.ControlContext = control
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, &Error{
			Kind:      ErrorConfiguration,
			Operation: "create cookie jar",
			Message:   "unable to initialize login client",
			Err:       err,
		}
	}

	return &Client{
		id:      config.StudentID,
		pwd:     config.Password,
		baseURL: defaultGatewayURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				DialContext: dialer.DialContext,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					ServerName: gatewayTLSName,
				},
			},
		},
		header: http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0"},
		},
	}, nil
}

// Run performs one login attempt. An already-online response is successful.
func (c *Client) Run() error {
	baseURL := strings.TrimRight(c.baseURL, "/")
	bodyInit, err := c.get(baseURL, nil)
	if err != nil {
		return requestError("initialize gateway session", err)
	}

	acidRegex := regexp.MustCompile(`ac_id=(\d+)`)
	acidMatch := acidRegex.FindSubmatch(bodyInit)
	if len(acidMatch) < 2 {
		return transientError("discover gateway access controller", errors.New("ac_id is missing"))
	}
	acid := string(acidMatch[1])

	portalParams := url.Values{}
	portalParams.Set("ac_id", acid)
	portalParams.Set("theme", "buaa")
	bodyPortal, err := c.get(baseURL+"/srun_portal_pc", portalParams)
	if err != nil {
		return requestError("load gateway portal", err)
	}

	ipRegex := regexp.MustCompile(`ip\s*:\s*["']([^"']+)["']`)
	ipMatch := ipRegex.FindSubmatch(bodyPortal)
	if len(ipMatch) < 2 {
		return transientError("discover client address", errors.New("IP address is missing"))
	}
	ip := string(ipMatch[1])

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	params := url.Values{}
	params.Set("callback", "jQuery")
	params.Set("username", c.id)
	params.Set("ip", ip)
	params.Set("_", timestamp)

	bodyChal, err := c.get(baseURL+"/cgi-bin/get_challenge", params)
	if err != nil {
		return requestError("request login challenge", err)
	}

	challenge, err := parseJSONP(bodyChal)
	if err != nil {
		return transientError("parse login challenge", err)
	}
	token, ok := challenge["challenge"].(string)
	if !ok || token == "" {
		return transientError("parse login challenge", errors.New("challenge token is missing"))
	}

	infoData := map[string]string{
		"username": c.id,
		"password": c.pwd,
		"ip":       ip,
		"acid":     acid,
		"enc_ver":  "srun_bx1",
	}
	infoBytes, _ := json.Marshal(infoData)
	infoStr := string(infoBytes)

	encInfo := "{SRBX1}" + srun.GetBase64(srun.GetXEncode(infoStr, token))
	md5Pwd := srun.GetMD5(c.pwd, token)

	chkstr := token + c.id + token + md5Pwd + token + acid + token + ip + token + "200" + token + "1" + token + encInfo
	chksum := srun.GetSHA1(chkstr)

	loginParams := url.Values{}
	loginParams.Set("callback", "jQuery")
	loginParams.Set("action", "login")
	loginParams.Set("username", c.id)
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
		return requestError("submit login request", err)
	}

	result, err := parseJSONP(bodyLogin)
	if err != nil {
		return transientError("parse login response", err)
	}

	if result["error"] == "ok" || result["res"] == "ok" {
		return nil
	}

	resCode := gatewayResultCode(result)
	if resCode == "sign_error" || resCode == "challenge_expire_error" {
		return &Error{
			Kind:      ErrorTransient,
			Operation: "submit login request",
			Code:      resCode,
			Message:   "gateway challenge expired",
		}
	}

	return &Error{
		Kind:      ErrorAuthentication,
		Operation: "submit login request",
		Code:      resCode,
		Message:   "gateway rejected the login",
	}
}

func requestError(operation string, err error) error {
	var classified *Error
	if errors.As(err, &classified) {
		return classified
	}
	return transientError(operation, err)
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
	req.Header = c.header.Clone()

	resp, err := c.httpClient.Do(req)
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
