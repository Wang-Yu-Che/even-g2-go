# OpenEvenSdk 协议分析

本文是 even-g2-go 第一阶段的实现依据。分析对象为
[Thepizzapie/OpenEvenSdk](https://github.com/Thepizzapie/OpenEvenSdk)，提交
fa0522ea283744eaf345a788a9fee82a740afd08（2026-06-14）。

优先级：PROTOCOL.md 是 wire protocol 的最高事实来源；iOS 实现已经在真实
G2 硬件验证，用于确认调用顺序与时序；Android 仅作交叉检查。

## 1. OpenEvenSdk 目录结构

    OpenEvenSdk/
    ├── PROTOCOL.md                 协议的单一事实来源
    ├── ios/                        已在真实硬件验证的 Swift/CoreBluetooth 实现
    │   └── Sources/G2Bridge/
    │       ├── CRC16.swift          CRC-16/CCITT-FALSE
    │       ├── G2Protocol.swift     framing、认证、teleprompter
    │       └── G2Connection.swift   双臂连接、GATT、notify、心跳、发送时序
    ├── android/                    Kotlin 移植，尚未完整硬件验证
    └── claude-glass-bridge/         不属于本阶段的 Python 上层桥接

even-g2-go 第一阶段只复刻 CRC16.swift、G2Protocol.swift 和
G2Connection.swift 中的双臂 teleprompter 路径；不实现 EvenHub、图片、音频或 HTTP。

## 2. BLE 连接流程

G2 是两个独立 BLE peripheral，左、右镜腿都必须连接。

1. 扫描名称包含 Even 的设备。
2. 名称含 _L_ 或 LEFT 归为左臂；含 _R_ 或 RIGHT 归为右臂；无法识别时填充尚空的槽位。
3. 左右各自连接，发现全部 service 与 characteristic。
4. 对每个具备 notify 或 indicate 属性的 characteristic 订阅；控制 notify UUID 优先作为控制通知通道。
5. 选择控制写 characteristic：优先 UUID 后缀 2E5401，否则第一个可写 characteristic。
6. 两臂都具备写通道后，依次向两臂发送七个认证包；每包间隔约 100 ms，结束后等待约 500 ms。
7. 状态进入 Ready 后启动文本模式心跳。

每个文本/teleprompter 包都必须先写左臂、等待约 18 ms、再写右臂、等待约 12 ms。仅向一侧发送无法渲染文本。

来源：PROTOCOL.md §§1、2、5、8；G2Connection.swift 的 slot(forName:)、
didDiscoverServices、didDiscoverCharacteristicsFor、authenticate、sendBoth。

## 3. GATT UUID

| 用途 | UUID | 第一阶段用途 |
|---|---|---|
| Service | 6E400001-B5A3-F393-E0A9-E50E24DCCA9E | 过滤/识别 G2 service |
| 控制写 | 00002760-08C2-11E1-9073-0E8AC72E5401 | 认证、心跳和 teleprompter 写入；writeWithoutResponse |
| 控制通知 | 00002760-08C2-11E1-9073-0E8AC72E5402 | ACK 与事件日志；notify |
| 二进制写 | 00002760-08C2-11E1-9073-0E8AC72E6401 | 可选图片通道，本阶段不使用 |
| LC3 通知 | 00002760-08C2-11E1-9073-0E8AC72E6402 | 麦克风，本阶段不使用 |

来源：PROTOCOL.md §2；G2Protocol.swift 的 UUID 常量。

## 4. 单包 packet envelope

    AA 21 SEQ LEN 01 01 SVC_HI SVC_LO PAYLOAD... CRC_LO CRC_HI

- LEN = (payload 长度 + 2) & 0xff。
- 01 01 是固定单片标记。
- service ID 是两个原样写入的字节，不是小端整数编码。
- CRC 只覆盖 payload，不覆盖 header / service ID，且以小端追加。
- 多片 framePb 属于 EvenHub 图片路径；第一阶段不需要实现。

来源：PROTOCOL.md §4a；G2Protocol.swift 的 buildPacket。

## 5. CRC

    poly       = 0x1021
    initial    = 0xffff
    MSB-first  = true
    final xor  = 0x0000
    wire order = little-endian（低字节在前）

计算范围严格是单包 payload。不能替换为 CRC-16/X25、IBM 或覆盖整个 packet 的变体。

来源：PROTOCOL.md §3；ios/Sources/G2Bridge/CRC16.swift。

## 6. Sequence 规则

参考实现没有采用单个连接全局递增的 sequence。它按逻辑流程维护，且同一包镜像到左右臂时使用相同 sequence：

| 流程 | sequence 行为 |
|---|---|
| 认证 | 固定 0x01 到 0x07 |
| 一次 teleprompter 发送 | 从 0x08 开始，每一个 config/page/marker/sync 包递增；每次完整渲染重新从 0x08 开始 |
| 文本模式心跳 | 独立从 0xC0 开始递增；溢出到 0x00 时重置为 0xC0 |
| EvenHub | 独立 ehSeq，不属于第一阶段 |

Go 实现不能把认证、teleprompter、心跳强行合并成一个共享单调计数器。应为这些逻辑流分别维护并发安全的状态；发送 teleprompter 时还应暂停心跳，避免流交叉。

来源：PROTOCOL.md §§4、5、7、8；G2Connection.swift 的 sendTeleprompter、hbSeq、sendHeartbeat。

## 7. 七包认证

连接和 GATT 初始化完成后，取同一个 Unix 秒级时间戳并 protobuf base-128 varint 编码。固定 txid：

    E8 FF FF FF FF FF FF FF FF 01

| # | seq | service | payload |
|---:|---:|---|---|
| 1 | 01 | 80 00 | 08 04 10 0C 1A 04 08 01 10 04 |
| 2 | 02 | 80 20 | 08 05 10 0E 22 02 08 02 |
| 3 | 03 | 80 20 | 08 80 01 10 0F 82 08 11 08 + ts + 10 + txid |
| 4 | 04 | 80 00 | 08 04 10 10 1A 04 08 01 10 04 |
| 5 | 05 | 80 00 | 08 04 10 11 1A 04 08 01 10 04 |
| 6 | 06 | 80 20 | 08 05 10 12 22 02 08 01 |
| 7 | 07 | 80 20 | 08 80 01 10 13 82 08 11 08 + ts + 10 + txid |

将每个 payload 按第 4 节封装，并依次发送到左、右臂。每个认证包完成双臂发送后等待约 100 ms；第七包后等待约 500 ms。iOS 实现不等待或解析认证成功 ACK，而是按上述固定时序转为 Ready。

来源：PROTOCOL.md §§5、6；G2Protocol.swift 的 authPackets；G2Connection.swift 的 authenticate。

## 8. Heartbeat

G2 BLE supervision timeout 约 2 秒。teleprompter/text 模式每约 1.5 秒向左右臂各发送：

    buildPacket(seq, 80, 00, 08 25)

心跳生命周期必须绑定连接 context：断开、认证失败或显式关闭时立即取消。teleprompter 整套发送期间暂停心跳，发送完成再恢复，避免与显示流交错。

来源：PROTOCOL.md §7；G2Protocol.swift 的 heartbeat；G2Connection.swift 的 startHeartbeat、sendHeartbeat。

## 9. Teleprompter 完整发送顺序

格式化规则：按空格折行，最多 25 字符/行；每页 10 行；每页行以换行连接且末尾加一个空格和换行；不足 14 页补充全空白页。初始化 totalLines 使用输入文本将字面量 \\n 替换为换行后的原始行数，不是补齐后的行数。

一次发送从 seq=0x08、msgId=0x14 开始。每个 packet 后 sequence 与 message ID 均加一：

1. displayConfig，service 0E 20，等待 150 ms。
2. teleprompterInit，service 06 20，等待 300 ms。
3. 发送 page 0–9，service 06 20，每包等待 45 ms。
4. 发送 marker，service 06 20，等待 45 ms。
5. 发送 page 10–11，service 06 20，每包等待 100 ms。
6. 发送 sync，service 80 00，等待 45 ms；此包激活渲染。
7. 发送 page 12 及之后，service 06 20，每包等待 100 ms。

每一项均按左臂写入、18 ms、右臂写入、12 ms镜像。视图约 18 秒自行超时；参考实现每约 11 秒重发完整 flow。CONFIG_BLOB、init、page、marker、sync 的精确 payload 公式以 PROTOCOL.md §8 为准，Go 代码必须逐字节复刻 G2Protocol.swift。

来源：PROTOCOL.md §8；G2Protocol.swift 的 displayConfig、teleprompterInit、contentPage、marker、sync、formatText；G2Connection.swift 的 sendTeleprompter、startDisplayRefresh。


## 10. iOS 对应源码位置

| 主题 | 文件 / 符号 |
|---|---|
| GATT 常量、varint、封包 | ios/Sources/G2Bridge/G2Protocol.swift：serviceUUID、writeUUID、notifyUUID、encodeVarint、buildPacket |
| CRC | ios/Sources/G2Bridge/CRC16.swift：CRC16.ccitt |
| 认证和 teleprompter payload | G2Protocol.swift：authPackets 至 inputLineCount |
| 扫描、左右分类、已连接设备接管 | G2Connection.swift：beginScan、slot(forName:)、adoptConnected、didDiscover |
| GATT 枚举、订阅与写通道选择 | G2Connection.swift：didDiscoverServices、didDiscoverCharacteristicsFor |
| notify 日志 | G2Connection.swift：didUpdateValueFor |
| 双臂节奏、认证、心跳 | G2Connection.swift：sendBoth、authenticate、startHeartbeat、sendHeartbeat |
| 文本发送与自动刷新 | G2Connection.swift：displayText、startDisplayRefresh、sendTeleprompter |

## 11. 当前未知点与边界

1. **macOS Go BLE backend 可行性尚未验证。** 本阶段尚未选择或实现 tinygo.org/x/bluetooth 后端；需要在 M2 前以最小 macOS 实机程序验证扫描、连接、完整 GATT 发现、writeWithoutResponse 与 notify。不能假设它具备 CoreBluetooth 的已连接 peripheral 接管或 TX-ready 回调。
2. **第一阶段无需依赖认证 ACK。** OpenEvenSdk 只记录通知，在第七认证包后固定等待约 500 ms 即转 Ready；协议没有给出必须匹配的 ACK schema。因此 Go 首版应打印 notify，但不编造 AuthAck 的解析或成功条件。
3. **未文档化的普通控制通知格式。** PROTOCOL.md 描述了 EvenHub（sid 0xE0）ACK/事件，但没有规定 teleprompter 控制通知的完整解析。首版应原样暴露/记录字节，不应猜测 packet body。
4. **名称兜底分类存在歧义。** 当广播名不含左右标记时，参考实现按发现顺序填空槽；这不能保证物理左右对应。应照搬并在日志中显示原始名称。
5. **字符计数语义。** Swift 使用 String.count 进行 25 列折行，而 payload 使用 UTF-8 字节长度。中文、emoji 的真实显示列宽没有协议保证；第一阶段应保持该参考逻辑，并实机验证。

## 实现约束摘要

- 不修改 header、CRC 范围、大小端、认证内容或时序。
- 认证固定序列和 teleprompter/heartbeat 独立序列按上述规则实现。
- 所有文本控制包均写入两臂；EvenHub 不在第一阶段范围。
- 协议层必须独立于 BLE transport，并以 golden tests 验证 Go 输出与本文所列参考实现逐字节一致。
