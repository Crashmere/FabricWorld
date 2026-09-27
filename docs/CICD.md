# 检查与发布

仓库为 public。推送 main / PR 运行 CI and deploy：verify 作业安装官方 libvips/HEIC 解码器，前端构建，Go race 测试与 vet、前端单测、shell 语法、隔离 Chromium 业务流程及 WebKit 购买日期、材质、菜单和图片预览回归，再构建 Linux amd64 产物。main 推送（或在 main 上手动运行）时，deploy 作业在检查通过后下载同一产物发布到生产，发送前核对当前 main 提交，已有更新提交时跳过这次发布。纯文档提交加 `[skip ci]`。

Playwright 的 chromium 项目运行全部流程；webkit-purchase 使用 iPhone 视口运行 purchase.spec.ts，webkit-materials 运行 materials.spec.ts 与 material-menu.spec.ts。WebKit 自动化仍是桌面构建，不等同真实 iOS。日期回归额外模拟 padding 被加到百分比宽度的盒模型，以覆盖 WebKit 301648 对应的布局风险。菜单和材质管理回归覆盖 320/375/1440 px、管理页独立添加/复用/移除材质、批量移除、版本冲突以及提交响应丢失后的刷新恢复；新增和复制布料保存后返回主页，编辑后返回详情，新增结果查询后的主页跳转由 fabric.spec.ts 覆盖。

webkit-controls 运行 controls.spec.ts，覆盖顶栏唯一的成品集切换、全部自绘下拉、弹窗焦点与 Escape 边界、长选项、单位换算和状态取消、自由输入建议、减少动态效果，以及 320/375/1440 px 布局。

webkit-photos 运行 photo-preview.spec.ts 与 photo-viewer.spec.ts，覆盖 320/375/1440 px 的本地/已上传照片预览、图片缩放、拖动、复位、切图和焦点。原生双指注入通过 Chromium CDP 验证图片变大且 visualViewport.scale 不变；WebKit 跳过这项 CDP 专用用例，仍运行按钮、鼠标及键盘交互。

成品模块增加 webkit-works，运行 works.spec.ts，覆盖成品照片、布料关联、草稿和并发修改、提交恢复、导出与回收站，以及 320/375/1440 px 长文本布局。涉及生产数据库结构的迁移需依 server-operations 授权表获准后才合入 main；发布顺序和兼容保护见 [WORKS.md](WORKS.md)。

production 环境 secrets：SSH_HOST、SSH_USER、SSH_PRIVATE_KEY、SSH_KNOWN_HOSTS。通过已受信连接核实主机公钥；不关闭严格主机校验。部署账号 fabricworld-deploy 的 authorized_keys 使用 restrict 和强制命令，只有 `deploy <commit> <sha256>`，没有 shell/SCP/端口转发。root 管理强制命令、发布脚本和 sudoers。

setup-ci.sh 接受一份 Ed25519 公钥，拒绝已有身份；私钥只进入 GitHub Secrets。该身份拥有 FabricWorld 代码/数据权限，不是整机管理员。普通发布不能修改 Nginx、unit、部署脚本或服务器文档。

发布脚本验证尺寸、SHA-256 和提交格式，保存旧程序，停止 FabricWorld，生成数据库+图片备份，以应用身份运行候选 migrate 和 check，再原子替换启动。当前 migrate 补充 material_catalog、works、work_changes 与 media.work_id、索引和照片保护触发器，兼容旧程序的 v1 布料格式；迁移失败进入旧程序恢复分支，成功的兼容扩展可在程序回退后保留。健康失败恢复旧程序；不自动恢复数据库。发布和每日备份可能锁冲突，发布失败时旧程序重启并报告错误。

`bash deploy/test-release.sh` 在 Linux 临时目录使用模拟服务检查上述失败分支与成功分支；实际发布脚本保留固定路径和 root 检查。verify 作业运行此测试，包含 migrate 失败分支，不在正式实例上制造故障。发布脚本由 root 管理，修改后需从已提交来源同步到 /opt/fabricworld/bin/deploy-release.sh；普通二进制发布不覆盖它。

每次发布保留 releases/ 中的候选、previous、metadata、result，成功后更新 current-commit。历史和 before-deploy 备份暂由运维定期清理（先明确保留点）；daily 自动保留 14 份。首次安装由管理员使用 install.sh，后续发布由 CI and deploy 完成。

CI and deploy 的发布步骤以 exit code 124 结束、最新发布目录为 failed 且没有 metadata，是 GitHub runner 到服务器的上传超时。不要反复重跑，直接按共享的 [GitHub 上传过慢时的备用发布](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#github-上传过慢时的备用发布)处理（服务器副本 `/opt/server-context/references/common-issues.md`），其中也包括残留清理。FabricWorld 的参数：产物取自失败的 CI and deploy run 本身，artifact `fabricworld-linux`（文件 `fabricworld-linux-amd64`），验收 `/fabricworld/healthz` 与 `/fabricworld/new`，不写测试数据。
