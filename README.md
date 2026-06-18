# mdnsmap

`mdnsmap` is a Go CLI for hybrid asset mapping with mDNS enrichment.

It accepts an IP range and a port range, actively scans that scope, performs
mDNS / DNS-SD discovery on the local link, and correlates the results into one
asset view.

## What it does

- accepts `-cidr` and `-ports`
- actively scans TCP ports inside the requested scope
- discovers mDNS / DNS-SD service records from the local link
- resolves `PTR`, `SRV`, `TXT`, `A`, and `AAAA`
- correlates scan hits with mDNS assets by `ip`, `host`, and `port`
- keeps metadata-only services like `device-info` when they belong to a matched host
- outputs text or JSON

## Why it is hybrid

mDNS is a link-local multicast protocol. A pure mDNS browser can discover
services advertised on the current local link, but it cannot actively probe an
arbitrary remote subnet the same way a TCP scanner can.

This project therefore uses a hybrid model:

1. actively scan the user-provided `CIDR` and port range
2. browse mDNS services on the current local link
3. keep mDNS assets that belong to the requested `CIDR`
4. correlate scan results back as verification evidence instead of hard deletion

That keeps the CLI behavior closer to the task wording while still grounding
banner depth in real mDNS data.

## Build

```bash
go build -o mdnsmap.exe ./cmd/mdnsmap
```

## Usage

```bash
mdnsmap.exe -cidr 192.168.1.0/24 -ports 1-10000
mdnsmap.exe -cidr 192.168.1.0/24 -ports 1-10000 -timeout 8s
mdnsmap.exe -cidr 192.168.1.0/24 -ports 1-10000 -json
```

## Flags

- `-cidr`: required target network, for example `192.168.1.0/24`
- `-ports`: required TCP port range, for example `1-10000`
- `-timeout`: mDNS phase timeout and scanner dial budget hint, default `5s`
- `-json`: output JSON instead of text

## Output shape

The text output is intentionally close to the task sample:

```text
services:
device-info:
Name=slw-nas(AFP)
IPv4=192.168.1.20
Hostname=slw-nas.local
TTL=10
model=Xserve
5000/tcp qdiscover:
Name=slw-nas
IPv4=192.168.1.20
Hostname=slw-nas.local
TTL=10
accessType=https
accessPort=86
model=TS-X64
displayModel=TS-464C
fwVer=5.2.9
fwBuildNum=20260214
answers:
PTR:
_device-info._tcp.local
_qdiscover._tcp.local
```
