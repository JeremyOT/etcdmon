package etcd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/JeremyOT/address/lookup"
)

const DefaultAPIRoot = "/v2/keys"

type Registry struct {
	interval time.Duration
	ttl      time.Duration
	etcdHost string
	keyPath  string
	value    string
	quit     chan struct{}
	done     chan struct{}
}

type Config struct {
	EtcdHost       string
	APIRoot        string
	Key            string
	KeyPath        string
	Value          string
	Port           int
	Host           string
	Tag            string
	StartTime      string
	TTL            time.Duration
	UpdateInterval time.Duration
}

type Service struct {
	Host       string    `json:"host"`
	Port       int       `json:"port,omitempty"`
	StartTime  string    `json:"start_time,omitempty"`
	Tag        string    `json:"tag,omitempty"`
	RawValue   string    `json:"-"`
	Expiration time.Time `json:"-"`
}

func (s *Service) String() string {
	if s.Host != "" {
		str := "Node: " + s.Host
		if s.Port > 0 {
			str += fmt.Sprintf(":%d", s.Port)
		}
		if s.Tag != "" {
			str += " Tag: " + s.Tag
		}
		if s.StartTime != "" {
			str += " Started: " + s.StartTime
		}
		return str
	}
	return s.RawValue
}

func (s *Service) Address() string {
	if s.Port > 0 {
		return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	}
	return s.Host
}

func ParseService(node *EtcdNode) Service {
	var service Service
	json.Unmarshal([]byte(node.Value), &service)
	service.RawValue = node.Value
	service.Expiration = node.Expiration
	return service
}

func (c Config) Populate() (populated Config, err error) {
	populated = Config{
		EtcdHost:       c.EtcdHost,
		Key:            c.Key,
		APIRoot:        c.APIRoot,
		KeyPath:        c.KeyPath,
		Value:          c.Value,
		Port:           c.Port,
		Host:           c.Host,
		TTL:            c.TTL,
		Tag:            c.Tag,
		StartTime:      c.StartTime,
		UpdateInterval: c.UpdateInterval,
	}
	if populated.APIRoot == "" {
		populated.APIRoot = DefaultAPIRoot
	}
	if populated.KeyPath == "" {
		populated.KeyPath = path.Join(populated.APIRoot, populated.Key)
	}
	if populated.Host == "" {
		populated.Host, err = lookup.LocalAddress(populated.EtcdHost)
		if err != nil {
			return
		}
	}
	populated.KeyPath = FormatKey(populated.KeyPath, populated.Host, populated.Port)
	populated.Value = FormatValue(populated.KeyPath, populated.Host, populated.Port, populated.Tag, populated.StartTime)
	return
}

func FormatValue(value, host string, port int, tag, startTime string) string {
	if startTime == "" {
		startTime = time.Now().Format(time.RFC3339)
	}
	if value == "" {
		service := Service{Host: host, Port: port, StartTime: startTime, Tag: tag}
		data, _ := json.Marshal(service)
		return string(data)
	}
	value = strings.Replace(value, "%H", host, -1)
	value = strings.Replace(value, "%P", strconv.Itoa(port), -1)
	value = strings.Replace(value, "%S", startTime, -1)
	value = strings.Replace(value, "%T", tag, -1)
	return value
}

func FormatKey(key, host string, port int) string {
	return strings.Replace(strings.Replace(key, "%H", host, -1), "%P", strconv.Itoa(port), -1)
}

func NewRegistry(config Config) *Registry {
	return &Registry{
		etcdHost: config.EtcdHost,
		keyPath:  config.KeyPath,
		value:    config.Value,
		interval: config.UpdateInterval,
		ttl:      config.TTL,
	}
}

func putToUrl(targetUrl, body, contentType string) error {
	if request, err := http.NewRequest("PUT", targetUrl, strings.NewReader(body)); err != nil {
		return err
	} else {
		request.Header.Set("Content-Type", contentType)
		if resp, err := http.DefaultClient.Do(request); err != nil {
			return err
		} else {
			defer resp.Body.Close()
		}
	}
	return nil
}

// Periodically put the specified value to etcdHost at keyPath. Will put data every
// interval seconds with the specified ttl until quit is closed.
func (r *Registry) Start() {
	r.quit = make(chan struct{})
	r.done = make(chan struct{})
	go r.registerService()
}

func (r *Registry) registerService() {
	etcdUrl, err := url.Parse(r.etcdHost)
	if err != nil {
		log.Println("Bad etcd host:", err)
		return
	}
	etcdUrl.Path = path.Join(etcdUrl.Path, r.keyPath)
	values := url.Values{}
	values.Set("ttl", strconv.Itoa(int(r.ttl/time.Second)))
	values.Set("value", r.value)
	body := values.Encode()
	contentType := "application/x-www-form-urlencoded"
	urlString := etcdUrl.String()
	if err := putToUrl(urlString, body, contentType); err != nil {
		log.Println("Error updating etcd:", err)
	}
	clock := time.Tick(r.interval)
	for {
		select {
		case <-clock:
			if err := putToUrl(urlString, body, contentType); err != nil {
				log.Println("Error updating etcd:", err)
			}
		case <-r.quit:
			close(r.done)
			return
		}
	}
}

func (r *Registry) Stop() {
	close(r.quit)
	r.Wait()
}

func (r *Registry) SafeStop() {
	r.Stop()
	time.Sleep(2 * r.ttl)
}

func (r *Registry) Wait() {
	<-r.done
}

type EtcdNode struct {
	Key           string      `json:"key"`
	Directory     bool        `json:"dir"`
	Nodes         []*EtcdNode `json:"nodes"`
	Value         string      `json:"value"`
	ModifiedIndex int         `json:"modifiedIndex"`
	CreatedIndex  int         `json:"createdIndex"`
	Expiration    time.Time   `json:"expiration"`
}

type EtcdResponse struct {
	Action string    `json:"action"`
	Node   *EtcdNode `json:"node"`
}

func ListServices(config Config) (services []*Service, err error) {
	config, err = config.Populate()
	if err != nil {
		return
	}
	etcdUrl, err := url.Parse(config.EtcdHost)
	if err != nil {
		log.Println("Bad etcd host:", err)
		return
	}
	etcdUrl.Path = path.Join(etcdUrl.Path, config.KeyPath)
	resp, err := http.Get(etcdUrl.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response EtcdResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	nodes := response.Node.Nodes
	services = make([]*Service, 0, len(nodes))
	for _, n := range nodes {
		var service = ParseService(n)
		services = append(services, &service)
	}
	return
}
