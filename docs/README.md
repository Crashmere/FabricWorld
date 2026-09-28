# 项目文档

| 主题 | 文档 |
| --- | --- |
| 架构、数据与图片生命周期 | [ARCHITECTURE.md](ARCHITECTURE.md) |
| 缝纫成品模块设计与上线步骤 | [WORKS.md](WORKS.md) |
| 自绘选择控件与顶栏导航 | [CONTROLS.md](CONTROLS.md) |
| HTTP 接口与错误 | [API.md](API.md) |
| 安装、备份、恢复与诊断 | [OPERATIONS.md](OPERATIONS.md) |
| 本机发布与回退 | [DEPLOYMENT.md](DEPLOYMENT.md) |
| 验证证据与未验证项 | [VERIFICATION.md](VERIFICATION.md) |
| 已审核设计基线 | [TECHNICAL_PLAN.md](TECHNICAL_PLAN.md) |

共享服务器维护入口为 server-operations，服务器副本 /opt/server-context。共享布料与成品库已接入统一设备认证，授权设备可查看、编辑、删除和导出。

日常发布与验证按 [本机发布说明](DEPLOYMENT.md) 执行；GitHub 只作源码备份。
