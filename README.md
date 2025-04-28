# Fwd2Me

Fwd2Me - really simple UPnP port forwarding

## Usage

```shell
$ fwd2me --help
Usage of fwd2me [options] port1[:remote[:proto]] port2 ...:
  -label string
        Label for the forwarding (default "fwd2me")
  -proto string
        Default forwarded port protocol (default "TCP")
```

## Example

### Symmetrical, TCP

```shell
$ fwd2me 80 443
Recreating forwarding from 46.164.xxx.xx to 192.168.1.76
Port forwarding created: internal (80), external (80), proto (TCP)
Port forwarding created: internal (443), external (443), proto (TCP)
```

### Symmetrical, UDP

```shell
$ fwd2me -proto UDP 62332
Recreating forwarding from 46.164.xxx.xx to 192.168.1.76
Port forwarding created: internal (62332), external (62332), proto (UDP)
```

### Assymetrical, TCP

```shell
$ fwd2me 16080:80
Recreating forwarding from 46.164.xxx.xx to 192.168.1.76
Port forwarding created: internal (16080), external (80), proto (TCP)
```


### Assymetrical, UDP

```shell
$ fwd2me 65101:51101:UDP
Recreating forwarding from 46.164.xxx.xx to 192.168.1.76
Port forwarding created: internal (65101), external (51101), proto (UDP)
```
