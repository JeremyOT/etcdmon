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
func RegisterService(etcdHost, keyPath, value string, ttl, interval time.Duration, quit chan int) {
	etcdUrl, err := url.Parse(etcdHost)
	if err != nil {
		log.Println("Bad etcd host:", err)
		return
	}
	etcdUrl.Path = path.Join(etcdUrl.Path, keyPath)
	values := url.Values{}
	values.Set("ttl", strconv.Itoa(int(ttl/time.Second)))
	values.Set("value", value)
	body := values.Encode()
	contentType := "application/x-www-form-urlencoded"
	urlString := etcdUrl.String()
	if err := putToUrl(urlString, body, contentType); err != nil {
		log.Println("Error updating etcd:", err)
	}
	clock := time.Tick(interval)
	for {
		select {
		case <-clock:
			if err := putToUrl(urlString, body, contentType); err != nil {
				log.Println("Error updating etcd:", err)
			}
		case <-quit:
			return
		}
	}
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
