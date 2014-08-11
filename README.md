etcdmon
=======
A tool for tracking a processes life in etcd.

[![Build Status](https://drone.io/github.com/JeremyOT/etcdmon/status.png)](https://drone.io/github.com/JeremyOT/etcdmon/latest)


Usage
-----

```bash
Usage:
  Updates -key in etcd periodically until either exited or a monitored process exits.
  To monitor an external command add the command argurments after all etcdmon options.
  E.g. etcdmon -key process -- my_script.sh arg1 arg2
  stdin, stdout and stderr will be piped through from the monitored command.

Options:
  -api="/v2/keys": The root value for any key path.
  -etcd="http://127.0.0.1:4001": The url of the etcd instance to connect to.
  -host="": The hostname or address that should be used to identify this daemon. If not set, etcdmon will attempt to determine it automatically.
  -interface="": The interface to use to infer the address of the local host.
  -interval=10ns: The number of seconds between each poll to etcd.
  -key="": The key path to post to. %H will be replaced with the current host and %P with any configured port. e.g. process/%H:%P
  -port=0: A port, if any that will be set along with the host name.
  -remote="": The address to use to infer the address of the local host.
  -ttl=30ns: The number of seconds that the key should stay alive after no polls are received.
  -value="": The the value to sent to etcd. Like path, %H and %P can be used for automatic replacement. If not set, the host (and port if set) will be used as the value.
```

#### Additional use cases

```bash
etcdmon -key key/path # send heartbeats to http://127.0.0.1:4001/v2/keys/key/path until etcdmon is terminated
etcdmon -remote github.com -getaddress # print the local network address as resolved by connecting to a remote address
```
