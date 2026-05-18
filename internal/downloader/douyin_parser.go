package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Star-wsc/dyblbl-videodownloader/internal/models"
)

// resolveShortURL 解析抖音短链接
func (d *DouyinDownloader) resolveShortURL(shortURL string) (string, error) {
	req, err := http.NewRequest("GET", shortURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// 不跟随重定向
	client := &http.Client{
		Transport: d.client.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return "", fmt.Errorf("no redirect")
}

// extractVideoID 从URL中提取视频ID
func (d *DouyinDownloader) extractVideoID(videoURL string) string {
	re := regexp.MustCompile(`douyin\.com/video/(\d+)`)
	matches := re.FindStringSubmatch(videoURL)
	if len(matches) > 1 {
		return matches[1]
	}

	re2 := regexp.MustCompile(`modal_id=(\d+)`)
	matches2 := re2.FindStringSubmatch(videoURL)
	if len(matches2) > 1 {
		return matches2[1]
	}

	return ""
}

// fetchHTML 获取页面HTML
func (d *DouyinDownloader) fetchHTML(videoURL string) (string, map[string]string, error) {
	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cookie", "msToken=abcdefg")
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	cookies := make(map[string]string)
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
	}

	return string(body), cookies, nil
}

// parseDetailAPI 调用详情API解析
func (d *DouyinDownloader) parseDetailAPI(videoID string) (*models.VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=%s&aid=6383&cookie_enabled=true&browser_language=zh-CN&browser_platform=Win32&browser_name=Chrome&browser_version=126.0.0.0", videoID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://www.douyin.com/video/%s", videoID))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 && body[0] == '<' {
		return nil, fmt.Errorf("API returned HTML instead of JSON")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	awemeDetail, ok := result["aweme_detail"].(map[string]interface{})
	if !ok || awemeDetail == nil {
		return nil, fmt.Errorf("aweme_detail not found in API response")
	}

	return d.parseAwemeDetail(awemeDetail), nil
}

// parseAwemeDetail 解析aweme_detail
func (d *DouyinDownloader) parseAwemeDetail(detail map[string]interface{}) *models.VideoInfo {
	videoData, _ := detail["video"].(map[string]interface{})
	if videoData == nil {
		return nil
	}

	videoURLs := d.extractVideoURLs(videoData)

	title, _ := detail["desc"].(string)

	author := ""
	authorID := ""
	if authorInfo, ok := detail["author"].(map[string]interface{}); ok {
		author, _ = authorInfo["nickname"].(string)
		authorID, _ = authorInfo["unique_id"].(string)
	}

	coverURL := ""
	if cover, ok := videoData["cover"].(map[string]interface{}); ok {
		if urlList, ok := cover["url_list"].([]interface{}); ok && len(urlList) > 0 {
			coverURL, _ = urlList[0].(string)
		}
	}
	if coverURL != "" && strings.HasPrefix(coverURL, "//") {
		coverURL = "https:" + coverURL
	}

	selectedURL := ""
	selectedQuality := ""
	qualityPriority := []string{"4k", "2k", "1080p", "720p", "480p"}
	for _, q := range qualityPriority {
		if u, ok := videoURLs[q]; ok && u != "" {
			selectedURL = u
			selectedQuality = q
			break
		}
	}
	if selectedURL == "" {
		for q, u := range videoURLs {
			if q != "download" && q != "4k_h265" && u != "" {
				selectedURL = u
				selectedQuality = q
				break
			}
		}
	}

	if selectedURL == "" {
		return nil
	}

	return &models.VideoInfo{
		Title:              title,
		Author:             author,
		AuthorID:           authorID,
		CoverURL:           coverURL,
		VideoURL:           selectedURL,
		SelectedQuality:    selectedQuality,
		AvailableQualities: getKeys(videoURLs),
	}
}

// extractVideoURLs 提取视频URL
func (d *DouyinDownloader) extractVideoURLs(videoData map[string]interface{}) map[string]string {
	urls := make(map[string]string)

	bitRate, _ := videoData["bit_rate"].([]interface{})
	bitRateQualities := make(map[string]string)

	for _, br := range bitRate {
		brMap, ok := br.(map[string]interface{})
		if !ok {
			continue
		}

		gearName, _ := brMap["gear_name"].(string)
		qualityType, _ := brMap["quality_type"].(float64)
		width, _ := brMap["width"].(float64)
		height, _ := brMap["height"].(float64)

		brPlayAddr, _ := brMap["play_addr"].(map[string]interface{})
		if brPlayAddr == nil {
			continue
		}

		brURLList, _ := brPlayAddr["url_list"].([]interface{})
		if len(brURLList) > 0 {
			u, _ := brURLList[0].(string)
			u = processDouyinURL(u)

			q := mapQualityAdvanced(gearName, qualityType, int(width), int(height))
			if q != "" {
				bitRateQualities[q] = u
			}
		}
	}

	for q, u := range bitRateQualities {
		urls[q] = u
	}

	playAddr, _ := videoData["play_addr"].(map[string]interface{})
	if playAddr == nil {
		playAddr, _ = videoData["playAddr"].(map[string]interface{})
	}

	if playAddr != nil {
		urlList, _ := playAddr["url_list"].([]interface{})
		if len(urlList) > 0 {
			u, _ := urlList[0].(string)
			u = processDouyinURL(u)
			if u != "" && len(urls) == 0 {
				urls["default"] = u
			}
		}
	}

	downloadAddr, _ := videoData["download_addr"].(map[string]interface{})
	if downloadAddr != nil {
		urlList, _ := downloadAddr["url_list"].([]interface{})
		if len(urlList) > 0 {
			u, _ := urlList[0].(string)
			u = processDouyinURL(u)
			if u != "" {
				urls["download"] = u
			}
		}
	}

	for _, key := range []string{"play_addr_h265", "play_addr_h264", "play_addr_bytevc1"} {
		if v, ok := videoData[key].(map[string]interface{}); ok {
			if urlList, ok := v["url_list"].([]interface{}); ok && len(urlList) > 0 {
				u, _ := urlList[0].(string)
				u = processDouyinURL(u)
				if u != "" {
					if _, exists := urls["4k"]; !exists {
						urls["4k_h265"] = u
					}
				}
			}
		}
	}

	return urls
}

// mapQualityAdvanced 映射质量名称
func mapQualityAdvanced(gearName string, qualityType float64, width, height int) string {
	if height >= 2160 || width >= 3840 {
		return "4k"
	}
	if height >= 1440 || width >= 2560 {
		return "2k"
	}
	if height >= 1080 || width >= 1920 {
		return "1080p"
	}
	if height >= 720 || width >= 1280 {
		return "720p"
	}
	if height >= 480 || width >= 854 {
		return "480p"
	}
	if height >= 360 || width >= 640 {
		return "360p"
	}

	lower := strings.ToLower(gearName)
	switch {
	case strings.Contains(lower, "4k") || strings.Contains(lower, "2160") || strings.Contains(lower, "uhd"):
		return "4k"
	case strings.Contains(lower, "2k") || strings.Contains(lower, "1440") || strings.Contains(lower, "qhd"):
		return "2k"
	case strings.Contains(lower, "1080") || strings.Contains(lower, "fhd") || strings.Contains(lower, "full_hd"):
		return "1080p"
	case strings.Contains(lower, "720") || strings.Contains(lower, "hd"):
		return "720p"
	case strings.Contains(lower, "480") || strings.Contains(lower, "sd") || strings.Contains(lower, "normal"):
		return "480p"
	case strings.Contains(lower, "360"):
		return "360p"
	}

	if qualityType >= 10 {
		return "4k"
	}
	if qualityType >= 8 {
		return "2k"
	}
	if qualityType >= 2 {
		return "1080p"
	}
	if qualityType >= 1 {
		return "720p"
	}

	return ""
}

// parseHTMLRegex HTML正则提取
func (d *DouyinDownloader) parseHTMLRegex(videoURL string) (*models.VideoInfo, error) {
	html, cookies, err := d.fetchHTML(videoURL)
	if err != nil {
		return nil, err
	}

	videoURLs := make(map[string]string)
	downloadURLs := make(map[string]string)

	patterns := []struct {
		regex   string
		urlType string
	}{
		{`"download_addr"[^}]*"url_list"\s*:\s*\["([^"]+)"`, "download"},
		{`"download"[^}]*"url_list"\s*:\s*\["([^"]+)"`, "download"},
		{`"play_addr"[^}]*"url_list"\s*:\s*\["([^"]+)"`, "play"},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 1 {
				u := decodeUnicodeURL(match[1])
				if strings.HasPrefix(u, "//") {
					u = "https:" + u
				}
				u = strings.Replace(u, "playwm", "play", -1)
				if p.urlType == "download" {
					downloadURLs["1080p"] = u
				} else {
					videoURLs["1080p"] = u
				}
				break
			}
		}
	}

	bitRateMatch := regexp.MustCompile(`"bit_rate"\s*:\s*\[([^\]]+)\]`).FindStringSubmatch(html)
	if bitRateMatch != nil {
		bitRates := bitRateMatch[1]
		qualityMap := map[string][]string{
			"4k":    {"4k", "2160p", "uhd"},
			"2k":    {"2k", "1440p", "qhd"},
			"1080p": {"1080p", "fhd", "full_hd"},
			"720p":  {"720p", "hd", "high"},
			"480p":  {"480p", "sd", "normal"},
		}

		for q, keywords := range qualityMap {
			for _, keyword := range keywords {
				pattern := fmt.Sprintf(`"gear_name"\s*:\s*"[^"]*%s[^"]*"[^}}]*"play_addr"[^}}]*"url_list"\s*:\s*\["([^"]+)"`, keyword)
				re := regexp.MustCompile(`(?i)` + pattern)
				match := re.FindStringSubmatch(bitRates)
				if len(match) > 1 {
					u := decodeUnicodeURL(match[1])
					if strings.HasPrefix(u, "//") {
						u = "https:" + u
					}
					u = strings.Replace(u, "playwm", "play", -1)
					videoURLs[q] = u
					break
				}
			}
		}
	}

	allURLs := make(map[string]string)
	for k, v := range videoURLs {
		allURLs[k] = v
	}
	for k, v := range downloadURLs {
		allURLs[k] = v
	}

	if len(allURLs) == 0 {
		return nil, fmt.Errorf("no video URL found in HTML")
	}

	selectedURL := ""
	qualityPriority := []string{"4k", "2k", "1080p", "720p", "480p"}
	for _, q := range qualityPriority {
		if u, ok := allURLs[q]; ok && u != "" {
			selectedURL = u
			break
		}
	}
	if selectedURL == "" {
		for _, u := range allURLs {
			selectedURL = u
			break
		}
	}

	descMatch := regexp.MustCompile(`"desc"\s*:\s*"([^"]*)"`).FindStringSubmatch(html)
	authorMatch := regexp.MustCompile(`"nickname"\s*:\s*"([^"]*)"`).FindStringSubmatch(html)
	coverMatch := regexp.MustCompile(`"cover"[^}]*"url_list"\s*:\s*\["([^"]+)"`).FindStringSubmatch(html)

	title := ""
	if len(descMatch) > 1 {
		title = descMatch[1]
	}

	author := "未知作者"
	if len(authorMatch) > 1 {
		author = authorMatch[1]
	}

	coverURL := ""
	if len(coverMatch) > 1 {
		coverURL = decodeUnicodeURL(coverMatch[1])
		if strings.HasPrefix(coverURL, "//") {
			coverURL = "https:" + coverURL
		}
	}

	selectedQuality := ""
	for q := range allURLs {
		if q != "download" && q != "4k_h265" {
			selectedQuality = q
			break
		}
	}

	return &models.VideoInfo{
		Title:              title,
		Author:             author,
		CoverURL:           coverURL,
		VideoURL:           selectedURL,
		SelectedQuality:    selectedQuality,
		AvailableQualities: getKeys(allURLs),
		Cookies:            cookies,
	}, nil
}

// processDouyinURL 处理抖音视频URL
func processDouyinURL(u string) string {
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	u = strings.Replace(u, "playwm", "play", -1)
	re := regexp.MustCompile(`[?&]watermark=\d+`)
	u = re.ReplaceAllString(u, "")
	re2 := regexp.MustCompile(`[?&]ratio=\w+`)
	u = re2.ReplaceAllString(u, "")
	return u
}

// decodeUnicodeURL 解码Unicode URL
func decodeUnicodeURL(u string) string {
	if !strings.Contains(u, "\\u") {
		return u
	}
	var result strings.Builder
	for i := 0; i < len(u); i++ {
		if i+5 < len(u) && u[i:i+2] == "\\u" {
			var code int
			fmt.Sscanf(u[i+2:i+6], "%x", &code)
			result.WriteRune(rune(code))
			i += 5
		} else if u[i] == '\\' {
			continue
		} else {
			result.WriteByte(u[i])
		}
	}
	return result.String()
}

// ParseVideoWithHTML 解析抖音视频（支持HTML解析）
func (d *DouyinDownloader) ParseVideoWithHTML(videoURL string) (*models.VideoInfo, error) {
	// 解析短链接
	if strings.Contains(videoURL, "v.douyin.com") {
		resolved, err := d.resolveShortURL(videoURL)
		if err == nil && resolved != "" {
			videoURL = resolved
		}
	}

	videoID := d.extractVideoID(videoURL)
	fullURL := videoURL
	if videoID != "" {
		fullURL = fmt.Sprintf("https://www.douyin.com/video/%s", videoID)
	}

	// 策略1: 调用详情API
	if videoID != "" {
		info, err := d.parseDetailAPI(videoID)
		if err == nil && info.VideoURL != "" {
			return info, nil
		}
	}

	// 策略2: HTML正则提取
	info, err := d.parseHTMLRegex(fullURL)
	if err == nil && info.VideoURL != "" {
		return info, nil
	}

	return nil, fmt.Errorf("all HTML parse strategies failed")
}
