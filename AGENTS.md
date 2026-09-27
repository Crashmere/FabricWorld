# FabricWorld 维护入口

先读 [docs/README.md](docs/README.md)，按需读取架构、API、运维和 CI/CD 文档。

- 共享布料与成品库接入统一设备认证；授权设备可以查看、编辑、删除和导出。首版联网使用，服务器为正式数据源。成品只关联布料来源，余料在布料详情手动更新。
- Go + Vue + SQLite + libvips。照片与数据库一起备份；不要复制活跃 WAL 主文件，也不要原地覆盖已保存照片。
- 尺寸统一整数毫米、金额整数分；未知尺寸不能写成 0。余料状态、多组尺寸、图片引用与 revision 必须一致。
- 简单下拉选项与输入建议使用统一自绘组件；日期、时间等复杂控件保留系统选择器。布料库/成品集只在顶栏切换，不在更多菜单重复入口。交互与动画约定见 docs/CONTROLS.md。
- 写入必须支持幂等和版本冲突。测试使用隔离合成数据，生产默认只读验收。
- 修改后运行 Go 测试、go vet、前端类型检查/构建；界面实测窄屏（375×667 和 320 px）与桌面。真实手机相机未验证时必须如实记录。
- 部署先使用 server-operations，显式读取 ssh ali 'cat /opt/AGENTS.md'。本项目仅维护自己的 location、unit、数据和发布身份。
- 更新对应 docs/deploy，新增共享状态同步到 agent-config；推送后运行 agent-config 的 `skills/server-operations/scripts/sync-docs.sh FabricWorld` 同步服务器副本。
- 哪些事直接做完再告知、哪些先确认，只看 server-operations SKILL.md 的授权表；文档维护与同步不需要事先确认。
- GitHub 仓库 public。生产地址、照片、数据库、备份、凭据不能入 Git。推送 main 运行 CI and deploy，检查通过后自动发布到生产；纯文档提交加 `[skip ci]`。

## 门户资源同步

- 本项目的 `deploy/portal.json` 是 ServerPortal 资源声明的维护源，记录目录用途、数据库、运行用户、端口、unit、访问路径、API 与备份类型。新增/迁移/删除数据根、接口或运行材料时，必须同步修改声明、对应 docs 与共享应用清单。
- 声明部署在 `/opt/fabricworld/config/portal.json`，root 管理；CI 共用 server-operations 校验器，发布前预检，发布后通过受限 SSH 自动同步并核对门户加载哈希。`registry.d/fabricworld.json` 自动登记链接，更新无需重启门户。只修改门户信息时可手动运行 CI 的 `portal_only=true`，保留业务运行版本。
- 统一认证由共享 Nginx 与门户负责，不在本项目另存设备白名单；本机调用和发布健康检查按共享约定保留。生产已启用设备认证；变更后同步 server-operations current-state。
- 门户只读展示不替代本项目原生一致性备份；备份格式或媒体生命周期变化必须同时验证门户全量/增量与离线恢复。真实业务数据、凭据和备份仍不得进入 Git。
