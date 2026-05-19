package login

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/tsx8/buaa-login/pkg/srun"
)

type Client struct {
	ID      string
	Pwd     string
	BaseURL string
	Client  *http.Client
	Header  http.Header
	Iface   string
}

func New(id, pwd string, iface string) *Client {
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if iface != "" {
		ifaceObj, err := net.InterfaceByName(iface)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: interface %s not found: %v\n", iface, err)
		} else {
			transport.DialContext = createDialContext(iface, ifaceObj)
		}
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Jar:       jar,
		Transport: transport,
	}

	return &Client{
		ID:      id,
		Pwd:     pwd,
		BaseURL: "https://gw.buaa.edu.cn",
		Client:  client,
		Header: http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0"},
		},
		Iface: iface,
	}
}

func createDialContext(ifaceName string, iface *net.Interface) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	addrs, err := iface.Addrs()
	if err == nil && len(addrs) > 0 {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					dialer.LocalAddr = &net.TCPAddr{IP: ipNet.IP}
					break
				}
			}
		}
	}

	dialer.Control = createControlFunc(ifaceName)

	return dialer.DialContext
}

type Config struct {
	Acid   string `json:"acid"`
	IP     string `json:"ip"`
	Portal struct {
		AuthIP string `json:"AuthIP"`
	} `json:"portal"`
}

func (c *Client) Run() (bool, map[string]interface{}, error) {
	if c.ID == "" || c.Pwd == "" {
		return false, nil, errors.New("id or password missing")
	}

	config, err := c.getConfig()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get config: %v", err)
	}

	ip := config.IP
	acid := config.Acid

	token, err := c.getToken(c.ID, ip)
	if err != nil {
		return false, nil, fmt.Errorf("get token failed: %v", err)
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

	reqLogin, _ := http.NewRequest("GET", c.BaseURL+"/cgi-bin/srun_portal", nil)
	reqLogin.URL.RawQuery = loginParams.Encode()
	reqLogin.Header = c.Header

	respLogin, err := c.Client.Do(reqLogin)
	if err != nil {
		return false, nil, fmt.Errorf("login request failed: %v", err)
	}
	defer respLogin.Body.Close()
	bodyLogin, _ := io.ReadAll(respLogin.Body)

	jsonRegex := regexp.MustCompile(`\(\{.*\}\)`)
	jsonPart := jsonRegex.Find(bodyLogin)
	if len(jsonPart) < 2 {
		return false, nil, fmt.Errorf("invalid login response: %s", string(bodyLogin))
	}
	jsonClean := jsonPart[1 : len(jsonPart)-1]

	var result map[string]interface{}
	if err := json.Unmarshal(jsonClean, &result); err != nil {
		return false, nil, err
	}

	errorCode, ok := result["error"].(string)
	if ok && errorCode == "ok" {
		return true, result, nil
	}

	if errorCode == "sign_error" || errorCode == "challenge_expire_error" {
		return false, result, nil
	}

	return false, result, errors.New("login returned non-ok status")
}

func (c *Client) getConfig() (*Config, error) {
	portalURL := c.BaseURL + "/srun_portal_pc?ac_id=1&theme=buaa"
	resp, err := c.Client.Get(portalURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyStr := string(body)

	config := &Config{
		Acid: "1",
	}

	acidRegex := regexp.MustCompile(`acid\s*:\s*"([^"]+)"`)
	acidMatch := acidRegex.FindSubmatch([]byte(bodyStr))
	if len(acidMatch) >= 2 {
		config.Acid = string(acidMatch[1])
	}

	ipRegex := regexp.MustCompile(`ip\s*:\s*"([^"]+)"`)
	ipMatch := ipRegex.FindSubmatch([]byte(bodyStr))
	if len(ipMatch) >= 2 {
		config.IP = string(ipMatch[1])
	}

	authIPRegex := regexp.MustCompile(`"AuthIP"\s*:\s*"([^"]+)"`)
	authIPMatch := authIPRegex.FindSubmatch([]byte(bodyStr))
	if len(authIPMatch) >= 2 {
		config.Portal.AuthIP = string(authIPMatch[1])
	}

	if config.IP == "" {
		return nil, errors.New("failed to get IP from config")
	}

	return config, nil
}

func (c *Client) getToken(username, ip string) (string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	params := url.Values{}
	params.Set("callback", "jQuery")
	params.Set("username", username)
	params.Set("ip", ip)
	params.Set("_", timestamp)

	req, _ := http.NewRequest("GET", c.BaseURL+"/cgi-bin/get_challenge", nil)
	req.URL.RawQuery = params.Encode()
	req.Header = c.Header

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	tokenRegex := regexp.MustCompile(`"challenge"\s*:\s*"([^"]+)"`)
	tokenMatch := tokenRegex.FindSubmatch(body)
	if len(tokenMatch) < 2 {
		return "", fmt.Errorf("failed to get challenge token from response: %s", string(body))
	}

	return string(tokenMatch[1]), nil
}
