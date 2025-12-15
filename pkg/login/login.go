package login

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/tsx8/buaa-login/pkg/srun"
)

type Client struct {
	ID       string
	Pwd      string
	BaseURL  string
	Client   *http.Client
	Header   http.Header
}

func New(id, pwd string) *Client {
	return &Client{
		ID:      id,
		Pwd:     pwd,
		BaseURL: "https://gw.buaa.edu.cn",
		Client: &http.Client{
			Timeout: 10 * time.Second,
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

	reqInit, _ := http.NewRequest("GET", c.BaseURL, nil)
	reqInit.Header = c.Header
	respInit, err := c.Client.Do(reqInit)
	if err != nil {
		return false, nil, fmt.Errorf("init request failed: %v", err)
	}
	defer respInit.Body.Close()
	bodyInit, _ := io.ReadAll(respInit.Body)

	ipRegex := regexp.MustCompile(`id="user_ip" value="(.*?)"`)
	ipMatch := ipRegex.FindSubmatch(bodyInit)
	if len(ipMatch) < 2 {
		return false, nil, errors.New("failed to find user_ip")
	}
	ip := string(ipMatch[1])

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	params := url.Values{}
	params.Set("callback", "jQuery112401316671457983556_"+timestamp)
	params.Set("username", c.ID)
	params.Set("ip", ip)
	params.Set("_", timestamp)

	reqChal, _ := http.NewRequest("GET", c.BaseURL+"/cgi-bin/get_challenge", nil)
	reqChal.URL.RawQuery = params.Encode()
	reqChal.Header = c.Header
	
	respChal, err := c.Client.Do(reqChal)
	if err != nil {
		return false, nil, fmt.Errorf("get_challenge failed: %v", err)
	}
	defer respChal.Body.Close()
	bodyChal, _ := io.ReadAll(respChal.Body)

	tokenRegex := regexp.MustCompile(`"challenge":"(.*?)"`)
	tokenMatch := tokenRegex.FindSubmatch(bodyChal)
	if len(tokenMatch) < 2 {
		return false, nil, errors.New("failed to get challenge token")
	}
	token := string(tokenMatch[1])

	infoData := map[string]string{
		"username": c.ID,
		"password": c.Pwd,
		"ip":       ip,
		"acid":     "1",
		"enc_var":  "srun_bx1",
	}
	infoBytes, _ := json.Marshal(infoData)
	infoStr := string(infoBytes)
	
	encInfo := "{SRBX1}" + srun.GetBase64(srun.GetXEncode(infoStr, token))
	md5Pwd := srun.GetMD5(c.Pwd, token)
	
	chkstr := token + c.ID + token + md5Pwd + token + "1" + token + ip + token + "200" + token + "1" + token + encInfo
	chksum := srun.GetSHA1(chkstr)

	loginParams := url.Values{}
	loginParams.Set("callback", "jQuery112401316671457983556_"+timestamp)
	loginParams.Set("action", "login")
	loginParams.Set("username", c.ID)
	loginParams.Set("password", "{MD5}"+md5Pwd)
	loginParams.Set("ac_id", "1")
	loginParams.Set("ip", ip)
	loginParams.Set("chksum", chksum)
	loginParams.Set("info", encInfo)
	loginParams.Set("n", "200")
	loginParams.Set("type", "1")
	loginParams.Set("os", "windows+10")
	loginParams.Set("name", "windows")
	loginParams.Set("double_stack", "0")
	loginParams.Set("_", timestamp)

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

	resCode, ok := result["res"].(string)
	if ok && resCode == "ok" {
		return true, result, nil
	}
	
	if resCode == "sign_error" || resCode == "challenge_expire_error" {
		return false, result, nil
	}

	return false, result, errors.New("login returned non-ok status")
}