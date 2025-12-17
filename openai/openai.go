package openai

import (
	"net/http"
	"sync/atomic"
	"time"
)

const ProviderName = "openai"

type Config struct {
	APIKey     string
	BaseURL    string
	APIPrefix  string
	Headers    map[string]string
	HTTPClient *http.Client

	MaxRetries int
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: normalizeConfig(cfg)}
}

var defaultClient atomic.Pointer[Client]

func init() {
	defaultClient.Store(NewClient(Config{}))
}

func Configure(cfg Config) {
	defaultClient.Store(NewClient(cfg))
}

func Chat(modelName string) ModelRef {
	return defaultClient.Load().Chat(modelName)
}

func (c *Client) Chat(modelName string) ModelRef {
	return ModelRef{
		modelName: modelName,
		client:    c,
	}
}

func Embed(modelName string) ModelRef {
	return defaultClient.Load().Embed(modelName)
}

func (c *Client) Embed(modelName string) ModelRef {
	return ModelRef{
		modelName: modelName,
		client:    c,
	}
}

func Image(modelName string) ModelRef {
	return defaultClient.Load().Image(modelName)
}

func (c *Client) Image(modelName string) ModelRef {
	return ModelRef{
		modelName: modelName,
		client:    c,
	}
}

func Transcription(modelName string) ModelRef {
	return defaultClient.Load().Transcription(modelName)
}

func (c *Client) Transcription(modelName string) ModelRef {
	return ModelRef{
		modelName: modelName,
		client:    c,
	}
}

func Speech(modelName string) ModelRef {
	return defaultClient.Load().Speech(modelName)
}

func (c *Client) Speech(modelName string) ModelRef {
	return ModelRef{
		modelName: modelName,
		client:    c,
	}
}

type ModelRef struct {
	modelName string
	client    *Client
}

func (m ModelRef) Provider() string { return ProviderName }
func (m ModelRef) Name() string     { return m.modelName }

func (m ModelRef) Client() *Client { return m.client }

func (c *Client) Config() Config { return c.cfg }

func normalizeConfig(cfg Config) Config {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	if cfg.APIPrefix == "" {
		cfg.APIPrefix = "/v1"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 2
	}
	if cfg.MinBackoff == 0 {
		cfg.MinBackoff = 250 * time.Millisecond
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = 5 * time.Second
	}
	return cfg
}
