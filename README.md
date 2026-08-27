# 化学品泄漏演练就绪门禁

本项目面向校园实验楼安全团队，提供一条完整、可审计的化学品泄漏应急演练治理流程。安全协调员创建案件并冻结泄漏情景与量化阈值，完成开始前核验后由观察员记录首演；未通过的观测点进入整改与定向复演，最终由未参与执行的独立安全复核员批准或拒绝。批准后生成不可变的就绪档案和可重新计算的 SHA-256 摘要。

服务由 Go 直接托管原生浏览器工作台和同源 JSON API，不需要 Node.js 构建链。案件快照、追加事件日志和幂等结果默认保存在 `data/`。事件日志包含长度、校验和、单调序号和前序摘要；启动时完整性校验失败会阻止服务启动。

## 构建

```bash
go build ./cmd/server
```

## 运行

```bash
go run ./cmd/server -addr=127.0.0.1:19081
```

浏览器打开 `http://127.0.0.1:19081/`。默认监听地址为 `127.0.0.1:19081`，不会绑定到 `0.0.0.0`。可以通过 `-addr=127.0.0.1:<port>` 指定回环地址，也可以将 `PORT` 设置为端口号，使服务监听 `127.0.0.1:<PORT>`。使用 `-data=<path>` 可以更改持久化目录。

## 测试

运行全部单元测试与 HTTP 集成测试：

```bash
go test ./...
```

运行可自行结束的真实 HTTP 全流程自检：

```bash
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
```

自检使用临时数据目录，在指定回环地址启动真实服务，依次完成首演失败、偏差整改、定向复演通过、独立批准和档案摘要校验，然后主动关闭服务并删除临时数据。

## 主要接口

- `GET /`：浏览器工作台。
- `GET /healthz`：进程健康检查。
- `GET /readyz`：事件日志完整性就绪检查。
- `/api/cases`：案件查询与创建入口。
- `/api/cases/{caseID}/...`：情景冻结、核验、场次记录、整改、复演、复核和档案校验入口。

情景冻结前可调用 `POST /api/cases/{caseID}/baseline/precheck` 获取规范化预览、候选摘要和逐字段问题；开始前核验需提交 `valid_until`，服务会在查询和首演启动时重新判断有效期。运行中的观察员可通过 `POST /api/cases/{caseID}/sessions/corrections` 追加动作或观测更正，协调员可通过 `POST /api/cases/{caseID}/deviations/remediate-batch` 原子登记多项整改。独立复核提交六项结构化清单后，已批准案件可从 `GET /api/cases/{caseID}/dossier/download` 获取带摘要校验的 JSON 附件。

所有写请求均携带 `request_id`、`expected_revision` 和 `actor_id`。`expected_revision` 防止陈旧页面覆盖新数据，`request_id` 与请求指纹用于持久幂等重放；同一 `request_id` 不能复用于不同内容。
