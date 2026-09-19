# 检查与发布

仓库为 public。推送 main / PR 运行 Build and verify：安装官方 libvips/HEIC 解码器，前端构建，Go race 测试与 vet、前端单测、shell 语法、隔离 Chromium 业务流程，再构建 Linux amd64 产物。部署独立使用手动 Deploy FabricWorld 工作流，只允许 main，并在发送前核对当前 main 提交。

production 环境 secrets：SSH_HOST、SSH_USER、SSH_PRIVATE_KEY、SSH_KNOWN_HOSTS。通过已受信连接核实主机公钥；不关闭严格主机校验。部署账号 fabricworld-deploy 的 authorized_keys 使用 restrict 和强制命令，只有 `deploy <commit> <sha256>`，没有 shell/SCP/端口转发。root 管理强制命令、发布脚本和 sudoers。

setup-ci.sh 接受一份 Ed25519 公钥，拒绝已有身份；私钥只进入 GitHub Secrets。该身份拥有 FabricWorld 代码/数据权限，不是整机管理员。普通发布不能修改 Nginx、unit、部署脚本或服务器文档。

发布脚本验证尺寸、SHA-256 和提交格式，保存旧程序，停止 FabricWorld，生成数据库+图片备份，以应用身份运行候选 check，再原子替换启动。健康失败恢复旧程序；不自动恢复数据库。发布和每日备份可能锁冲突，发布失败时旧程序重启并报告错误。

每次发布保留 releases/ 中的候选、previous、metadata、result，成功后更新 current-commit。历史和 before-deploy 备份暂由运维定期清理（先明确保留点）；daily 自动保留 14 份。首次安装由管理员使用 install.sh，后续手动发布通过 CI 入口验证。
