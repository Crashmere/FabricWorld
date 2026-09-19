# FabricWorld 维护入口

先读 [docs/README.md](docs/README.md)，按需读取架构、API、运维和 CI/CD 文档。

- 用户确认共享布料库，无登录和身份认证；持有网址的人可以查看、编辑、删除和导出。首版联网使用，服务器为正式数据源。
- Go + Vue + SQLite + libvips。照片与数据库一起备份；不要复制活跃 WAL 主文件，也不要原地覆盖已保存照片。
- 尺寸统一整数毫米、金额整数分；未知尺寸不能写成 0。余料状态、多组尺寸、图片引用与 revision 必须一致。
- 写入必须支持幂等和版本冲突。测试使用隔离合成数据，生产默认只读验收。
- 修改后运行 Go 测试、go vet、前端类型检查/构建；界面实测窄屏（375×667 和 320 px）与桌面。真实手机相机未验证时必须如实记录。
- 部署先使用 server-operations，显式读取 ssh ali 'cat /opt/AGENTS.md'。本项目仅维护自己的 location、unit、数据和发布身份。
- 更新对应 docs/deploy，新增共享状态同步到 agent-config；服务器文档来自已提交的白名单文件。
- GitHub 仓库 public。生产地址、照片、数据库、备份、凭据不能入 Git。推送 main 运行 CI，生产发布为单独手动工作流。
