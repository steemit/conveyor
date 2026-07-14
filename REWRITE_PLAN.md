# Conveyor — Go 重构计划（Gin + steemgosdk）· 修订版

> 本版相对初稿合并了技术审计结论，修正 3 处实质性算法错误、去除 2 处重复造轮子，并补充隐患应对。带 `🔧` 为审计修订项。

## 1. 背景与目标

将现有 TypeScript/Node.js（Koa + `@steemit/koa-jsonrpc` + dsteem + Sequelize）的 Conveyor JSON-RPC 2.0 服务，用 **Go + Gin + steemgosdk** 重写：

- **完全向后兼容**：保持 JSON-RPC 2.0 over `POST /` 的对外契约不变；condenser 等消费方零改动。
- **签名鉴权完全兼容**：精确复刻 `__signed` 验证协议，并直接复用 steemutil 现成 crypto。
- **全量功能对等**：21 个 RPC 方法、8 个子系统全部移植。
- **确定性逻辑对等**：feature-flag 的 MT19937 概率算法逐位复刻，避免线上已分配 flag 漂移。

> 本仓库（含所有远程分支）不存在 Go 版本，本计划为全新实现。steemgosdk/steemutil 位于 `/home/ety001/workspace/steemgosdk`、`/home/ety001/workspace/steemutil`，作为本地 `replace` 依赖引入。

> **前置依赖（已交付）**：steemutil v0.0.24–v0.0.25、steemgosdk v0.0.23 已按两份独立计划文档完成。详见：
> - `/home/ety001/workspace/steemutil/STEEMUTIL_TODO.md`（U1 `rpc.VerifySignedRpc`、U3 注释修正、U4 链上结构体 + 价格算术）→ 已落盘为 v0.0.24 / v0.0.25
> - `/home/ety001/workspace/steemgosdk/STEEMGOSDK_TODO.md`（G1 `VerifySignedRequest`、G2 链查询封装、G3 价格算术）→ 已落盘为 v0.0.23
>
> **勘误（原 U2「签名字节布局转换」已撤销）**：初稿及 steemutil TODO 曾要求 `DsteemSigToBtcec` 转换 dsteem `{31,32}` ↔ btcec `{4,5}`。实际验证：btcec `SignCompact`/`RecoverCompact` 的压缩键基值为 27，压缩位 +4，故 recoveryId 字节 = `27+4+recovery = 31+recovery ∈ {31,32}`，**与 dsteem 格式字节级一致，无需任何转换胶水**。最终只加了一个格式守卫 `wif.ValidateCompactSignature`。原 U2 转换函数从未实现、也不需要。

## 2. 技术选型

| 关注点 | TS 现状 | Go 方案 | 说明 |
|---|---|---|---|
| HTTP 框架 | Koa + koa-router | **Gin** | 用户指定 |
| 区块链客户端 | dsteem | **steemgosdk + steemutil** | `api.Call(apiName, method, params)` |
| RPC 协议 | `@steemit/koa-jsonrpc` | **自研** `internal/jsonrpc` | JSON-RPC 2.0 信封 |
| 签名鉴权 | rpc-auth + dsteem crypto | 🔧**复用 steemutil `rpc.Validate`+`wif.*`** | 不另引库自研 crypto |
| 数据库 | Sequelize (SQLite/PG) | **GORM** | auto-migrate、软删除 |
| 对象存储 | s3-blob-store | **AWS SDK Go v2** + memory | draft/flag blob |
| 配置 | node-config (TOML) | **viper** | 兼容 TOML + 环境变量 |
| 日志 | bunyan | **zerolog** | 结构化 JSON |
| 缓存(LRU) | lru-cache | **golang-lru/v2** | summarizer 10k/1h |
| 缓存(TTL) | node-cache | **go-cache** | CachingClient |
| UUID | uuid/v4 | **google/uuid** | draft uuid |
| MT19937 | random-js v1 | 🔧**逐位移植（含 53 位 real）** | ⚠️确定性关键 |
| 前缀树 | trie-prefix-tree | 自研或 tongue | autocomplete |
| HTML 解析 | unfluff | go-readability+goquery | 行为近似替换 |
| 多进程 | cluster | Go goroutine | 无需 fork |

Go 1.21+。模块 `github.com/steemit/conveyor`。

## 3. 目录结构

```
go.mod / go.sum
cmd/conveyor/main.go
internal/
  config/config.go
  jsonrpc/
    server.go                 # 信封、batch、notification、派发
    auth.go                   # __signed 验证（调用 steemutil rpc/wif）
    errors.go / context.go
  blockchain/{client.go,types.go}     # 封装 steemgosdk.API；自建结构体
  store/{store.go,memory.go,s3.go}    # BlobStore 接口 + JSON 助手
  models/{user.go,tag.go,usertag.go}  # GORM 模型
  featureflags/{flags.go,mt19937.go}  # 🔧 含正确 53 位 real
  drafts/drafts.go
  userdata/userdata.go
  tags/tags.go
  prices/prices.go
  summarizer/{summarizer.go,blacklist.go}
  usersearch/{search.go,client.go,indexes.go,lists.go,user.go}
server/{server.go,registry.go}        # Gin 装配 + 21 方法注册
user-data/                            # accounts.js→accounts.json；各列表→.json
```

> 注：签名字节布局无需转换——btcec 压缩签名与 dsteem 格式字节级一致（详见顶部勘误）。conveyor 直接调用 steemutil `rpc.Validate` + `VerifySignedRpc`。

## 4. 逐子系统移植

### 4.1 JSON-RPC 服务器 + 鉴权（最高优先级）

**信封处理**（对齐 koa-jsonrpc）：仅 `POST /`；非 POST→405+InvalidRequest(-32600)；解析失败→400+ParseError(-32700)；空数组 batch→400+InvalidRequest；batch 用 goroutine 并发；notification（无 id 且无 error）不回响应；错误 `{code,message,data?}`，message 沿用 `"<顶层>: <cause>"`。

**`__signed` 验证** 🔧**直接复用已交付的 steemutil + steemgosdk，不自研 crypto**：
1. 🔧**优先**：直接调用 steemgosdk `API.VerifySignedRequest(&signedReq)`（G1，已交付）。它内部用 `condenser_api.get_accounts` 取账户、喂给 steemutil `rpc.VerifySignedRpc`，完成「单 posting key + 单签名 + threshold」校验，返回 `(params, account, error)`。conveyor 无需自写 verifyFunc。
2. 若需绕过 G1 自行编排：`rpc.Validate(&signedReq, myVerifyFunc)`（已含解包、60s 时效、`hashMessage`、nonce 校验、params 解码），`myVerifyFunc` 直接转发给 `rpc.VerifySignedRpc(msg, sigs, acct, fetcher)`。
3. 🔧**签名格式无需转换**：hex→bytes 后直接传给 `wif.RecoverPublicKeyFromSignature`。btcec 压缩签名字节与 dsteem 格式字节级一致（详见顶部勘误），无需 `DsteemSigToBtcec` 胶水。steemutil 已加 `wif.ValidateCompactSignature` 做格式守卫。
   - `pubKey.FromStr(posting.key_auths[0])` 复用现成 WIF 解码（STM 前缀+base58+ripemd160[0:4]），与 dsteem 等价。
   - `bytes.Equal(pubKey.ToByte(), recovered.ToByte())`。

> 🔧修正初稿：`hashMessage` 的 nonce 进**第一轮**（`SHA256(K ‖ SHA256(ts‖acct‖method‖params‖nonce8))`），与 JS 一致。steemutil `auth.go` 的代码本身正确（仅注释写错，U3 会修正）。直接调 `rpc.hashMessage` 即正确，勿按文字描述手写。

> 🔧补充说明（已更新）：steemgosdk v0.0.23 已新增**验证端**封装 `API.VerifySignedRequest`（G1），conveyor 直接调用即可。其底层是 steemutil `rpc.Validate` + `rpc.VerifySignedRpc`。

### 4.2 Drafts（blob store）
`list/save/remove`，key=`${name}_${account}_drafts.json`；缺 uuid 用 `google/uuid` v4；remove 找不到抛 `JsonRpcError(100,...)`。

### 4.3 Feature Flags（🔧正确 53 位 MT19937）
- 全局概率 `${name}_feature-flags.json`；每用户覆盖 `${name}_${account}_feature-flags.json`。
- `getFlag`：用户覆盖优先；否则 `flagProbability < probabilities[flag]`。
- 🔧**逐位移植（修正致命错误）**：初稿 `genrandUint32()/2^32` 错误，会导致现网 flag 翻转。random-js v1 的 `real(0,1)` 消费**两次**引擎输出拼 53 位除以 2^53：
  ```
  hash = SHA256(account + flag)
  seeds = [ readInt32LE(hash, 4*i) for i in 0..7 ]
  mt.seed(0x012bd6aa); mt.seedWithArray(seeds)
  a = mt.next() & 0x1fffff
  b = mt.next() >>> 0
  return (float64(a)*4294967296.0 + float64(b)) / 9007199254740992.0
  ```
  flag 名 `/^[a-z_]+$/`。
- 🔧先 `yarn install` 落盘 `random-js@1.0.8` 的 `lib/random.js` 作 golden 参照（当前 node_modules 为空），记 checksum。
- `setProbability` 仅 admin；==0 删除；[0,1]。

### 4.4 User Data（GORM User）
`get/set_user_data`（本人或 admin）、`is_email/phone_registered`（仅 admin）。User：account PK、email unique、phone unique。ValidationError→400；get 未找到 404。

### 4.5 Tags（GORM Tag/UserTag）
6 方法仅 admin。`assign_tag` 重复幂等跳过；FK 失败→420；`unassign_tag` 软删除；`get_users_by_tags` 取交集；`get_tags_for_user` audit 返全部否则返活跃 tag。tag 名 `/^[a-z0-9_]+$/`。

### 4.6 Prices（steemgosdk + steemutil int64）
并行 `get_order_book[1]`、`get_feed_history`、`GetDynamicGlobalProperties()`。🔧价格算术**不复刻 dsteem 的 float64**——直接调用 steemutil v0.0.25 的 `protocol/api.ComputePrices`（`Asset{Amount int64, Symbol}`，内部用 `math/big.Int` 复刻 steemd C++ 的 128 位交叉乘除）。这比 TS 的 float64 更精确，是与共识语义对齐的正确实现。
> ⚠️**注意**：Go 版 `get_prices` 输出的 `steem_sbd/steem_usd/steem_vest` 数值可能与 TS 版（dsteem float64）在末尾小数位有细微差异。schema 中三字段均为 `type: number`，差异属"更精确但非 bit-identical"，通常可接受；若 condenser 有严格相等断言需留意。

### 4.7 Summarizer（HTTP+LRU）
解析 URL→黑名单→LRU(10000,1h)→抓取(2s)→go-readability+goquery。失败→400。

### 4.8 User Search
CachingClient 6 路并行+游标分页(1000)+转账计数；Trie 并行 a-z 加载+定时刷新；autocomplete 每类≤10；GDPR 返 `[]`。链查询可复用 steemgosdk G2 封装。

### 4.9 健康检查
`GET /`、`/.well-known/healthcheck.json` 返 `{ok,version,date}`。

## 5. 配置兼容
viper 加载 default/production/test.toml + 环境变量；🔧修正拼写 `accounts_refresh_intercal`→`accounts_refresh_interval`。

## 6. 🔧兼容性清单（验收必过）
1. ✅ `hashMessage` nonce 在第一轮（直接调 steemutil，U3 修正注释）
2. ✅ 🔧 签名格式**无需转换**：btcec 压缩签名 == dsteem 格式（字节级一致，详见顶部勘误）
3. ✅ 🔧 复用 steemutil RecoverPublicKeyFromSignature + `VerifySignedRpc`
4. ✅ 🔧 复用 PublicKey.ToStr/FromStr
5. ✅ 仅单一 posting key + threshold
6. ✅ 60s 时效
7. ✅ 🔧 MT19937：init(0x012bd6aa)+init_by_array+8 LE 种+53 位 real
8. ✅ JSON-RPC 错误码 -32xxx + 400/401/404/420
9. ✅ batch 并发 + notification 过滤
10. ✅ GORM 表结构含 unique/软删除/FK
11. ✅ 🔧 Price 用 steemutil int64/math.big（`ComputePrices`），复刻 steemd 共识语义；⚠️与 TS float64 末位可能有细微差异

## 7. 里程碑
M0 脚手架 → M1 鉴权（依赖 steemutil v0.0.24 `VerifySignedRpc`、steemgosdk v0.0.23 G1 `VerifySignedRequest`，含签名 round-trip golden test）→ M2 存储+DB → M3 drafts/feature-flags(53位MT19937+random-js golden)/user-data/tags → M4 区块链+prices（依赖 steemutil v0.0.25 `ComputePrices`、steemgosdk G2）→ M5 user-search（依赖 steemgosdk G2）→ M6 summarizer → M7 测试+Docker。

## 8. 测试策略
🔧先 `yarn install` 落盘 random-js golden；单元(MT19937 对账、真实签名→Go 恢复 round-trip、WIF 编解码、GORM、黑名单)；集成(mock `api.Call` 用 `test/steemd_responses/` fixture)；参考现有 24 个 mocha 用例移植。

## 9. 交付物
- `next` 孤儿分支的 `REWRITE_PLAN.md`（本文档）。
- 后续在 `next` 分支按里程碑落地 Go 代码、go.mod、Dockerfile、Makefile。
