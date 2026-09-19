# HTTP API

外部前缀 `/fabricworld/`，Nginx 去前缀；下列为 Go 内部路径。JSON 采用 camelCase。所有写入要求 `Idempotency-Key` 为 32 位小写十六进制随机串；更新/删除/恢复提交当前 revision。

| 方法与路径 | 内容 |
| --- | --- |
| GET /healthz | 数据库可连接时返回 status=ok |
| GET /api/fabrics | items/total/offset/stats；q、status(stock/unused/using/used/all)、material、color、location、tag、min_width/min_length(cm)、sort(purchase/updated/name)、offset、trash=1 |
| POST /api/fabrics | 创建完整布料表单，返回 Fabric |
| POST /api/integrations/ledger | 按 Ledger transactionId 幂等创建布料，返回当前 Fabric |
| GET /api/fabrics/{id} | 详情，包括 pieces、photos、photoIds、revision、deletedAt |
| PUT /api/fabrics/{id} | 默认保存完整表单；action=remnant 仅更新 pieces/status，需 revision |
| DELETE /api/fabrics/{id} | body 包含 revision，软删除，返回新版本 |
| POST /api/fabrics/{id}/restore | body 包含 revision，恢复 30 天内删除记录 |
| GET /api/fabrics/{id}/changes | 最近 100 次修改，包含 before/after 业务快照 |
| POST /api/uploads | multipart 第一个 part 为 file；单文件，返回 Media |
| GET /api/operations/{key} | 原写入结果；404 表示尚未查到，不能直接等同失败 |
| GET /api/suggestions | materials/tags/location/color 去重提示，各最多 200 项 |
| GET /api/materials | 已保存材质数组；每项 name/count/trashCount/version，包含回收站 |
| POST /api/materials/remove | body 包含 name/version，原子移除全部相关布料的该材质；返回 name/affected |
| GET /api/export | format=csv/zip，与列表筛选相同；默认 stock，禁止回收站导出 |
| GET /media/{id}/main | 标准 JPEG；仅有效暂存或未删除记录图片 |
| GET /media/{id}/thumb | WebP 缩略图 |

Fabric 包含 name、materials、materialPercentages、composition、color、tags、status、location、purchaseDate、shop、price、notes、pieces、photoIds。Price 为十进制字符串，空表示未知。Piece 包含 width/length 十进制字符串、unit cm/m、count、irregular、note；widthMM/lengthMM 由服务器重算。只读字段 ID、时间、金额分值等由服务端覆盖，不能用于绕过校验。

materialPercentages 为可选的材质名称 → 十进制字符串映射，例如 `{"棉":"75.1250","麻":"50.5"}`。只保存已选材质的比例；单选固定 100，多选可以留空，不限制各项或总和为 100，不舍入输入。每项最多 20 字符，仅接受非负十进制数字或空串。composition 仍是独立说明文本。旧记录可以没有该字段，旧客户端修改时省略字段会保留仍选中的材质比例；传空对象或空串可明确清空多选比例。CSV 材质列带百分比，ZIP JSON 保留独立映射。

材质目录的 count 是使用该名称的布料总数，包含已用完及回收站；trashCount 是其中回收站数量。version 是相关布料按 ID 排序后的 ID/revision 集合的 SHA-256 摘要（64 位十六进制字符串），客户端从目录原样提交。移除请求需 Idempotency-Key，按名称精确匹配；相关集合有变化返回 409 / material_changed，刷新目录后重新确认。移除材质及其百分比，剩一项时设为 100，其余资料、照片和独立成分说明保留；相关布料 revision 增加，历史动作是 remove_material。无部分成功，幂等结果可用 operations 接口查询。目录从当前记录生成，不包含仅存在于历史快照中的名称。

Ledger 联动输入为 {transactionId,name,purchaseDate,price}，transactionId 为标准小写 UUID，日期与总价必填。该接口以来源 ID 代替 Idempotency-Key；相同 ID 永远指向首次创建的布料，后续调用只返回现有记录，不更新字段。来源记录长期保留，已删除/清理的布料返回 409/404，不重新创建。沿用本站来源检查和写入限速，不新增身份认证。

名称最多 500 字符（兼容 Ledger 标题），空名称在联动中使用“未命名布料”；日期原样映射，金额为十进制元。状态未使用，默认一组宽长未知、1 片，其余资料和图片为空；客户端跳转 /fabricworld/fabrics/:id/edit 补全。

余料请求只需 `{revision, action: "remnant", status: "using" | "used", pieces}`，返回完整 Fabric。used 必须传空 pieces；其他有效状态兼容旧客户端并归为 using。只作用于已有布料，不能用余料动作创建记录。即使携带 name、photoIds 等旧表单字段，也不会修改资料和照片归属；版本过期仍返回 409。

错误包含 code/message/field，HTTP 400 格式、403 来源、404 不存在、409 冲突、410 过期、413 过大、415 不支持格式、422 字段无效、429 并发/频率/维护繁忙、507 磁盘或配额不足。移动端在网络中断/5xx 未知结果下先查原幂等键；不要生成新键盲目重试创建。
