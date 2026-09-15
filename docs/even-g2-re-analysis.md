# Even-G2-RE 可吸收结论

分析对象：[lonelyobserver0/Even-G2-RE](https://github.com/lonelyobserver0/Even-G2-RE)，
提交 `4420ee6597b915de466b60da19ac665c8d1ef2a4`（2026-03-13）。

## 已吸收

- `AA 12` 单分片应用包使用 8 字节头、payload CRC-16/CCITT-FALSE 和 little-endian service ID。
- 已在抓包中出现的 service ID：default `0x0001`、dashboard `0x0008`、configuration
  `0x000C`、translation `0x000E`、system `0x0080`、sync info `0x0108`、device info
  `0x0109`、device settings `0x010D`、navigation `0x0180`。
- CLI `decode-packet` 可离线校验并解码 `AA 12` 和现有 `AA 21` 单分片包。

实现位于 `protocol/application.go`。这些能力只负责 transport envelope，不推断内部
protobuf schema。

## 暂不吸收

- Dashboard `cmdId=7/8/9` 构造：字段编号、页面关联和图片包装仍是实验性假设。
- Even RLE：仓库只确认相关符号存在，未给出经过样本验证的编码算法。
- OTA、文件服务和 Ring relay：目前主要是类名、命令名和流程描述，缺少可验证 payload。
- `6401/6402`、`7401/7402` 通用通道 API：`psType` 路由已由 Android DEX 确认，但不同
  功能与通道的稳定映射尚不完整；当前继续仅按已验证用途使用 `5401/5402` 和麦克风
  `6402`。

后续实现上述能力前，需要官方 App outbound bytes、完整 ATT write 抓包或真机黄金样本。
