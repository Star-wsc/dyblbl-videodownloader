package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	DownloadDir   string `json:"download_dir"`
	Proxy         string `json:"proxy"`
	MaxConcurrent int    `json:"max_concurrent"`
	SpeedLimit    int    `json:"speed_limit"`
	FileTemplate  string `json:"file_template"`
	AutoClassify  bool   `json:"auto_classify"`
	BilibiliCookie string `json:"bilibili_cookie"`
	DouyinCookie   string `json:"douyin_cookie"`
	mu             sync.RWMutex
	configFile     string
}

var appConfig *Config

func Load() *Config {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".video-downloader-cli")

	appConfig = &Config{
		DownloadDir:   "./downloads",
		MaxConcurrent: 5,
		FileTemplate:  "{platform}_{title}",
		configFile:    filepath.Join(configDir, "config.json"),
	}

	appConfig.loadFromFile()
	return appConfig
}

func (c *Config) loadFromFile() {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := os.Open(c.configFile)
	if err != nil {
		return
	}
	defer f.Close()

	var saved Config
	dec := json.NewDecoder(f)
	if err := dec.Decode(&saved); err != nil {
		return
	}

	if saved.DownloadDir != "" {
		c.DownloadDir = saved.DownloadDir
	}
	if saved.Proxy != "" {
		c.Proxy = saved.Proxy
	}
	if saved.MaxConcurrent > 0 {
		c.MaxConcurrent = saved.MaxConcurrent
	}
	if saved.SpeedLimit > 0 {
		c.SpeedLimit = saved.SpeedLimit
	}
	if saved.FileTemplate != "" {
		c.FileTemplate = saved.FileTemplate
	}
	c.AutoClassify = saved.AutoClassify
	if saved.BilibiliCookie != "" {
		c.BilibiliCookie = saved.BilibiliCookie
	}
	if saved.DouyinCookie != "" {
		c.DouyinCookie = saved.DouyinCookie
	}
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveInternal()
}

func (c *Config) saveInternal() error {
	if err := os.MkdirAll(filepath.Dir(c.configFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.configFile, data, 0600)
}

func GetConfig() *Config {
	if appConfig == nil {
		return Load()
	}
	return appConfig
}

func (c *Config) SetDownloadDir(dir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DownloadDir = dir
	return c.saveInternal()
}

func (c *Config) SetProxy(proxy string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Proxy = proxy
	return c.saveInternal()
}

func (c *Config) SetBilibiliCookie(cookie string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.BilibiliCookie = cookie
	return c.saveInternal()
}

func (c *Config) SetDouyinCookie(cookie string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DouyinCookie = cookie
	return c.saveInternal()
}

func (c *Config) SetMaxConcurrent(val int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaxConcurrent = val
	return c.saveInternal()
}

func (c *Config) SetSpeedLimit(val int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SpeedLimit = val
	return c.saveInternal()
}

func (c *Config) SetFileTemplate(val string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FileTemplate = val
	return c.saveInternal()
}

func (c *Config) SetAutoClassify(val bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AutoClassify = val
	return c.saveInternal()
}
