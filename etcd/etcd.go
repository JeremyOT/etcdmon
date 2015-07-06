package etcd

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Registry struct {
	interval time.Duration
	ttl      time.Duration
	etcdHost string
	keyPath  string
	value    string
	quit     chan struct{}
	done     chan struct{}
}

func NewRegistry(etcdHost, keyPath, value string, ttl, interval time.Duration) *Registry {
	return &Registry{
		etcdHost: etcdHost,
		keyPath:  keyPath,
		value:    value,
		interval: interval,
		ttl:      ttl,
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

func ListServices(etcdHost, keyPath string) (nodes []*EtcdNode, err error) {
	etcdUrl, err := url.Parse(etcdHost)
	if err != nil {
		log.Println("Bad etcd host:", err)
		return
	}
	etcdUrl.Path = path.Join(etcdUrl.Path, keyPath)
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
	nodes = response.Node.Nodes
	return
}
