package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JeremyOT/address/lookup"
	"github.com/JeremyOT/etcdmon/cmd"
	"github.com/JeremyOT/etcdmon/etcd"
)

var etcdAddress = flag.String("etcd", "http://127.0.0.1:4001", "The url of the etcd instance to connect to.")
var etcdKey = flag.String("key", "", "The key path to post to. %H will be replaced with the current host and %P with any configured port. e.g. process/%H-%P")
var updateInterval = flag.Duration("interval", 10*time.Second, "The number of seconds between each poll to etcd.")
var ttl = flag.Duration("ttl", 30*time.Second, "The number of seconds that the key should stay alive after no polls are received.")
var host = flag.String("host", "", "The hostname or address that should be used to identify this daemon. If not set, etcdmon will attempt to determine it automatically.")
var port = flag.Int("port", 0, "A port, if any that will be set along with the host name.")
var value = flag.String("value", "", "The the value to sent to etcd. Like path, %H and %P can be used for automatic replacement. If not set, a json object with the host (and port if set) will be used as the value.")
var apiRoot = flag.String("api", "/v2/keys", "The root value for any key path.")
var remoteTestAddress = flag.String("remote", "", "The address to use to infer the address of the local host.")
var interfaceName = flag.String("interface", "", "The interface to use to infer the address of the local host.")
var listRegistered = flag.Bool("list", false, "List all currently registered services matching the supplied parameters and exit.")
var listRegisteredExpiration = flag.Bool("expiry", false, "List expiration time as well.")

func formatValue(value, host string, port int) string {
	if value == "" {
		if port > 0 {
			return fmt.Sprintf(`{"host": "%s", "port": %d}`, host, port)
		} else {
			return fmt.Sprintf(`{"host": "%s"}`, host)
		}
	}
	return strings.Replace(strings.Replace(value, "%H", host, -1), "%P", strconv.Itoa(port), -1)
}

func formatKey(key, host string, port int) string {
	return strings.Replace(strings.Replace(key, "%H", host, -1), "%P", strconv.Itoa(port), -1)
}

func monitorSignal(sigChan chan os.Signal, registry *etcd.Registry, command *cmd.Command) {
	for sig := range sigChan {
		switch sig {
		case syscall.SIGQUIT:
			if command != nil {
				log.Println("Received signal:", sig, "exiting after grace period")
				registry.SafeStop()
				log.Println("Killing process")
				if err := command.Kill(); err != nil {
					log.Println("Failed to kill process:", err)
					os.Exit(1)
				}
			} else {
				log.Println("Received signal:", sig, "exiting immediately")
				registry.Stop()
			}
		default:
			log.Println("Received signal:", sig, "exiting immediately")
			registry.Stop()
			if command != nil {
				if err := command.Signal(sig); err != nil {
					log.Println("Failed to send signal:", sig, err)
					if err = command.Kill(); err != nil {
						log.Println("Failed to kill process:", err)
						os.Exit(1)
					}
				}
			}
		}
	}
}

func main() {
	flag.Usage = func() {
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
	flag.Parse()
	if *etcdAddress == "" || *etcdKey == "" {
		flag.Usage()
	}
	log.SetOutput(os.Stdout)
	if *listRegistered {
		keyPath := path.Join(*apiRoot, *etcdKey)
		services, err := etcd.ListServices(*etcdAddress, keyPath)
		if err != nil {
			panic(err)
		}
		for _, node := range services {
			if *listRegisteredExpiration {
				if node.Expiration.IsZero() {
					fmt.Println(node.Value, "Expires: Never")
				} else {
					fmt.Println(node.Value, "Expires:", node.Expiration.Sub(time.Now()))
				}
			} else {
				fmt.Println(node.Value)
			}
		}
		return
	}
	remote := *remoteTestAddress
	if remote == "" {
		remote = *etcdAddress
	}
	args := flag.Args()
	serviceHost := *host
	if serviceHost == "" {
		if *interfaceName != "" {
			ip, err := lookup.InterfaceIP(*interfaceName)
			if err != nil {
				log.Println("Error retrieving address. Try using a different -interface='' or setting it manually with -host=''.", err)
				os.Exit(1)
			}
			serviceHost = ip.String()
		} else {
			var err error
			serviceHost, err = lookup.LocalAddress(remote)
			if err != nil {
				log.Println("Error retrieving address. Try using a different -remote='' or setting it manually with -host=''.", err)
				os.Exit(1)
			}
		}
	}
	keyPath := formatKey(path.Join(*apiRoot, *etcdKey), serviceHost, *port)
	formattedValue := formatValue(*value, serviceHost, *port)
	fmt.Println("Etcd:", *etcdAddress, "Key:", keyPath, "Value:", formattedValue, "TTL:", *ttl, "UpdateInterval:", *updateInterval)
	registry := etcd.NewRegistry(*etcdAddress, keyPath, formattedValue, *ttl, *updateInterval)

	sigChan := make(chan os.Signal)
	if len(args) == 0 {
		fmt.Println("No command specified. Running indefinitely.")
		registry.Start()
		go monitorSignal(sigChan, registry, nil)
		signal.Notify(sigChan, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		registry.Wait()
	} else {
		command := cmd.New(args)
		fmt.Println("Monitoring command:", strings.Join(args, " "))
		command.Start()
		registry.Start()
		go monitorSignal(sigChan, registry, command)
		signal.Notify(sigChan, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		command.Wait()
	}
}
