package avatar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxAttempts = 4

type InfraiClient struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type imageData struct {
	ID string `json:"id"`
}

func NewInfraiClient(baseURL, key string, client *http.Client) *InfraiClient {
	return &InfraiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    client,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *InfraiClient) Upload(ctx context.Context, content io.Reader, filename, requestID string) (string, error) {
	file, err := io.ReadAll(content)
	if err != nil {
		return "", err
	}
	body, contentType, err := multipartBody(map[string]string{"filename": filename}, "file", filename, file)
	if err != nil {
		return "", err
	}
	return c.imageRequest(ctx, http.MethodPost, "/v1/image/upload", body, contentType, requestID)
}

func (c *InfraiClient) SmartCrop(ctx context.Context, image, aspect, requestID string) (string, error) {
	body, contentType, err := multipartBody(map[string]string{"image": image, "aspect": aspect}, "", "", nil)
	if err != nil {
		return "", err
	}
	return c.imageRequest(ctx, http.MethodPost, "/v1/image/smart_crop", body, contentType, requestID)
}

func (c *InfraiClient) Resize(ctx context.Context, image string, opts ResizeOptions, requestID string) (string, error) {
	fields := map[string]string{
		"image": image, "width": strconv.Itoa(opts.Width), "height": strconv.Itoa(opts.Height),
		"fit": opts.Fit, "enlarge": strconv.FormatBool(opts.Enlarge), "format": opts.Format,
		"store": strconv.FormatBool(opts.Store),
	}
	body, contentType, err := multipartBody(fields, "", "", nil)
	if err != nil {
		return "", err
	}
	return c.imageRequest(ctx, http.MethodPost, "/v1/image/resize", body, contentType, requestID)
}

func (c *InfraiClient) UpdateUser(ctx context.Context, userID string, metadata map[string]any, requestID string) error {
	body, err := json.Marshal(map[string]any{"user_id": userID, "metadata": metadata})
	if err != nil {
		return err
	}
	path := "/v1/auth/user/update/" + url.PathEscape(userID)
	_, err = c.request(ctx, http.MethodPatch, path, body, "application/json", requestID)
	return err
}

func (c *InfraiClient) imageRequest(ctx context.Context, method, path string, body []byte, contentType, requestID string) (string, error) {
	env, err := c.request(ctx, method, path, body, contentType, requestID)
	if err != nil {
		return "", err
	}
	var data imageData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("decode image data: %w", err)
	}
	if data.ID == "" {
		return "", errors.New("image response did not include an id")
	}
	return data.ID, nil
}

func (c *InfraiClient) request(ctx context.Context, method, path string, body []byte, contentType, requestID string) (envelope, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return envelope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", requestID)

		res, err := c.http.Do(req)
		if err != nil {
			return envelope{}, fmt.Errorf("send request: %w", err)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return envelope{}, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(responseBody, &env)
		if decodeErr == nil && res.StatusCode == http.StatusTooManyRequests && attempt+1 < maxAttempts {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return envelope{}, err
			}
			continue
		}
		if decodeErr != nil {
			return envelope{}, fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return envelope{}, fmt.Errorf("Infrai request returned HTTP %d", res.StatusCode)
		}
		if !env.OK && env.Error != nil {
			return envelope{}, &InfraiError{Code: env.Error.Code, Message: env.Error.Message, HTTPStatus: res.StatusCode}
		}
		if !env.OK {
			return envelope{}, &InfraiError{Message: "request rejected", HTTPStatus: res.StatusCode}
		}
		return env, nil
	}
	return envelope{}, errors.New("retry limit reached")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func multipartBody(fields map[string]string, fileField, filename string, file []byte) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", err
		}
	}
	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, filename)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(file); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}
