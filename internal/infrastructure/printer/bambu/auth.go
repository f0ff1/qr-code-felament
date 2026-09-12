package bambu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	bambicloud "github.com/torbenconto/bambulabs_cloud_api"
)

const (
	authUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	authTimeout   = 30 * time.Second
)

type authLoginResponse struct {
	AccessToken string `json:"accessToken"`
	LoginType   string `json:"loginType"`
	TfaKey      string `json:"tfaKey"`
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Error       string `json:"error"`
}

func cloudAPIBase(region string) string {
	if override := strings.TrimSpace(cloudAPIBaseOverride); override != "" {
		return strings.TrimRight(override, "/")
	}
	if MapCloudRegion(region) == bambicloud.China {
		return "https://api.bambulab.cn/v1"
	}
	return "https://api.bambulab.com/v1"
}

// cloudAPIBaseOverride is used by tests only.
var cloudAPIBaseOverride string

// CloudLogin authenticates with Bambu. On verifyCode it also requests the email OTP
// via /user-service/user/sendemail/code (the third-party Go client skips that step).
func CloudLogin(email, password, region string) (CloudLoginResult, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return CloudLoginResult{}, fmt.Errorf("cloud email and password are required")
	}

	body := map[string]string{
		"account":  email,
		"password": password,
		"apiError": "",
	}
	resp, status, raw, err := postCloudJSON(cloudAPIBase(region)+"/user-service/user/login", body, "")
	if err != nil {
		return CloudLoginResult{}, err
	}
	if status != http.StatusOK {
		return CloudLoginResult{}, fmt.Errorf("login failed (%d): %s", status, authErrorMessage(resp, raw))
	}

	switch strings.TrimSpace(resp.LoginType) {
	case "verifyCode":
		if err := CloudSendEmailCode(email, region); err != nil {
			return CloudLoginResult{}, fmt.Errorf("password ok, but email code was not sent: %w", err)
		}
		return CloudLoginResult{NeedsVerify: true}, nil
	case "tfa":
		return CloudLoginResult{}, fmt.Errorf("account requires authenticator 2FA (tfa) — set a password login with email code or disable app 2FA in Bambu account")
	case "":
		if strings.TrimSpace(resp.AccessToken) == "" {
			return CloudLoginResult{}, fmt.Errorf("empty access token from Bambu: %s", authErrorMessage(resp, raw))
		}
		return CloudLoginResult{Token: resp.AccessToken}, nil
	default:
		if strings.TrimSpace(resp.AccessToken) != "" {
			return CloudLoginResult{Token: resp.AccessToken}, nil
		}
		return CloudLoginResult{}, fmt.Errorf("unsupported loginType %q: %s", resp.LoginType, authErrorMessage(resp, raw))
	}
}

// CloudSendEmailCode asks Bambu to email the 6-digit login code.
func CloudSendEmailCode(email, region string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	body := map[string]string{
		"email": email,
		"type":  "codeLogin",
	}
	resp, status, raw, err := postCloudJSON(cloudAPIBase(region)+"/user-service/user/sendemail/code", body, "")
	if err != nil {
		return err
	}
	// Some deployments return 200 with empty body; treat non-2xx as failure.
	if status < 200 || status >= 300 {
		return fmt.Errorf("send email code failed (%d): %s", status, authErrorMessage(resp, raw))
	}
	return nil
}

// CloudVerify completes login with the email verification code.
func CloudVerify(email, code, region string) (string, error) {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return "", fmt.Errorf("cloud email and verification code are required")
	}
	body := map[string]string{
		"account": email,
		"code":    code,
	}
	resp, status, raw, err := postCloudJSON(cloudAPIBase(region)+"/user-service/user/login", body, "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("verify code failed (%d): %s", status, authErrorMessage(resp, raw))
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return "", fmt.Errorf("empty token after verification: %s", authErrorMessage(resp, raw))
	}
	return resp.AccessToken, nil
}

func postCloudJSON(url string, payload any, bearer string) (authLoginResponse, int, string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return authLoginResponse{}, 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return authLoginResponse{}, 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", authUserAgent)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	client := &http.Client{Timeout: authTimeout}
	res, err := client.Do(req)
	if err != nil {
		return authLoginResponse{}, 0, "", err
	}
	defer res.Body.Close()

	rawBytes, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return authLoginResponse{}, res.StatusCode, "", err
	}
	raw := string(rawBytes)

	var parsed authLoginResponse
	if len(rawBytes) > 0 {
		_ = json.Unmarshal(rawBytes, &parsed)
	}
	return parsed, res.StatusCode, raw, nil
}

func authErrorMessage(resp authLoginResponse, raw string) string {
	if msg := strings.TrimSpace(resp.Message); msg != "" {
		return msg
	}
	if err := strings.TrimSpace(resp.Error); err != "" {
		return err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "empty response"
	}
	if len(raw) > 240 {
		return raw[:240]
	}
	return raw
}
