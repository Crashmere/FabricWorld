# 安装、运行与恢复

公网入口使用可信 IP 证书的 HTTPS，原有 /fabricworld/ 路径保持。公网 HTTP 返回 308；API 客户端直接使用 HTTPS。Nginx 覆盖 `X-Forwarded-Proto`；写入来源校验只信任来自回环地址的代理头，仍拒绝跨站来源。证书、续期、回退和整机验收见 [共享 HTTPS 运维](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/https.md)（服务器副本 /opt/server-context/references/https.md）。本项目的后端与发布检查保留本机 HTTP，127.0.0.1:80 的代理检查入口不能从公网访问。公网已接入 ServerPortal 统一设备认证：先在 /portal/login 输入口令授权设备，随后使用同源 Secure/HttpOnly Cookie 访问；未授权 API 返回 401。本机发布检查与服务间调用保留。

维护前使用 server-operations，显式 `ssh ali 'cat /opt/AGENTS.md'`。共享 Nginx/server-context 不属于本项目。真实主机地址从受信 SSH 配置取得，不写入 public 仓库。

## 运行布局

- `/opt/fabricworld/bin/fabricworld`：内嵌网页的 Linux amd64 程序。
- `/opt/fabricworld/config/`：unit、备份 timer、Nginx location；root 管理。
- `/opt/fabricworld/data/fabricworld.db`：SQLite；同目录 media/tmp/maintenance.lock。
- `/opt/fabricworld/backups/`：数据库与媒体快照。
- `/opt/fabricworld/docs/`、AGENTS.md、docs/SOURCE：已提交文档副本与来源。
- `/opt/fabricworld/releases/`、current-commit：发布材料与运行提交。

运行用户 fabricworld，发布用户 fabricworld-deploy。data/backups 0700，文件 0600；其余目录/程序 root 管理。监听 127.0.0.1:18082，通过 `/fabricworld/` 访问。无需新增云安全组端口。

## 首次安装

在已授权的新空目录执行。先检查目录、身份、18082 端口、所有现有应用健康与 Nginx 配置。安装软件遵循 software-installation；Ubuntu 官方源依赖：

```sh
apt-get --simulate install --no-install-recommends libvips-tools libheif-plugin-libde265
DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install -y --no-install-recommends libvips-tools libheif-plugin-libde265
```

确认包计划后安装；保留主机现有工具，不升级/删除无关服务。本地 `make linux` 构建，校验产物 SHA-256 后上传隔离暂存目录，管理员执行：

```sh
bash deploy/install.sh /path/to/fabricworld-linux-amd64
ln -s /opt/fabricworld/config/nginx-location.conf /etc/nginx/app-locations/fabricworld.conf
nginx -t
systemctl reload nginx
```

install 拒绝现有安装，显式初始化空库。接入前必须在隔离测试库完成照片和备份恢复验证；正式库不导入示例数据。新 location 不修改旧应用前缀。检查本地与代理 health、/fabricworld/new 深链接及 assets；复查 Ledger/FeeTable。

## 本地开发

Go 使用 go.mod 中的工具链；若本机 goenv 包装器先检查尚未安装的指定版本，可使用已安装 Go 的 GOTOOLCHAIN=auto 下载/选择项目工具链，不修改系统默认版本。本次环境使用 `GOENV_VERSION=1.24.0 go ...`，实际执行 go1.26.8。

安装 libvips（macOS 推荐 Homebrew vips），然后：

```sh
npm --prefix web run build
go build -o bin/fabricworld ./cmd/fabricworld
bin/fabricworld init --data .local/dev-data
bin/fabricworld serve --data .local/dev-data --with-prefix
```

升级已有本地数据目录前先备份，再执行 bin/fabricworld migrate --data .local/dev-data；缺少材质目录或成品扩展时 check/serve 会拒绝启动，不能用 init 覆盖已有库。

本地地址为 `http://127.0.0.1:18082/fabricworld/`。日常前端热更新可运行 web 的 npm run dev，API 代理到不带 --with-prefix 的本地后端。测试数据全部在 .local，Git 忽略。

测试浏览器可安装在项目忽略目录：

## 备份

每天北京时间 03:30 加 0–5 分钟随机延迟，保留 14 份 daily。备份 unit 的可写目录必须是整个 /opt/fabricworld，原因见共享的 [systemd 沙箱下照片硬链接备份失败](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#systemd-沙箱下照片硬链接备份失败)。任务先清理过期回收站/暂存/操作结果，再生成一致性快照和图片硬链接；文件清单含 SHA-256。不要编辑或覆盖快照里的图片，它们与正式不可变图片共享 inode。只有完整备份才有 manifest.json。

```sh
systemctl status fabricworld-backup.timer
journalctl -u fabricworld-backup.service -n 50 --no-pager
runuser -u fabricworld -- /opt/fabricworld/bin/fabricworld backup --data /opt/fabricworld/data --out /opt/fabricworld/backups/manual-UNIQUE
```

backup 输出目录必须不存在。锁冲突返回失败，检查日志后重试，不伪造成功。每日任务可用 `systemctl start fabricworld-backup.service` 触发。普通发布另外生成 before-deploy 快照；发布历史和这些快照目前需人工按明确目录保留/清理，不计入 daily 14 份。注意 df/du；本机快照不是异机备份。2026-09-27 已另取一份包含本项目数据库和照片的全应用数据归档，下载到维护电脑并校验，见 [共享备份说明](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md#手工数据归档)；当前没有自动异机同步。

## 隔离恢复与生产恢复

先核对隔离端口 19082 空闲。18083 已分配给 RecipeBox，不能用作恢复演练监听。

```sh
fabricworld restore --source /path/to/backup --out /path/to/new-restore-directory
fabricworld migrate --data /path/to/new-restore-directory
fabricworld check --data /path/to/new-restore-directory
fabricworld serve --data /path/to/new-restore-directory --listen 127.0.0.1:19082 --with-prefix
```

restore 验证清单、限制相对路径、复制为独立文件，拒绝已存在的目标。备份源和恢复目标上级目录都须对运行身份可访问。验证照片、封面、回收站、尺寸和记录。生产恢复先确认时点和数据损失，停服务，保留当前数据目录，恢复到独立目录并检查后切换，设置正确所有者，再启动。不能把程序回退当成数据库回退。

## 诊断与验证

```sh
systemctl status fabricworld fabricworld-backup.timer
journalctl -u fabricworld -n 100 --no-pager
curl --fail http://127.0.0.1:18082/healthz
curl --fail http://127.0.0.1/fabricworld/healthz
systemctl show fabricworld -p MemoryCurrent -p MemoryPeak -p NRestarts
df -h /opt/fabricworld
du -sh /opt/fabricworld/data /opt/fabricworld/backups
vips --version
```

常见失败：415 文件类型不支持；422 HEIC 解码或超限；429 图片处理/备份锁/限速；507 磁盘或媒体配额；409 多设备版本冲突。缺库时查目录、权限与备份，不执行 init。

图片处理支持已验证的 libvips 8.15/8.18，色彩参数使用二者共用的 `--export-profile=srgb`。8.15 不支持新版名称 `--output-profile`；转换失败会在 journal 记录子进程错误，页面返回可读提示。

## Ledger 联动运行约定

Ledger 从本机 HTTP 调用 /api/integrations/ledger，默认目标 127.0.0.1:18082；配置归 Ledger 的 LEDGER_FABRICWORLD_URL。FabricWorld 无新增配置、共享数据库或服务依赖；先发布 FabricWorld 接口，再发布 Ledger 弹窗。Nginx、运行身份和权限保持原有配置。

每日清理长期保留 fingerprint 以 ledger: 开头的 operations（来源与首次结果），保证跨日重试不重复建布料。该记录与数据库一起备份恢复，不放到浏览器或 Ledger 数据库。旧版清理不认识此保留规则；长期回退旧版前停用联动并核对来源记录。恢复旧备份时需核对备份之后的同步记录，不把程序回退当成数据库恢复。

## 材质字段兼容

材质百分比作为可选 JSON 字段存入现有 schema v1，无需表结构迁移。新版服务兼容旧客户端省略百分比字段的保存请求；旧版服务不认识该字段，回退后编辑记录会丢失其百分比，回退期间应暂停资料编辑并保留发布前备份。

独立材质目录使用新增 material_catalog(name,id)，允许零使用名称并持久保存；布料计数仍从当前记录计算。目录扩展创建这张兼容表；完整 migrate 也会补充成品结构，保留 user_version=1 和所有布料/照片数据。首次安装直接建表；旧库发布先用旧程序备份，候选程序 migrate 后再 check（已安装的发布脚本已包含这一步）。移除也更新回收站记录，恢复布料不会重新带回已移除的材质。操作结果保留 7 天，遇到未知结果先用原键查询或重试；集合版本过期时重新核对当前目录。前一版程序可读取扩展后的 v1 数据库，忽略独立目录表；回退期间不会展示零使用名称，但不会清除其持久数据。回退程序不会撤销已完成的批量移除，历史中保留修改前后快照。恢复升级前备份后需显式 migrate 才能启动新版。生产验收只读检查目录、页面及健康，批量写入回归使用隔离合成库。

## 成品模块升级

成品使用同一个数据库与 media 目录，无新增运行时、Nginx、unit、端口、权限或备份任务。新增 works/work_changes、media.work_id 及兼容保护需显式迁移；生产授权后走已有 本机发布脚本的备份 → migrate → check → 启动流程。清理同时覆盖成品回收站与照片。旧程序回退期间仍可读写布料，忽略成品，照片保护触发器阻止旧清理误删成品照片；备份仍覆盖全部媒体。完整步骤与限制见 [WORKS.md](WORKS.md)。

## 文档同步

文档提交推送后运行 `~/agent-config/skills/server-operations/scripts/sync-docs.sh FabricWorld`，它负责漂移检查、安装到 /opt/fabricworld、逐文件校验、docs/SOURCE 和清理（用法见 server-operations 的 maintenance）。共享清单改动后不带参数运行同一脚本，同时同步 /opt/server-context 与 /opt/AGENTS.md。

## ServerPortal 接入材料

`deploy/portal.json` 是本应用资源说明的维护源。本机发布使用 server-operations 校验器检查，再将同一声明与二进制保存到同一本地版本目录；发布前执行 `portal-check`，发布后执行 `portal`，通过现有受限 SSH 安装到 `/opt/fabricworld/config/portal.json` 并核对采集器实际加载的 SHA-256。`config/portal-source.json` 记录声明来源提交；它与程序的 current-commit 各自表示不同材料的版本。

门户从 `/opt/serverportal/registry.d/fabricworld.json` 的受控链接发现本应用，声明成功更新后自动加载，无需重启。首次正常 本机发布也会建立链接，无需再编辑门户中央应用列表。普通发布可更新本应用的声明，其他 unit/env/Nginx/发布脚本仍由管理员安装。

源码或数据库行为变更不能使用该选项代替程序发布。

数据根、媒体、备份格式、unit、端口或访问路径变化时，同一提交维护声明及对应文档，更新共享清单并核对资源覆盖。文件、媒体、数据库表和 systemd 状态由门户自动读取；目录用途、API 说明和权限边界须由维护 agent 明确更新。共同协议、失败处置与新应用接入见 [门户维护](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/portal.md)。

门户 /portal/ 已统一保护公网访问，发布脚本通过回环检查应用健康，本机发布公网检查预期未授权返回 401。设备授权永久有效至主动撤销，Cookie 经共享 Nginx 随有效请求续期；本应用若新增 add_header，必须保留共享 Set-Cookie 转发，规则及验收见共享门户维护文档。门户备份使用本应用原生一致性快照；真实完整链恢复验收按用户要求暂缓，不因本次维护自动继续下载或恢复。

## 手机桌面图标

现有 /fabricworld/apple-touch-icon.png 为 180×180；Nginx 规则仅放行它与 favicon.ico、fabricworld.svg 的 GET/HEAD，保留版本查询参数。页面、API 和用户媒体继续使用设备认证。共同原因、部署状态与手机验收见[共享排障记录](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#统一认证后-iphone-桌面图标缺失)。

## 当前发布入口

本项目为个人使用：在本地验证本次改动即可发布，不设全量回归门槛，不默认新增或保留永久测试。界面改动检查实际使用的电脑/手机场景；数据迁移、批量写入/删除和备份恢复先用隔离副本针对性验证。

完整流程见 [本机发布与回退](DEPLOYMENT.md)。GitHub 只保存源码；本机 `make release` 构建，`make deploy` 更新生产，文档单独同步。
