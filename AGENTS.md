# 项目约定

- 默认使用中文文档与沟通，公开 Go API 使用简洁英文注释。
- 协议实现以 OpenEvenSdk 的 `PROTOCOL.md` 和已验证的 iOS 实现为准。
- 保持 BLE transport、protocol、G2 client、CLI 四层职责分离。
- 每个里程碑完成后运行 `go test ./...` 与 `go vet ./...`。
- 不在没有协议证据时新增字段、时序或兼容分支。
