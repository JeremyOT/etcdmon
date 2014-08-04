package etcd

import (
	"fmt"
	"log"
	"net"
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
	clock := time.Tick(time.Duration(interval) * time.Second)
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

// Make a connection to testUrl to infer the local host address. Returns
// the address of the interface that was used for the connection.
func LocalHost(testUrl string) (host string, err error) {
	if strings.HasPrefix(testUrl, "http://") {
		testUrl = testUrl[7:]
	} else if strings.HasPrefix(testUrl, "https://") {
		testUrl = testUrl[8:]
	}
	if !strings.ContainsRune(testUrl, ':') {
		testUrl = testUrl + ":80"
	}
	if conn, err := net.Dial("udp", fmt.Sprintf("%s", testUrl)); err != nil {
		return "", err
	} else {
		defer conn.Close()
		host = strings.Split(conn.LocalAddr().String(), ":")[0]
	}
	return
}
