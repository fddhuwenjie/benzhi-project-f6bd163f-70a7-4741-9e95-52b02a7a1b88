# BENZHI_README

## 项目说明
- 项目：benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88
- 项目用途：完整实现校园实验楼化学品泄漏演练就绪治理服务，包含情景冻结、开始前核验、首演评价、偏差整改、定向复演、独立复核、不可变档案和摘要校验，并由 Go 直接提供浏览器工作台与同源 JSON API。
- Go 工具链：`golang:1.23`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：化学品泄漏演练就绪门禁
- 项目介绍：面向校园实验楼安全团队的单流程演练治理应用，将化学品泄漏应急演练从情景冻结、开始前核验、现场记录、偏差整改、定向复演推进到独立签署与就绪档案冻结。项目采用 standard 档位，目标约 2470 行真实生产 Go 代码和 22 个生产 Go 文件；Go 服务直接提供无需 Node 构建链的原生浏览器工作台与同源 JSON 端点。
- 项目概述：面向校园实验楼安全团队的单流程演练治理应用，将化学品泄漏应急演练从情景冻结、开始前核验、现场记录、偏差整改、定向复演推进到独立签署与就绪档案冻结。项目采用 standard 档位，目标约 2470 行真实生产 Go 代码和 22 个生产 Go 文件；Go 服务直接提供无需 Node 构建链的原生浏览器工作台与同源 JSON 端点。
- 核心工作流：安全协调员创建演练案件并冻结泄漏情景与通过阈值，完成开始前核验后启动演练；观察员按时间线记录响应动作和偏差，协调员为未通过项制定整改并发起定向复演；全部失败项复验合格后由未参与执行的安全复核员批准或拒绝，批准时冻结可校验的就绪档案并使案件进入只读终态。
- 对外接口：由 Go 服务托管的原生单页浏览器工作台，包含案件状态带、情景与阈值编辑区、开始前核验表、演练计时事件流、偏差整改队列、定向复演面板、独立复核区和就绪档案校验视图；页面仅调用同源 JSON 端点完成主流程，不提供独立 CLI 产品界面，也不引入 Node 构建链。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88-arm64 linux/arm64

docker run -it benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
