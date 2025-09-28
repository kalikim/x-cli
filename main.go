package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kalikim/x-cli/config"
	"github.com/spf13/cobra"
)

const (
	mediaUploadEndpoint = "https://upload.twitter.com/1.1/media/upload.json"
	tweetEndpoint       = "https://api.twitter.com/2/tweets"
)

type tweetPayload struct {
	Text  string           `json:"text"`
	Media *tweetMediaBlock `json:"media,omitempty"`
}

type tweetMediaBlock struct {
	MediaIDs []string `json:"media_ids"`
}

func main() {
	var text string
	var image string

	rootCmd := &cobra.Command{
		Use:   "x-cli",
		Short: "Post to X (Twitter) from your terminal 🚀",
		RunE: func(cmd *cobra.Command, args []string) error {
			text = strings.TrimSpace(text)
			if text == "" {
				return errors.New("text flag cannot be empty")
			}

			cfg := config.LoadConfig()
			if err := cfg.Validate(); err != nil {
				return err
			}

			client := &http.Client{Timeout: 20 * time.Second}

			var mediaIDs []string
			if image != "" {
				id, err := uploadMedia(client, cfg, image)
				if err != nil {
					return err
				}
				mediaIDs = append(mediaIDs, id)
			}

			if err := postTweet(client, cfg, text, mediaIDs); err != nil {
				return err
			}

			if len(mediaIDs) > 0 {
				fmt.Println("✅ Tweet with media posted successfully!")
			} else {
				fmt.Println("✅ Tweet posted successfully!")
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&text, "text", "t", "", "Tweet text")
	rootCmd.Flags().StringVarP(&image, "image", "i", "", "Path to image file")
	rootCmd.MarkFlagRequired("text")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func postTweet(client *http.Client, cfg config.Config, text string, mediaIDs []string) error {
	payload := tweetPayload{Text: text}
	if len(mediaIDs) > 0 {
		payload.Media = &tweetMediaBlock{MediaIDs: mediaIDs}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding tweet payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, tweetEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating tweet request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	header, err := buildOAuth1Header(http.MethodPost, tweetEndpoint, nil, cfg)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", header)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("posting tweet: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading tweet response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("twitter API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func uploadMedia(client *http.Client, cfg config.Config, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading media: %w", err)
	}

	mimeType := detectMime(path, data)

	params := map[string]string{
		"media_data": base64.StdEncoding.EncodeToString(data),
	}

	if strings.HasPrefix(mimeType, "image/") {
		params["media_category"] = "tweet_image"
	}

	body, err := signedPost(client, cfg, mediaUploadEndpoint, params)
	if err != nil {
		return "", fmt.Errorf("uploading media: %w", err)
	}

	var resp struct {
		MediaIDString string `json:"media_id_string"`
		Error         struct {
			Message string `json:"message"`
		} `json:"error"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decoding media upload response: %w", err)
	}

	if resp.MediaIDString == "" {
		switch {
		case resp.Error.Message != "":
			return "", fmt.Errorf("media upload failed: %s", resp.Error.Message)
		case len(resp.Errors) > 0 && resp.Errors[0].Message != "":
			return "", fmt.Errorf("media upload failed: %s", resp.Errors[0].Message)
		default:
			return "", fmt.Errorf("media upload failed: %s", string(body))
		}
	}

	return resp.MediaIDString, nil
}

func signedPost(client *http.Client, cfg config.Config, endpoint string, params map[string]string) ([]byte, error) {
	body := encodeParams(params)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	header, err := buildOAuth1Header(http.MethodPost, endpoint, params, cfg)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", header)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("twitter API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return responseBody, nil
}

func buildOAuth1Header(method, rawURL string, params map[string]string, cfg config.Config) (string, error) {
	nonce, err := generateNonce()
	if err != nil {
		return "", err
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	oauthParams := map[string]string{
		"oauth_consumer_key":     cfg.APIKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            cfg.AccessToken,
		"oauth_version":          "1.0",
	}

	baseURL, queryParams, err := normalizeURL(rawURL)
	if err != nil {
		return "", err
	}

	signingValues := url.Values{}
	for k, v := range params {
		signingValues.Add(k, v)
	}
	for k, v := range oauthParams {
		signingValues.Add(k, v)
	}
	for k, vs := range queryParams {
		for _, v := range vs {
			signingValues.Add(k, v)
		}
	}

	parameterString := encodeValues(signingValues)
	baseString := strings.ToUpper(method) + "&" + percentEncode(baseURL) + "&" + percentEncode(parameterString)
	signingKey := percentEncode(cfg.APISecret) + "&" + percentEncode(cfg.AccessSecret)

	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	oauthParams["oauth_signature"] = signature

	headerValues := make([]string, 0, len(oauthParams))
	keys := make([]string, 0, len(oauthParams))
	for k := range oauthParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		headerValues = append(headerValues, fmt.Sprintf("%s=\"%s\"", percentEncode(k), percentEncode(oauthParams[k])))
	}

	return "OAuth " + strings.Join(headerValues, ", "), nil
}

func encodeParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	values := url.Values{}
	for k, v := range params {
		values.Add(k, v)
	}

	return values.Encode()
}

func encodeValues(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	first := true
	for _, k := range keys {
		vs := values[k]
		sort.Strings(vs)
		for _, v := range vs {
			if !first {
				builder.WriteByte('&')
			}
			first = false
			builder.WriteString(percentEncode(k))
			builder.WriteByte('=')
			builder.WriteString(percentEncode(v))
		}
	}

	return builder.String()
}

func percentEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func generateNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func normalizeURL(raw string) (string, url.Values, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("parsing URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Host)
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}

	base := scheme + "://" + host + path

	return base, parsed.Query(), nil
}

func detectMime(path string, data []byte) string {
	if ext := filepath.Ext(path); ext != "" {
		if typ := mime.TypeByExtension(ext); typ != "" {
			return typ
		}
	}

	return http.DetectContentType(data)
}
