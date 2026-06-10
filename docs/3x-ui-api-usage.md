# SubForMe 调用 3x-ui API 清单

本文档记录 SubForMe 当前实际调用的 3x-ui API，方便在 3x-ui 升级时快速判断是否会影响本项目。

## 调用约定

- 认证方式：优先使用 3x-ui API Token，通过 `Authorization: Bearer <token>` 请求头调用。
- 请求头：所有请求都会带 `X-Requested-With: XMLHttpRequest`。
- 默认路径：优先调用 `/panel/api/...`。
- 兼容路径：客户端还会依次尝试 `/xui/panel/api/...`、`/api/...`，用于兼容不同 base path 或旧部署形态。
- 超时：3x-ui 客户端默认 HTTP 超时为 15 秒；流量刷新单台面板还有额外的刷新超时控制。

## 实际业务调用

| API | 方法 | SubForMe 功能 | 大致用途 | 升级时重点观察 |
| --- | --- | --- | --- | --- |
| `/panel/api/inbounds/list` | `GET` | 测试面板连接、导入主面板、同步面板、节点解析、本地 inbound 缓存 | 获取面板上的 inbound 列表、节点配置、用户挂载关系、clientStats 流量统计 | 返回结构是否仍是 `success + obj/data`；`settings`、`streamSettings`、`sniffing` 是否仍包含完整配置；`settings.clients` 是否仍可读取 |
| `/panel/api/inbounds/add` | `POST` | 同步到非主面板 | 当目标面板缺少主面板对应 inbound 时，创建 inbound；SubForMe 会去掉 `settings.clients`，只同步 inbound 配置本身 | 请求体是否仍支持 JSON；`settings`、`streamSettings`、`sniffing` 是否仍接受嵌套 JSON 对象 |
| `/panel/api/inbounds/update/{id}` | `POST` | 同步到非主面板 | 当目标面板已有 inbound 但配置和主面板不同，更新 inbound 配置；同样会去掉 clients | 请求体是否仍支持 JSON；更新语义是否仍是替换完整 inbound 配置 |
| `/panel/api/inbounds/del/{id}` | `POST` | 同步到非主面板 | 删除目标面板上主面板已经不存在的 stale inbound | 路径和空 body 调用是否仍支持 |
| `/panel/api/clients/list` | `GET` | 流量刷新、手动/定时清零前统计、同步面板用户差异判断 | 获取全局 client 列表、每个用户挂载的 inbound IDs、用户配置、全局流量记录 | `traffic.up/down`、`inboundIds`、`email`、`enable`、`uuid/password/subId` 等字段是否还在 |
| `/panel/api/clients/add` | `POST` | 同步到非主面板 | 在目标面板创建用户，并一次性挂载到指定 inbound IDs | 请求体 `{ client, inboundIds }` 是否仍支持；字段名是否变化 |
| `/panel/api/clients/update/{email}` | `POST` | 同步到非主面板 | 按 email 更新目标面板用户配置，并传播到其已挂载的 inbound | email 路径参数是否仍支持；更新语义是否仍是替换完整 client |
| `/panel/api/clients/del/{email}` | `POST` | 同步到非主面板 | 删除目标面板上本地已不存在或不该同步过去的用户 | 路径和删除语义是否仍支持；是否默认删除 traffic row |
| `/panel/api/clients/{email}/attach` | `POST` | 同步到非主面板 | 把已有用户挂载到新增的 inbound IDs | 请求体 `{ inboundIds: [...] }` 是否仍支持 |
| `/panel/api/clients/{email}/detach` | `POST` | 同步到非主面板 | 把已有用户从不再需要的 inbound IDs 解绑 | 请求体 `{ inboundIds: [...] }` 是否仍支持；解绑后 orphan client 的处理是否变化 |
| `/panel/api/clients/resetAllTraffics` | `POST` | 手动/定时面板流量清零 | 清空该面板所有 client 的 up/down 计数，然后同步清空 SubForMe 本地缓存 | 接口是否仍是全局清零；是否仍不影响 quota/expiry；是否需要新的权限 |
| `/panel/api/server/restartXrayService` | `POST` | 预留 Xray 重启能力 | 触发面板重启 Xray；当前业务主流程很少直接调用 | 返回码是否仍支持 `200 OK` 或 `202 Accepted` |

## 间接依赖的字段

这些不是单独 API，但 3x-ui 升级时如果字段变化，会影响 SubForMe 的解析或同步。

| 来源 API | 字段 | SubForMe 用途 |
| --- | --- | --- |
| `inbounds/list` | `id` | 作为 3x-ui inbound ID，用于更新、删除、挂载用户 |
| `inbounds/list` | `remark`、`tag`、`protocol`、`listen`、`port` | 生成本地节点、判断同步目标 inbound、生成订阅节点 |
| `inbounds/list` | `settings.clients[]` | 从主面板导入用户、重建用户和 inbound 的关联 |
| `inbounds/list` | `streamSettings` | 解析 reality/tls/xhttp 等节点参数，用于生成订阅 |
| `inbounds/list` | `sniffing`、`trafficReset`、`total`、`expiryTime`、`enable` | inbound 配置同步和本地缓存 |
| `clients/list` | `email` | 用户唯一匹配键 |
| `clients/list` | `inboundIds` | 判断用户应挂载/解绑哪些 inbound |
| `clients/list` | `traffic.up`、`traffic.down` | 面板流量读取并写入 SubForMe 本地数据库 |
| `clients/list` | `uuid`、`password`、`auth`、`flow`、`security`、`subId` | 用户配置同步、订阅节点生成 |
| `clients/list` | `totalGB`、`expiryTime`、`limitIp`、`tgId`、`reset`、`comment`、`enable` | 用户配置同步 |

## 代码中保留但当前业务未使用的旧接口

以下方法还在 `backend/internal/xui/client.go` 中保留，但当前主业务没有调用。它们属于旧的 inbound-client 风格接口；如果未来要清理代码，可以优先考虑移除或确认新版 3x-ui 是否仍支持。

| API | 方法 | 本项目状态 | 说明 |
| --- | --- | --- | --- |
| `/panel/api/clients/resetTraffic/{email}` | `POST` | 方法存在，当前业务未调用 | 单用户清零；现在项目使用全局 `resetAllTraffics` |
| `/panel/api/inbounds/addClient` | `POST` | 方法存在，当前业务未调用 | 旧式给单个 inbound 添加 client |
| `/panel/api/inbounds/updateClient/{clientID}` | `POST` | 方法存在，当前业务未调用 | 旧式按 UUID/password 更新 inbound 内 client |
| `/panel/api/inbounds/{id}/delClient/{clientID}` | `POST` | 方法存在，当前业务未调用 | 旧式按 UUID/password 删除 inbound 内 client |
| `/panel/api/inbounds/{id}/delClientByEmail/{email}` | `POST` | 方法存在，当前业务未调用 | 旧式按 email 删除 inbound 内 client |

## 3x-ui 升级检查建议

每次 3x-ui 发布新版时，优先检查 release notes 或 OpenAPI 中这些点：

1. `/panel/api/inbounds/list`、`/panel/api/clients/list` 是否有字段结构变化。
2. `/panel/api/clients/*` 全局 client API 是否还保留，尤其是 `add`、`update/{email}`、`attach`、`detach`、`del/{email}`。
3. `/panel/api/clients/resetAllTraffics` 是否仍是清空所有 client 流量。
4. `/panel/api/inbounds/add` 和 `/panel/api/inbounds/update/{id}` 是否还接受 JSON 请求体。
5. Bearer API Token 是否仍适用于所有 `/panel/api/*` 接口。

## 当前已知风险

- 3x-ui 3.3.0 的 breaking change 是 `/panel/setting`、`/panel/xray` 移动到 `/panel/api/setting`、`/panel/api/xray`；SubForMe 当前没有调用这两组旧路径。
- `inbounds/add` 和 `inbounds/update/{id}` 已按 3x-ui 3.3.0 OpenAPI 调整为 JSON body；同步时仍会清空 `settings.clients`，避免把主面板 inbound 内的用户列表直接复制到目标面板。
