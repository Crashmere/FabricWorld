# HTTP API

外部前缀 `/fabricworld/`，Nginx 去前缀；下列为 Go 内部路径。JSON 采用 camelCase。所有写入要求 `Idempotency-Key` 为 32 位小写十六进制随机串；更新/删除/恢复提交当前 revision。

| 方法与路径 | 内容 |
| --- | --- |
| GET /healthz | 数据库可连接时返回 status=ok |
| GET /api/fabrics | items/total/offset/stats；q、status(stock/unused/using/used/all)、material、color、location、tag、min_width/min_length(cm)、sort(purchase/updated/name)、offset、trash=1 |
| POST /api/fabrics | 创建完整布料表单，返回 Fabric |
| GET /api/fabrics/{id} | 详情，包括 pieces、photos、photoIds、revision、deletedAt |
| PUT /api/fabrics/{id} | 保存完整表单；action=remnant 表示余料更新 |
| DELETE /api/fabrics/{id} | body 包含 revision，软删除，返回新版本 |
| POST /api/fabrics/{id}/restore | body 包含 revision，恢复 30 天内删除记录 |
| GET /api/fabrics/{id}/changes | 最近 100 次修改，包含 before/after 业务快照 |
| POST /api/uploads | multipart 第一个 part 为 file；单文件，返回 Media |
| GET /api/operations/{key} | 原写入结果；404 表示尚未查到，不能直接等同失败 |
| GET /api/suggestions | materials/tags/location/color 去重提示，各最多 200 项 |
| GET /api/export | format=csv/zip，与列表筛选相同；默认 stock，禁止回收站导出 |
| GET /media/{id}/main | 标准 JPEG；仅有效暂存或未删除记录图片 |
| GET /media/{id}/thumb | WebP 缩略图 |

Fabric 包含 name、materials、composition、color、tags、status、location、purchaseDate、shop、price、notes、pieces、photoIds。Price 为十进制字符串，空表示未知。Piece 包含 width/length 十进制字符串、unit cm/m、count、irregular、note；widthMM/lengthMM 由服务器重算。只读字段 ID、时间、金额分值等由服务端覆盖，不能用于绕过校验。

错误包含 code/message/field，HTTP 400 格式、403 来源、404 不存在、409 冲突、410 过期、413 过大、415 不支持格式、422 字段无效、429 并发/频率/维护繁忙、507 磁盘或配额不足。移动端在网络中断/5xx 未知结果下先查原幂等键；不要生成新键盲目重试创建。
