# even-g2-go 实现路线

目标是在 macOS 上不依赖 Even 官方 App，通过 BLE 覆盖 OpenEvenSdk 已公开的 G2 能力。

- M1–M6（完成）：协议基础、扫描、双臂连接、认证、心跳、teleprompter `displayText`。
- M7（完成）：EvenHub 多分片帧、protobuf 解码、ACK 与输入事件解析。
- M8（完成）：右臂通知路由、magic 请求关联、EvenHub 心跳和页面生命周期。
- M9（完成并经 G2 真机确认）：全屏 `displayList`、`showText`、`updateText`，点击、翻页、双击回调。
- M10（完成）：4-bpp BMP、576×288 图像处理和 2×2 切片。
- M11（完成并经 G2 真机确认）：`displayImage`、图片容器 CREATE、warmup、3800 字节数据片、4 ACK 滑动窗口、块间心跳和 Session 跳跃重试。
- M12（完成）：统一高层 `Connect`、状态、显式/自动重连和公开接口整理。
- M13（完成并经 G2 真机确认）：本地 HTTP bridge：`/status`、`/text`、`/image`。
- M14（完成并经 G2 真机确认）：麦克风控制、独立 `6402` 订阅、LC3 原始包接收、5×40 字节拆帧和包计数检测。
- M15（进行中）：G2 真机回归、具名事件 API、示例、文档和版本发布准备；macOS arm64 已支持内嵌 `liblc3` 解码和 WAV 输出。

每个里程碑先用 OpenEvenSdk 的 `PROTOCOL.md` 和平台实现建立黄金测试，再接入设备层；未经真机确认的行为必须明确标为实验性。
