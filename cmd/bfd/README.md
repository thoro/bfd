# BFD Cli Options

-s 127.0.0.1:54211


## bfd peers

Lists all peers the bfd server has.

```
Name    IP
```

## bfd peers -p 172.0.13.2 enable

Enables the peer.

## bfd peers -p 172.0.13.2 disable

Disables the peer. (State is set to Admin Down)

## bfd peers -p 172.0.13.2

```
DesiredMinTxInterval: 140 ms
RequiredMinRxInterval: 50 ms
DetectMultiplier: 3
IsMultiHop: Yes/No
Authentication: SimplePassword (MyPassword123)

SessionState: Up
```

Lists details on one peer.

## bfd peers -p 172.0.13.2 set [DesiredMinTxInterval|RequiredMinRxInterval|DetectMultiplier] value

Updates the specified property to the passed value

## bfd peers add {name} {ip}172.0.13.3 {DesiredMinTxInterval}130 {RequiredMindRxInterval}40 {DetectMultiplier}2 [{IsMultiHop}Yes|No] [None|SimplePassword|KeyedMD5|MeticulousKeyedMD5|KeyedSHA1|MeticulousKeyedSHA1] {Password}

Created a new temporary bfd peer.

## bfd peers del {name/ip}

Deletes a bfd peer

## bfd monitor -p 172.0.13.2