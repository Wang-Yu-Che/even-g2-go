# MentraOS G2 协议覆盖

本项目以 MentraOS `G2.kt` 中有明确 wire-format 或设备验证说明的实现为依据。

| 服务 | ID | Go 接口 | 状态 |
|---|---:|---|---|
| Dashboard | `0x01` | `ConfigureDashboard`、`PushDashboardSchedule` | 已接入 |
| Menu | `0x03` | `SetMenu`、`EvenHubEventMenu` | 已接入 |
| Notification | `0x04` | `ConfigureNotifications`、`SubscribeNotificationResponses` | 已接入 |
| EvenAI | `0x07` | `SetHeyEven`、`ControlEvenAI`、`AskEvenAI`、`TriggerEvenAISkill` | 已接入 |
| Navigation | `0x08` | `StartCompass`、`StopCompass`、`SubscribeNavigation` | 已接入 |
| G2 Settings | `0x09` | 设置查询、亮度、抬头角度、显示位置 | 已接入 |
| Gesture Control | `0x0D` | `InitializeGestureControl` | 已接入初始化 |
| Onboarding | `0x10` | `SkipOnboarding` | 已接入 |
| Device Settings | `0x80` | 认证、心跳、`SyncTime`、`SetRingConnection` | 已接入 |
| EvenHub Control | `0x81` | 启动 prelude | 已接入 |
| File Command/Data | `0xC4` / `0xC5` | `TransferFile` | 已接入 |
| EvenHub | `0xE0` | 页面、文本、图像、音频、IMU、事件 | 已接入 |

原生通知内容通过右镜腿独立 File GATT characteristic 传输，不走普通控制
characteristic。文件协议按 START、DATA、RESULT_CHECK 三阶段串行执行；任何超时或
取消都会将通道标记为必须重连，避免迟到 ACK 被下一次传输误收。

未实现的 `G2.kt` 空方法（Wi-Fi、相机、视频、RGB LED）不是遗漏协议：G2 本身没有
对应硬件，MentraOS 中也没有可移植的 G2 wire-format。通知删除同样没有已验证操作码。
