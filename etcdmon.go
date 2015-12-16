package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
var listRegistered = flag.Bool("list", false, "List all currently registered services matching the supplied parameters and exit.")

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
	registryConfig := etcd.Config{
		APIRoot:        *apiRoot,
		EtcdHost:       *etcdAddress,
		Key:            *etcdKey,
		Value:          *value,
		Host:           *host,
		UpdateInterval: *updateInterval,
		TTL:            *ttl,
	}
	registryConfig, err := registryConfig.Populate()
	if err != nil {
		panic(err)
	}
	if *listRegistered {
		// keyPath := path.Join(*apiRoot, *etcdKey)
		services, err := etcd.ListServices(registryConfig)
		if err != nil {
			panic(err)
		}
		for _, node := range services {
			fmt.Println(node)
		}
		return
	}
	args := flag.Args()

	fmt.Println("Etcd:", registryConfig.EtcdHost, "Key:", registryConfig.KeyPath, "Value:", registryConfig.Value, "TTL:", registryConfig.TTL, "UpdateInterval:", registryConfig.UpdateInterval)
	registry := etcd.NewRegistry(registryConfig)

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
