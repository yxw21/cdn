package cdn

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type provider interface {
	FetchIPRanges() ([]string, error)
	FetchIPRangesWithCache(provider) ([]string, error)
}

const (
	Akamai     = "akamai"
	Bunny      = "bunny"
	CacheFly   = "cachefly"
	CloudFlare = "cloudflare"
	CloudFront = "cloudfront"
	Fastly     = "fastly"
	GCore      = "gcore"
	Google     = "google"
	Key        = "key"
	Quic       = "quic"
)

var Providers = make(map[string]provider)

type cacheData struct {
	Timestamp int64
	IPRanges  []string
}

type cacheManager struct {
	providerName string
}

func (cm *cacheManager) filePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	fileName := fmt.Sprintf(".%s.cdn.ip.range", cm.providerName)
	return filepath.Join(homeDir, fileName), nil
}

func (cm *cacheManager) read() ([]string, error) {
	var cache cacheData
	path, err := cm.filePath()
	if err != nil {
		return cache.IPRanges, err
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return cache.IPRanges, err
	}
	err = json.Unmarshal(file, &cache)
	if err != nil {
		return cache.IPRanges, err
	}
	if time.Now().Unix()-cache.Timestamp > 7*24*60*60 {
		return cache.IPRanges, fmt.Errorf("cache expired")
	}
	return cache.IPRanges, nil
}

func (cm *cacheManager) write(data []string) error {
	path, err := cm.filePath()
	if err != nil {
		return err
	}
	cache := cacheData{
		Timestamp: time.Now().Unix(),
		IPRanges:  data,
	}
	file, err := json.MarshalIndent(cache, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, file, 0644)
}

func newCacheManager(providerName string) *cacheManager {
	return &cacheManager{providerName: providerName}
}

type defaultProvider struct {
	cache *cacheManager
}

func (dp defaultProvider) processLines(lines []string) []string {
	var result []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		line = strings.Trim(line, "\r\t ")
		result = append(result, line)
	}
	return result
}

func (dp defaultProvider) FetchIPRangesWithCache(p provider) ([]string, error) {
	lines, err := dp.cache.read()
	if len(lines) > 0 && err == nil {
		return lines, nil
	} else {
		ipRanges, err := p.FetchIPRanges()
		if err != nil {
			return nil, err
		}
		if len(ipRanges) > 0 {
			err = dp.cache.write(ipRanges)
			if err != nil {
				return nil, err
			}
		}
		return ipRanges, nil
	}
}

type akamai struct{ defaultProvider }

func (a akamai) FetchIPRanges() ([]string, error) {
	var result []string
	req, err := http.NewRequest("GET", "https://techdocs.akamai.com/origin-ip-acl/docs/update-your-origin-server", nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return result, err
	}
	ips := doc.Find(".rdmd-code").Eq(0).Text()
	result = strings.Split(ips, "\n")
	result = a.processLines(result)
	return result, nil
}

func newAkamai() *akamai {
	return &akamai{defaultProvider: defaultProvider{
		cache: newCacheManager(Akamai),
	}}
}

type bunny struct{ defaultProvider }

func (b bunny) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://api.bunny.net/system/edgeserverlist/plain")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	result = strings.Split(string(bs), "\n")
	result = b.processLines(result)
	return result, nil
}

func newBunny() *bunny {
	return &bunny{defaultProvider: defaultProvider{
		cache: newCacheManager(Bunny),
	}}
}

type cacheFly struct{ defaultProvider }

func (c cacheFly) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://cachefly.cachefly.net/ips/cdn.txt")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	result = strings.Split(string(bs), "\n")
	result = c.processLines(result)
	return result, nil
}

func newCacheFly() *cacheFly {
	return &cacheFly{defaultProvider: defaultProvider{
		cache: newCacheManager(CacheFly),
	}}
}

type cloudFlare struct{ defaultProvider }

func (c cloudFlare) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://www.cloudflare.com/ips-v4")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	result = strings.Split(string(bs), "\n")
	result = c.processLines(result)
	return result, nil
}

func newCloudFlare() *cloudFlare {
	return &cloudFlare{defaultProvider: defaultProvider{
		cache: newCacheManager(CloudFlare),
	}}
}

type cloudFront struct{ defaultProvider }

func (c cloudFront) FetchIPRanges() ([]string, error) {
	var (
		result []string
		data   = make(map[string][]string)
	)
	resp, err := http.Get("https://d7uri8nf7uskq.cloudfront.net/tools/list-cloudfront-ips")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return result, err
	}
	result = data["CLOUDFRONT_GLOBAL_IP_LIST"]
	result = c.processLines(result)
	return result, nil
}

func newCloudFront() *cloudFront {
	return &cloudFront{defaultProvider: defaultProvider{
		cache: newCacheManager(CloudFront),
	}}
}

type fastly struct {
	defaultProvider
	Addresses []string
}

func (f fastly) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://api.fastly.com/public-ip-list")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&f)
	if err != nil {
		return result, err
	}
	result = f.processLines(f.Addresses)
	return result, nil
}

func newFastly() *fastly {
	return &fastly{defaultProvider: defaultProvider{
		cache: newCacheManager(Fastly),
	}}
}

type google struct {
	defaultProvider
	Prefixes []struct {
		IPv4Prefix string
	}
}

func (g google) FetchIPRanges() ([]string, error) {
	var result []string
	r := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.gstatic.com/ipranges/cloud.json", nil)
	if err != nil {
		return result, err
	}
	resp, err := r.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&g)
	if err != nil {
		return result, err
	}
	for _, item := range g.Prefixes {
		result = append(result, item.IPv4Prefix)
	}
	result = g.processLines(result)
	return result, nil
}

func newGoogle() *google {
	return &google{defaultProvider: defaultProvider{
		cache: newCacheManager(Google),
	}}
}

type gCore struct {
	defaultProvider
	Addresses []string
}

func (g gCore) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://api.gcore.com/cdn/public-ip-list")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&g)
	if err != nil {
		return result, err
	}
	result = g.processLines(g.Addresses)
	return result, nil
}

func newGCore() *gCore {
	return &gCore{defaultProvider: defaultProvider{
		cache: newCacheManager(GCore),
	}}
}

type key struct {
	defaultProvider
	Prefixes []string
}

func (k key) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://www.keycdn.com/shield-prefixes.json")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&k)
	if err != nil {
		return result, err
	}
	result = k.processLines(k.Prefixes)
	return result, nil
}

func newKey() *key {
	return &key{defaultProvider: defaultProvider{
		cache: newCacheManager(Key),
	}}
}

type qUic struct{ defaultProvider }

func (q qUic) FetchIPRanges() ([]string, error) {
	var result []string
	resp, err := http.Get("https://quic.cloud/ips")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	result = strings.Split(string(bs), "<br />")
	result = q.processLines(result)
	return result, nil
}

func newQUic() *qUic {
	return &qUic{defaultProvider: defaultProvider{
		cache: newCacheManager(Quic),
	}}
}

func GetProvider(name string) (provider, error) {
	provider, exists := Providers[name]
	if !exists {
		return nil, fmt.Errorf("CDN provider not found: %s", name)
	}
	return provider, nil
}

func PreCache() {
	for _, pro := range Providers {
		_, _ = pro.FetchIPRangesWithCache(pro)
	}
}

func QueryName(ip net.IP) string {
	var wg sync.WaitGroup
	resultChan := make(chan string, len(Providers))
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	for name, pro := range Providers {
		wg.Add(1)
		go func(name string, pro provider) {
			defer wg.Done()
			ipRanges, err := pro.FetchIPRangesWithCache(pro)
			if err != nil {
				return
			}
			for _, rangeOrIP := range ipRanges {
				_, cidr, err := net.ParseCIDR(rangeOrIP)
				if err != nil {
					if rangeOrIP == ip.String() {
						resultChan <- name
						return
					}
				} else {
					if cidr.Contains(ip) {
						resultChan <- name
						return
					}
				}
			}
		}(name, pro)
	}
	select {
	case result := <-resultChan:
		return result
	case <-done:
		return ""
	}
}

func init() {
	Providers[Akamai] = newAkamai()
	Providers[Bunny] = newBunny()
	Providers[CacheFly] = newCacheFly()
	Providers[CloudFlare] = newCloudFlare()
	Providers[CloudFront] = newCloudFront()
	Providers[Fastly] = newFastly()
	Providers[GCore] = newGCore()
	Providers[Google] = newGoogle()
	Providers[Key] = newKey()
	Providers[Quic] = newQUic()
}
