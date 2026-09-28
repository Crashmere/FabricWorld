# FabricWorld

手机优先的共享布料库与缝纫成品集：记录布料、管理余料，保存作品照片、纸样和制作心得，也适配电脑。

- 公网经统一设备认证后，在同一网址共享读写。
- JPEG/PNG/WebP/HEIC 照片、封面排序、尺寸与多组余料、材质标签和搜索筛选。
- 成品集：完成日期、类别、纸样、尺码、标签与心得，多对多关联布料，双向查看作品和来源。
- 修改历史、30 天回收站、CSV 与带照片 ZIP 导出。
- Vue 3 + TypeScript + Go + SQLite + libvips；独立 systemd 服务与 Nginx 子路径。

源码：[Crashmere/FabricWorld](https://github.com/Crashmere/FabricWorld)（public）。

[文档入口](docs/README.md) · [运行与恢复](docs/OPERATIONS.md) · [验收与限制](docs/VERIFICATION.md)

日常发布与验证按 [本机发布说明](docs/DEPLOYMENT.md) 执行；GitHub 只作源码备份。

本地开发前运行 `npm --prefix web ci` 安装锁定依赖；正式发布的 `make release` 会在隔离快照中自动安装依赖并构建。
