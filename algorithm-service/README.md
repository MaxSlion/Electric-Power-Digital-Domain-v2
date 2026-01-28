# Algorithm Service

Python 算法服务底座，基于“微内核 + 插件化”架构实现硬件自适应、任务调度、进度回传与结果上报。

## 目录结构

```
algorithm-service/
  core/                 # 平台核心：资源池、注册表、调度器
  data/                 # 任务数据与状态存储
  infrastructure/       # 日志、进度、gRPC 骨架
  logs/                 # 日志文件夹
  plugins/              # 业务算法插件
  proto/                # gRPC 接口定义
  main.py               # 服务入口
```

## 快速开始

1. 安装依赖：
   - 建议使用虚拟环境。
   - 安装 `requirements.txt`。

2. 生成 gRPC 代码：
   - 使用 `grpcio-tools` 将 [proto/algorithm.proto](algorithm-service/proto/algorithm.proto) 编译为 `*_pb2.py` 文件。

3. 运行服务：
   - 直接运行 [main.py](main.py) 会启动本地调度器并加载插件。

## 插件开发规范

1. 继承 `BaseAlgorithm`。
2. 实现 `meta_info` 与 `execute()`。
3. 使用 `@AlgorithmRegistry.register` 装饰器注册。
4. 插件目录支持多级结构（例如 `plugins/M01/WF01/xxx.py`），服务会递归扫描并在返回中补充 `model = "M01-WF01"`。
5. 插件输入仅使用 `ctx.data` 与 `ctx.params`，禁止读取 `data_ref`。

参考实现：
- [plugins/M01/WF01/m02_safety_check.py](plugins/M01/WF01/m02_safety_check.py)

## 数据接入（服务层解析）

`SubmitTask` 的 `data_ref` 由服务层统一解析并加载为 `pandas.DataFrame`，然后注入 `AlgorithmContext.data`。

支持：
- 文件：本地路径或 `file://`
   - 示例：`D:/data/realtime.csv`
   - 示例：`file:///D:/data/realtime.csv`
- MySQL：
   - `mysql://user:pass@host:3306/db?table=table_name`
   - `mysql://user:pass@host:3306/db?query=SELECT%20*%20FROM%20table_name`
- Redis：
   - `redis://host:6379/0?key=some_key&type=string|hash|list`

> 约束：服务层必须输出 `pandas.DataFrame`，否则任务失败。

## 服务发现返回字段

`GetAvailableSchemes` 现在返回 `code`、`name`、`resource_type`，并新增 `model` 与 `class_name` 字段。

## 任务状态查询

`ListTasks` 接口可查询当前服务内记录的任务状态：
- `status`: QUEUED / RUNNING / SUCCESS / FAILED
- `percentage`, `message`, `updated_at`, `error_message`

`WatchTaskProgress` 是 gRPC Server Streaming：
- 前端应在 `SubmitTask` 后立即建立流并保持连接以获得实时进度。
- 任务已完成后再次调用会返回最近一次进度并结束（用于回放/补偿）。

任务状态会落盘到 SQLite：`algorithm-service/data/tasks.db`（主进程写入）。

`GetTaskStatus(task_id)` 用于查询单个任务的状态记录。
