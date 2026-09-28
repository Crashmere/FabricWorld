# HTTP API

外部前缀 `/fabricworld/`，Nginx 去前缀；下列为 Go 内部路径，`{id}`、`{key}` 为路径参数。公网 API 与媒体由 ServerPortal 统一设备认证保护，未授权返回 401；本机回环调用保留。JSON 采用 camelCase。除 Ledger 联动使用 transactionId 外，写入要求 `Idempotency-Key` 为 32 位小写十六进制随机串；更新/删除/恢复布料或成品提交当前 revision，材质移除使用目录 version。

| 方法与路径 | 内容 |
| --- | --- |
| GET /healthz | 数据库可连接时返回 status=ok |
| GET /api/fabrics | items/total/offset/stats，24 条/页；查询参数 q、status(stock/unused/using/used/all)、material、color、location、tag、min_width/min_length(cm)、sort(purchase/updated/name)、offset、trash=1 |
| POST /api/fabrics | 创建完整布料表单，返回 Fabric |
| POST /api/integrations/ledger | 按 Ledger transactionId 幂等创建布料，返回当前 Fabric |
| GET /api/fabrics/{id} | 详情，包括 pieces、photos、photoIds、revision、deletedAt |
| PUT /api/fabrics/{id} | 默认保存完整表单；action=remnant 仅更新 pieces/status，需 revision |
| DELETE /api/fabrics/{id} | body 包含 revision，软删除，返回新版本 |
| POST /api/fabrics/{id}/restore | body 包含 revision，恢复 30 天内删除记录 |
| GET /api/fabrics/{id}/changes | 最近 100 次修改，包含 before/after 业务快照 |
| POST /api/uploads | multipart/form-data，第一个 part 为 file；单张最大 25 MiB，成功返回 200 和 Media |
| GET /api/operations/{key} | 原写入结果；404 表示尚未查到，不能直接等同失败 |
| GET /api/suggestions | materials 合并独立目录与未删除布料材质；tags/location/color 各最多 200 项 |
| GET /api/materials | 已保存材质数组；每项 name/count/trashCount/version，包含回收站 |
| POST /api/materials | body 包含 name（去首尾空白后 1–40 字符），持久添加零使用材质；返回 name，要求 Idempotency-Key |
| POST /api/materials/remove | body 包含 name/version，原子移除全部相关布料的该材质；返回 name/affected |
| GET /api/export | 查询参数 format=csv/zip，与列表筛选相同；status 默认 stock，忽略分页与 trash，始终排除回收站 |
| GET /media/{id}/main | 标准 JPEG；仅有效暂存或未删除记录图片 |
| GET /media/{id}/thumb | WebP 缩略图 |

布料列表的 status 省略或为 all 时包含全部未删除状态，sort 默认 purchase，offset 默认 0；trash=1 改为查询回收站。total 是当前筛选条数，stats 的 total/stock/incomplete 始终按全部未删除布料统计，不随筛选变化。

## 成品接口

| 方法与路径 | 内容 |
| --- | --- |
| GET /api/works | items/total/offset/categories；q、category、fabric（来源 ID）、sort(completed/updated/name)、offset、trash=1，每页 24 件 |
| POST /api/works | 创建完整 Work，名称必填；返回 Work |
| GET /api/works/{id} | 成品详情，含照片与布料来源可用性 |
| PUT /api/works/{id} | 完整编辑，提交当前 revision |
| DELETE /api/works/{id} | body 包含 revision，软删除，返回新版本 |
| POST /api/works/{id}/restore | body 包含 revision，恢复 30 天内删除的成品 |
| GET /api/works/{id}/changes | 最近 100 次，含 before/after 成品快照 |
| GET /api/works/export | format=csv/zip，使用相同搜索/类别/布料筛选，忽略分页并排除回收站 |

以上两表覆盖当前业务、健康和媒体接口；门户的 `deploy/portal.json.apis` 与两表保持一致。每个方法和路径单独登记，查询参数与 body 的 action 不属于路径。成品列表 sort 默认 completed，offset 默认 0；categories 是全部未删除成品的类别，不受当前筛选限制。 核对与同步流程见[共享排障说明](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#门户-api-列表与服务不一致)。

Work 包含 name、category、completedDate、pattern、size、recipient、tags、notes、fabrics、photoIds，以及服务端 id/revision/photos/createdAt/updatedAt/deletedAt。名称必填，日期可空，fabrics 为 `{fabricId,name,note,available}` 数组；输入只使用 fabricId 和 note，name 由服务器从来源生成快照，available 为实时读取状态。已用完布料可关联；新关联不能使用已删除或不存在的布料，既有失效关联可继续保留。来源改变不自动更新成品 revision，保存成品不修改库存。

写入使用 Idempotency-Key 和原 operations 查询；revision_conflict、key_reused、deleted、expired、validation 沿用错误格式。成品照片从原 uploads 上传，媒体只能绑定一个成品或布料；删除期间无法通过 media 接口查看，恢复后重新可用。字段上限、页面及迁移见 [WORKS.md](WORKS.md)。

表单写入的同一幂等键须保持 HTTP 方法、内部路径和 body 字节一致；上传按原文件字节识别重放，不要求 multipart 边界相同。普通操作结果由维护清理保留约 7 天，Ledger 来源记录长期保留。`GET /api/operations/{key}` 返回原提交结果，可能是 Fabric、Work、Media 或材质操作结果，不代表资源之后的最新状态。上传支持 JPEG、PNG、WebP、HEIC/HEIF；暂存照片须在 24 小时内绑定。媒体仅接受 main/thumb，照片已移除、所属记录已删除或暂存已过期时返回 404。

## 布料字段与联动

Fabric 包含 name、materials、materialPercentages、composition、color、tags、status、location、purchaseDate、shop、price、notes、pieces、photoIds。Price 为十进制字符串，空表示未知。Piece 包含 width/length 十进制字符串、unit cm/m、count、irregular、note；widthMM/lengthMM 由服务器重算。只读字段 ID、时间、金额分值等由服务端覆盖，不能用于绕过校验。

materialPercentages 为可选的材质名称 → 十进制字符串映射，例如 `{"棉":"75.1250","麻":"50.5"}`。只保存已选材质的比例；单选固定 100，多选可以留空，不限制各项或总和为 100，不舍入输入。每项最多 20 字符，仅接受非负十进制数字或空串。composition 仍是独立说明文本。旧记录可以没有该字段，旧客户端修改时省略字段会保留仍选中的材质比例；传空对象或空串可明确清空多选比例。CSV 材质列带百分比，ZIP JSON 保留独立映射。

材质目录的 count 是使用该名称的布料总数，包含已用完及回收站；trashCount 是其中回收站数量。version 是独立目录身份（如有）与相关布料按 ID 排序后的 ID/revision 集合的 SHA-256 摘要（64 位十六进制字符串），客户端从目录原样提交。移除请求需 Idempotency-Key，按名称精确匹配；相关集合有变化返回 409 / material_changed，刷新目录后重新确认。移除材质及其百分比，剩一项时设为 100，其余资料、照片和独立成分说明保留；相关布料 revision 增加，历史动作是 remove_material。无部分成功，幂等结果可用 operations 接口查询。目录合并当前布料与 material_catalog，不包含仅存在于历史快照中的名称。独立添加的名称在零使用时仍保留；移除同时删除其目录项。重复添加同名材质不生成重复条目，操作结果与目录写入在同一事务提交；重放返回旧结果，不重新创建后来已被移除的名称。重新添加使用新的目录身份，过期移除版本不能删除新条目。

Ledger 联动输入为 {transactionId,name,purchaseDate,price}，transactionId 为标准小写 UUID，日期与总价必填。该接口以来源 ID 代替 Idempotency-Key；相同 ID 永远指向首次创建的布料，后续调用只返回现有记录，不更新字段。来源记录长期保留，已删除/清理的布料返回 409/404，不重新创建。沿用本站来源检查和写入限速，不新增身份认证。

名称最多 500 字符（兼容 Ledger 标题），空名称在联动中使用“未命名布料”；日期原样映射，金额为十进制元。状态未使用，默认一组宽长未知、1 片，其余资料和图片为空；客户端跳转 /fabricworld/fabrics/:id/edit 补全。

余料请求只需 `{revision, action: "remnant", status: "using" | "used", pieces}`，返回完整 Fabric。used 必须传空 pieces；其他有效状态兼容旧客户端并归为 using。只作用于已有布料，不能用余料动作创建记录。即使携带 name、photoIds 等旧表单字段，也不会修改资料和照片归属；版本过期仍返回 409。

应用错误为 JSON 对象，包含 code/message 和可选 field；HTTP 400 格式、403 来源、404 不存在、409 冲突、410 恢复期限已过、413 过大、415 不支持格式、422 字段无效、429 并发/频率/维护繁忙、500 内部错误、507 磁盘或配额不足。公网认证失败由共享入口返回 401。移动端在网络中断/5xx 未知结果下先查原幂等键；不要生成新键盲目重试创建。
