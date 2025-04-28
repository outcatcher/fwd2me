## Fwd2Me

Fwd2Me - really simple UPnP port forwarding for Linux

### Usage

```shell
$ fwd2me --help
Usage of fwd2me [options] [port1 port2 ...]:
  -label string
    	Label for the forwarding (default "fwd2me")
  -proto string
    	Forwarded port protocol (default "TCP")
```

### Example

```shell
$ fwd2me 80 443
Recreating forwarding from 46.164.xxx.xx to 192.168.1.76
Port 80 forwarded
Port 443 forwarded
```

