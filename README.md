# FabricWorld

手机优先的共享布料库：拍照记录购买的布料，管理材质、尺寸、收纳位置和使用后的余料，也适配电脑。

- 无登录与身份认证，同一网址共享读写。
- JPEG/PNG/WebP/HEIC 照片、封面排序、尺寸与多组余料、材质标签和搜索筛选。
- 修改历史、30 天回收站、CSV 与带照片 ZIP 导出。
- Vue 3 + TypeScript + Go + SQLite + libvips；独立 systemd 服务与 Nginx 子路径。
- 数据库与照片一致性备份，GitHub Actions 检查，推送 main 后自动发布。

源码：[Crashmere/FabricWorld](https://github.com/Crashmere/FabricWorld)（public）。

[文档入口](docs/README.md) · [运行与恢复](docs/OPERATIONS.md) · [验收与限制](docs/VERIFICATION.md)
