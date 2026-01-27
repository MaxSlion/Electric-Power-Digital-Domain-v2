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

参考实现：
- [plugins/M01/WF01/m02_safety_check.py](plugins/M01/WF01/m02_safety_check.py)

## 服务发现返回字段

`GetAvailableSchemes` 现在返回 `code`、`name`、`resource_type`，并新增 `model` 与 `class_name` 字段。

## 任务状态查询

`ListTasks` 接口可查询当前服务内记录的任务状态：
- `status`: QUEUED / RUNNING / SUCCESS / FAILED
- `percentage`, `message`, `updated_at`

`WatchTaskProgress` 在任务完成后再次调用会立即返回最近一次进度并结束。

任务状态会落盘到 SQLite：`algorithm-service/data/tasks.db`。

`GetTaskStatus(task_id)` 用于查询单个任务的状态记录。
