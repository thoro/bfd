# bfd/bfdd

This repository consists of 3 parts.

1. The bfdd application, the main server application that can be controller via a gRPC Api
2. The bfd application, a cli tool to control bfdd
3. The library bfdd is based on, which can be reused in other applications.

## bfdd

The server application runs passively. It will not interact with any other application on the device it's running.
Therefore it's also the responsibility of a connector to handle state changes.

### Known Issues

- Only simple password authentication is implemented
- Updating of desiredMinTx (to something larger) or updating requiredMinRx (to something smaller) while the session is up leads to a panic (on purpose)
- Echo functionality is not implemented

### Config File Format

The config file needs to be placed at /etc/bfdd/config.yaml.
A different path can be passed via the -c / --config option of the binary.


The file format is yaml encoded, and consists of 2 main properties.

listen: Defines on which interfaces bfdd listens for incoming packets
peers: a map that defines which peers bfdd tries to contact with which settings (name, port, interval, detectioMultiplier)

name: a display name for the cli / api
port: the port to which bfd packets are sent
interval: the interval that packets are sent in ms
detectionMultiplier: after how many missed packets is the peer declared down (minimum detection interval = interval * detectionMultiplier)

Example:
```
listen:
- 0.0.0.0
- 192.168.1.1

peers:
  172.17.0.3:
    name: cogent
    port: 3784
    interval: 100
    detectionMultiplier: 5
```

## bfd

The client application is there to manage the running bfdd application.

The cli api is based on Cobra therefore the help options represent all available commands.

Possible commands:

| Command   | Description |
| --------- | ------------ |
| bfd peers | Lists all peers the bfd server has. |
| bfd peers -p 172.0.13.2 enable | Enabled the passed bfd peer |
| bfd peers -p 172.0.13.2 disable | Disable the passed bfd peer |
| bfd peers -p 172.0.13.2 | List information about a peer |
| bfd peers -p 172.0.13.2 set [DesiredMinTxInterval|RequiredMinRxInterval|DetectMultiplier] value | Sets a property on the peer |
| bfd peers add {name} {ip}172.0.13.3 {DesiredMinTxInterval}130 {RequiredMindRxInterval}40 {DetectMultiplier}2 [{IsMultiHop}Yes|No] [None|SimplePassword|KeyedMD5|MeticulousKeyedMD5|KeyedSHA1|MeticulousKeyedSHA1] {Password} | Adds a peer |
| bfd peers del {name/ip} | Deletes a peer |
| bfd monitor -p 172.0.13.2 | Monitors a peer for session state changes |


