# 安装、运行与恢复

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

确认包计划后安装；保留主机现有工具，不升级/删除无关服务。CI 或本地 `make linux` 构建，校验产物 SHA-256 后上传隔离暂存目录，管理员执行：

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
npm --prefix web ci
npm --prefix web run build
go build -o bin/fabricworld ./cmd/fabricworld
bin/fabricworld init --data .local/dev-data
bin/fabricworld serve --data .local/dev-data --with-prefix
```

本地地址为 `http://127.0.0.1:18082/fabricworld/`。日常前端热更新可运行 web 的 npm run dev，API 代理到不带 --with-prefix 的本地后端。测试数据全部在 .local，Git 忽略。

浏览器测试需要 Chromium（本地默认使用已安装 Chrome，可设 PW_CHANNEL=chromium）和 Playwright 官方 WebKit。只跑现有 Chrome 流程可用 `npm --prefix web run test:e2e -- --project=chromium`；完整检查还应运行 webkit-purchase 与 webkit-materials。测试浏览器可安装在项目忽略目录：

```sh
PLAYWRIGHT_BROWSERS_PATH="$PWD/.local/playwright" node web/node_modules/playwright/cli.js install webkit
PLAYWRIGHT_BROWSERS_PATH="$PWD/.local/playwright" npm --prefix web run test:e2e
```

## 备份

每天北京时间 03:30 加 0–5 分钟随机延迟，保留 14 份 daily。任务先清理过期回收站/暂存/操作结果，再生成一致性快照和图片硬链接；文件清单含 SHA-256。不要编辑或覆盖快照里的图片，它们与正式不可变图片共享 inode。只有完整备份才有 manifest.json。

```sh
systemctl status fabricworld-backup.timer
journalctl -u fabricworld-backup.service -n 50 --no-pager
runuser -u fabricworld -- /opt/fabricworld/bin/fabricworld backup --data /opt/fabricworld/data --out /opt/fabricworld/backups/manual-UNIQUE
```

backup 输出目录必须不存在。锁冲突返回失败，检查日志后重试，不伪造成功。每日任务可用 `systemctl start fabricworld-backup.service` 触发。普通发布另外生成 before-deploy 快照；发布历史和这些快照目前需人工按明确目录保留/清理，不计入 daily 14 份。注意 df/du；本机快照不是异机备份。

## 隔离恢复与生产恢复

```sh
fabricworld restore --source /path/to/backup --out /path/to/new-restore-directory
fabricworld check --data /path/to/new-restore-directory
fabricworld serve --data /path/to/new-restore-directory --listen 127.0.0.1:18083 --with-prefix
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

## 文档同步

从已推送提交通过 git archive 导出 AGENTS.md 和受跟踪 docs/*.md 白名单，管理员同步到 /opt/fabricworld，最后写 docs/SOURCE（repository、commit、subdirectory、synced_at）。比较 SHA-256；不上传整个工作目录或 .local。共享清单单独维护在 agent-config/server-operations，按其 maintenance 同步 /opt/server-context 与 /opt/AGENTS.md。
