# mdnsmap

`mdnsmap` 是一个使用 Golang 实现的 mDNS 资产测绘 CLI。

它接收 `IP 网段` 和 `端口范围` 作为输入，对目标范围做主动 TCP 端口测绘，同时结合本地链路上的 mDNS / DNS-SD 发现结果，输出该范围内的 mDNS 资产信息，重点提取 `ip / port / host / banner`。

## 题目对应

本项目对应的能力点如下：

- 输入 `IP 网段`
- 输入 `端口范围`
- 输出该范围内的 mDNS 协议资产信息
- 至少包含 `ip / port / host / banner`
- 深度解析 `PTR / SRV / TXT / A / AAAA`
- 支持 `device-info` 这类无端口元数据服务
- 支持 `qdiscover` 这类复合 TXT banner 的深度拆解

## 实现思路

mDNS 本质上是链路本地多播发现协议，纯 mDNS 浏览器只能发现当前链路上的广播/组播资产，无法像 TCP 扫描器那样对任意远端网段逐个探测。

因此本项目采用了“主动测绘 + mDNS 深度识别”的混合方案：

1. 对用户输入的 `CIDR` 和端口范围做主动 TCP 扫描
2. 在当前本地链路上执行 mDNS / DNS-SD 服务发现
3. 解析 `PTR / SRV / TXT / A / AAAA`
4. 按 `ip / host / port` 关联扫描结果与 mDNS 资产
5. 输出结构化的 mDNS 资产信息

这种实现更贴近题目里“输入网段和端口范围进行测绘”的语义，同时保留了 mDNS banner 的真实深度。

## 当前支持的识别信息

- 服务类型：如 `http`、`smb`、`afpovertcp`、`qdiscover`、`workstation`
- 实例名称：如 `slw-nas`、`slw-nas(AFP)`
- 主机名：如 `slw-nas.local`
- IPv4 / IPv6
- 服务端口
- TTL
- TXT banner 字段
- PTR 服务类型列表
- TCP 主动验证结果

JSON 输出中会额外给出 `verified_open` 字段，用于表示该端口是否被主动扫描验证为开放。

## 构建

```bash
go build -o mdnsmap.exe ./cmd/mdnsmap
```

## 使用方式

```bash
mdnsmap.exe -cidr 192.168.1.0/24 -ports 1-10000
mdnsmap.exe -cidr 192.168.1.0/24 -ports 1-10000 -timeout 8s
mdnsmap.exe -cidr 192.168.1.0/24 -ports 1-10000 -json
```

## 参数说明

- `-cidr`：必填，目标网段，例如 `192.168.1.0/24`
- `-ports`：必填，目标端口范围，例如 `1-10000`
- `-timeout`：发现阶段超时与扫描拨号预算提示，默认 `5s`
- `-json`：以 JSON 格式输出结果

## 输出示例

文本输出风格尽量贴近题目示例：

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

JSON 输出示例结构：

```json
{
  "services": [
    {
      "name": "slw-nas",
      "service": "qdiscover",
      "service_type": "_qdiscover._tcp.local.",
      "protocol": "tcp",
      "hostname": "slw-nas.local.",
      "port": 5000,
      "ipv4": "192.168.1.20",
      "verified_open": true,
      "banner": {
        "accessType": "https",
        "accessPort": "86",
        "model": "TS-X64"
      }
    }
  ],
  "ptr_answers": [
    "_qdiscover._tcp.local."
  ]
}
```

## 测试与验证

项目已完成以下基础验证：

- `go test ./...`
- `go build ./...`
- 可执行文件构建成功

此外，项目中包含了合成 mDNS 记录测试，用于验证：

- `device-info` 元数据服务保留
- `qdiscover` 复合 TXT 字段拆解
- 文本输出顺序
- mDNS 资产与 TCP 扫描结果的关联逻辑

## 已知边界

- mDNS 发现结果仍然依赖当前可观测链路，远端不在同一广播域/多播域的资产可能无法获得完整 mDNS 广告信息
- 大网段配合大端口范围时，主动扫描耗时会增加
- TCP 扫描结果作为辅助验证证据，不代表 mDNS 记录本身的真假

## 项目结构

```text
cmd/mdnsmap/main.go
internal/cli/
internal/discovery/
internal/format/
internal/model/
internal/parser/
internal/sweep/
```
