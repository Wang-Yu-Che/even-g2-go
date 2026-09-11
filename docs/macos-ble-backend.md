# macOS BLE backend 调查

调查版本：tinygo.org/x/bluetooth v0.16.0。

Darwin 实现位于该模块的 adapter_darwin.go、gap_darwin.go 和
gattc_darwin.go，底层使用 tinygo-org/cbgo 调用 CoreBluetooth。

第一阶段所需能力中，公开 API 已覆盖：

- Adapter.Enable
- 无 service filter 的 Adapter.Scan 与 Adapter.StopScan
- 从 ScanResult 读取 LocalName、Address 与 RSSI
- 通过扫描所得 Address 连接 peripheral
- service / characteristic discovery
- characteristic write 与 write without response
- EnableNotifications

已知限制：该库没有公开 CoreBluetooth
retrieveConnectedPeripherals(withServices:) 的等价入口。因此，已经被系统或其他应用持有、
且不再出现在扫描回调中的镜腿，当前纯 Go backend 无法像 OpenEvenSdk iOS 实现那样主动接管。
M3 应先验证真实 macOS 环境中两臂是否能正常重新广播和连接；如果该限制实际阻断连接，再评估
向上游补充 API 或最小 CoreBluetooth bridge，不提前引入自有 CGO 层。

另一个 API 差异是 Darwin 的 DeviceCharacteristic 不公开 characteristic properties。
因此当前实现会枚举全部 GATT 项，但只选择协议明确指定的 5401 写通道和 5402 notify 通道；
无法安全复刻 iOS 实现的“第一个 writable characteristic”回退和“订阅所有 notify/indicate”策略。
