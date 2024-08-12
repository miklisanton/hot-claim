package main

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/andybalholm/brotli"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const rpcURL = "https://rpc.mainnet.near.org"

type RPCResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Result      []byte   `json:"result"`
		Logs        []string `json:"logs"`
		BlockHeight int64    `json:"block_height"`
		BlockHash   string   `json:"block_hash"`
	} `json:"result"`
	ID string `json:"id"`
}

type Payload struct {
	GameState *GameState `json:"game_state"`
}

type GameState struct {
	Refferals int     `json:"refferals"`
	Inviter   string  `json:"inviter"`
	Village   *string `json:"village"` // Use pointer to handle null values
	LastClaim int64   `json:"last_claim"`
	Firespace int     `json:"firespace"`
	Boost     int     `json:"boost"`
	Storage   int     `json:"storage"`
	Balance   int     `json:"balance"`
}

type ProxyChangeResponse struct {
	ProxyID int `json:"proxy_id"`
}

type ProxyDetailsResponse struct {
	Pass  string `json:"proxy_pass"`
	Login string `json:"proxy_login"`
	IP    string `json:"proxy_host_ip"`
	Port  string `json:"proxy_http_port"`
}

type ProxyClient struct {
	client   *http.Client
	proxyUrl *url.URL
}

type Headers struct {
	DeviceID      string `json:"device_id"`
	Authorization string `json:"authorization"`
	TelegramData  string `json:"telegram_data"`
	UserAgent     string `json:"user_agent"`
	Proxy         string `json:"proxy"`
	Username      string `json:"username"`
}

type MobileData struct {
	Authorization string `json:"authorization"`
	ProxyKey      string `json:"proxy_key"`
}

type Config struct {
	Accounts    []Headers  `json:"accounts"`
	MobileProxy MobileData `json:"mobile_proxy"`
}

func LoadConfig(path string) (*Config, error) {
	jsonFile, err := os.Open(path)
	defer jsonFile.Close()
	if err != nil {
		return nil, err
	}

	config := &Config{}
	if err = json.NewDecoder(jsonFile).Decode(config); err != nil {
		return nil, err
	}
	return config, nil
}

func newProxyClient(proxyUrl string) (*ProxyClient, error) {
	if proxyUrl == "" {
		return &ProxyClient{
			client:   http.DefaultClient,
			proxyUrl: nil,
		}, nil
	}
	proxyURL, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	return &ProxyClient{
		client:   &http.Client{Transport: transport},
		proxyUrl: proxyURL,
	}, nil
}

func GetGameState(id string) (*GameState, error) {
	argsBase64 := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"account_id": "%s"}`, id)))
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "dontcare",
		"method":  "query",
		"params": map[string]interface{}{
			"request_type": "call_function",
			"finality":     "optimistic",
			"account_id":   "game.hot.tg",
			"method_name":  "get_user",
			"args_base64":  argsBase64,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	var rpcResponse RPCResponse
	err = json.NewDecoder(resp.Body).Decode(&rpcResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	var gameState GameState
	if err := json.Unmarshal(rpcResponse.Result.Result, &gameState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %v", err)
	}
	log.Printf("hot balance: %v", gameState.Balance)

	return &gameState, nil
}

func (c *ProxyClient) claimHot(headers Headers) error {
	endpoint := "https://api0.herewallet.app/api/v1/user/hot/claim"

	gameState, err := GetGameState(headers.Username)
	if err != nil {
		return fmt.Errorf("error getting game state: %v", err)
	}

	payload := Payload{
		GameState: gameState,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	body := bytes.NewReader(jsonData)

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.ContentLength = int64(body.Len())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8")
	req.Header.Set("Authorization", headers.Authorization)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DeviceId", headers.DeviceID)
	req.Header.Set("Host", "api0.herewallet.app")
	req.Header.Set("Network", "mainnet")
	req.Header.Set("Origin", "https://tgapp.herewallet.app")
	req.Header.Set("Platform", "telegram")
	req.Header.Set("Referer", "https://tgapp.herewallet.app/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Telegram-Data", headers.TelegramData)
	req.Header.Set("User-Agent", headers.UserAgent)
	req.Header.Set("is-sbt", "false")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="125", "Chromium";v="125", "Not.A/Brand";v="24"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)

	// Create a new HTTP client and send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Error sending request:", err)

	}
	defer resp.Body.Close()

	// Read and print the response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("recieved status code: %d", resp.StatusCode)
	}

	// Handle response body based on encoding
	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return err
		}
	case "deflate":
		reader, err = zlib.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating zlib reader:", err)
			return err
		}
	case "br":
		// Handle Brotli if necessary
		reader = brotli.NewReader(resp.Body)
	default:
		reader = resp.Body
	}

	// Read and print the response
	respBody, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	fmt.Println(string(respBody))

	return nil
}

func multiClaim(cfg Config) {
	log.Printf("Claiming on %d accounts", len(cfg.Accounts))
	for _, acc := range cfg.Accounts {
		if acc.Proxy == "" {
			// Use mobile proxy if normal proxy not specified
			proxy, err := getMobileProxy(cfg.MobileProxy.Authorization, cfg.MobileProxy.ProxyKey)
			if err != nil {
				log.Printf("Error getting mobile proxy: %v\n", err)
			}
			acc.Proxy = proxy
		}

		cl, err := newProxyClient(acc.Proxy)
		if err != nil {
			log.Println(err)
		}

		err = cl.claimHot(acc)
		if err != nil {
			log.Println(err)
		}
		sleep := time.Second * 20
		time.Sleep(time.Duration(rand.Intn(int(sleep))))
	}
}

func getMobileProxy(authorization, proxyKey string) (string, error) {
	endpoint := fmt.Sprintf("https://changeip.mobileproxy.space/?proxy_key=%s&format=json", proxyKey)
	resp, err := http.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	var response ProxyChangeResponse
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	return getProxyDetail(authorization, strconv.Itoa(response.ProxyID))
}

func getProxyDetail(authorization, proxyID string) (string, error) {
	endpoint := fmt.Sprintf("https://mobileproxy.space/api.html?command=get_my_proxy&proxy_id=%s", proxyID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+authorization)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	var response []ProxyDetailsResponse

	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}
	data := response[0]

	return fmt.Sprintf("http://%s:%s@%s:%s", data.Login, data.Pass, data.IP, data.Port), nil
}

func main() {
	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}
	quit := make(chan struct{})
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(125 * time.Minute)

	go func() {
		multiClaim(*cfg)

		for {
			select {
			case <-ticker.C:
				multiClaim(*cfg)
			case <-sigs:
				ticker.Stop()
				return
			}
		}
	}()

	<-sigs
	log.Println("Shutting down...")
	close(quit)
	<-quit
	log.Println("Stopped")
}
