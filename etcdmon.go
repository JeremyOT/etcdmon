package main

import (
	"flag"
	"fmt"
	"github.com/JeremyOT/etcdmon/cmd"
	"github.com/JeremyOT/etcdmon/etcd"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

var etcAddress = flag.String("etcd", "http://127.0.0.1:4001", "The url of the etcd instance to connect to.")
var etcKey = flag.String("key", "", "The key path to post to. %H will be replaced with the current host and %P with any configured port. e.g. process/%H:%P")
var updateInterval = flag.Int("interval", 15, "The number of seconds between each poll to etcd.")
var ttl = flag.Int("ttl", 30, "The number of seconds that the key should stay alive after no polls are received.")
var host = flag.String("host", "", "The that should be used to identify this daemon. If not set, etcdmon will attempt to determine it automatically.")
var port = flag.Int("port", 0, "A port, if any that will be set along with the host name.")
var value = flag.String("value", "", "The the value to sent to etcd. Like path, %H and %P can be used for automatic replacement. If not set, the host (and port if set) will be used as the value.")
var apiRoot = flag.String("api", "/v2/keys", "The root value for any key path.")
var getHost = flag.Bool("gethost", false, "Print the current hostname (using -etcd for lookup) and exit.")

func formatValue(value, host string, port int) string {
	if value == "" {
		if port > 0 {
			return fmt.Sprintf("%s:%d", host, port)
		} else {
			return host
		}
	}
	return strings.Replace(strings.Replace(value, "%H", host, -1), "%P", strconv.Itoa(port), -1)
}

func formatKey(key, host string, port int) string {
	return strings.Replace(strings.Replace(key, "%H", host, -1), "%P", strconv.Itoa(port), -1)
}

func main() {
	flag.Parse()
	if *getHost {
		if serviceHost, err := etcd.LocalHost(*etcAddress); err != nil {
			log.Println("Error retrieving host:", err)
			os.Exit(1)
		} else {
			fmt.Print(serviceHost)
			os.Exit(0)
		}
	}
	args := flag.Args()
	if *etcAddress == "" || *etcKey == "" {
		fmt.Println("Usage:")
		fmt.Println("  Updates -key in etcd periodically until either exited or a monitored process exits.")
		fmt.Println("  To monitor an external command add the command argurments after all etcdmon options.")
		fmt.Println("  E.g. etcdmon -key process -- my_script.sh arg1 arg2")
		fmt.Println("  stdin, stdout and stderr will be piped through from the monitored command.")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}
	serviceHost := *host
	if serviceHost == "" {
		var err error
		serviceHost, err = etcd.LocalHost(*etcAddress)
		if err != nil {
			log.Println("Error retrieving host. Try setting it manually with -host=''.", err)
			os.Exit(1)
		}
	}
	keyPath := formatKey(path.Join(*apiRoot, *etcKey), serviceHost, *port)
	formattedValue := formatValue(*value, serviceHost, *port)
	fmt.Println("Etcd:", *etcAddress, "Key:", keyPath, "Value:", formattedValue, "TTL:", *ttl, "UpdateInterval:", *updateInterval)
	quit := make(chan int)
	if len(args) == 0 {
		fmt.Println("No command specified. Running indefinitely.")
	} else {
		fmt.Println("Monitoring command:", strings.Join(args, " "))
		go cmd.RunCommand(args, quit)
	}
	etcd.RegisterService(*etcAddress, keyPath, formattedValue, time.Duration(*ttl)*time.Second, time.Duration(*updateInterval)*time.Second, quit)
}
