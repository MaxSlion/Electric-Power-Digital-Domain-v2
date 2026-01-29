# Plugin 算法入口设计方案

本文档约定算法插件的入口规范、数据交付方式、进度/日志上报与结果输出格式，供后续插件开发对齐使用。

## 1. 设计目标

- 插件只关注业务逻辑，禁止自行解析 `data_ref` 与获取数据等任何操作。
- 数据由服务层解析并注入 `AlgorithmContext.data`。
- 插件通过 `ctx.log()` 与 `ctx.report_progress()` 统一上报。
- 插件返回 `dict`，由服务层统一序列化并上报。

## 2. 生命周期与职责边界

### 服务层职责

- gRPC `SubmitTask` 收到请求后：
  - 解析 `data_ref` 并读取数据（文件 / MySQL / Redis）。
  - 构建 `AlgorithmContext`，注入 `data`。
  - 调度线程/进程池执行插件。
  - 统一异常捕获与结果上报。

### 插件职责

- 只使用 `ctx.data` 获取数据。
- 仅使用 `ctx.params` 获取业务参数。
- 按阶段上报进度、日志。
- 返回结构化结果 `dict`。

## 3. 输入接口约定

### 3.1 gRPC SubmitTask 接口字段：

- `scheme_code`：插件唯一标识，对应 `meta_info.code`。
- `data_ref`：数据来源引用（必须）。
- `params_json`：业务参数（仅与算法逻辑相关）。

### 3.2 data_ref 解析规范

支持类型：
- **文件**：本地路径或 `file://`
  - 示例：`D:/data/realtime.csv`
  - 示例：`file:///D:/data/realtime.csv`
- **MySQL**：
  - `mysql://user:pass@host:3306/db?table=table_name`
  - `mysql://user:pass@host:3306/db?query=SELECT%20*%20FROM%20table_name`
- **Redis**：
  - `redis://host:6379/0?key=some_key&type=string|hash|list`

> 解析结果会自动注入 `ctx.data`。若解析失败，任务会失败并上报错误。

## 4. AlgorithmContext 规范

插件入口 `execute(self, ctx)` 中可用字段：

- `ctx.task_id`：任务 ID
- `ctx.params`：业务参数（dict）
- `ctx.data`：服务层解析后的数据对象
  - 文件 / MySQL / Redis 读取结果
  - 通常为 `pandas.DataFrame` 或原始 JSON

## 5. 进度与日志规范

- `ctx.log(level, message)`
  - level 使用标准 logging 级别：`logging.INFO / WARNING / ERROR`
- `ctx.report_progress(percentage, message)`
  - percentage：0-100
  - message：阶段描述

建议进度切分：
- 0-20：数据校验/预处理
- 20-70：核心算法处理
- 70-100：收尾与结果封装

## 6. 返回结果规范

- 插件返回 `dict`。
- 必须保证 JSON 可序列化。
- 不应返回 DataFrame 对象，需转为 `list[dict]` 或其他轻量结构。

建议结构：
- `run_id`：本次执行标识
- `status`：`SUCCESS` / `FAILED`
- `payload`：核心结果
- `metrics`：可选，性能或质量指标
- `artifacts_keys`：可选，输出字段列表

## 7. 标准插件模板（参考结构）

- 文件路径示例：`plugins/M01/WF01/m01_realtime_data.py`
- 必须继承 `BaseAlgorithm`
- 必须使用 `@AlgorithmRegistry.register`

参考实现：
- [M01/WF01/m02_safety_check.py](M01/WF01/m02_safety_check.py)



在开发阶段请参考以下目录结构，开发完成后将插件放置于 `plugins/` 目录下：

```
KBM/
├─ __init__.py
└─ WF01/
  ├─ kbm_wf01.py          # 插件主入口
  ├─ data/                # 测试数据
  │  ├─ history.csv
  │  └─ realtime_frame.csv
  └─ features/            # 特征工程相关脚本
    ├─ __init__.py
    ├─ kbm_01_01_realtime_ingest.py
    ├─ kbm_01_02_history_extract.py
    ├─ kbm_01_03_standardize.py
    ├─ kbm_01_04_align.py
    ├─ kbm_01_05_mode_partition.py
    ├─ kbm_01_06_safety_targets.py
    ├─ kbm_01_07_constraint_catalog.py
    ├─ kbm_01_08_feature_extract.py
    ├─ kbm_01_09_risk_scenarios.py
    ├─ kbm_01_10_n1_standard.py
    ├─ kbm_01_11_sc_standard.py
    ├─ kbm_01_12_simulation.py
    └─ kbm_01_13_visualize.py
```

> 插件主入口在开发阶段可直接使用pandas读取测试数据，实现算法逻辑；上线时请遵循插件开发规范，从ctx.data获取数据

## 8. 禁止项

- 插件中不得直接调用 gRPC/数据库连接进行外部上报。
- 插件中不得自行创建线程/进程池。

## 9. 常见问题

### Q1: `ctx.data` 不是 DataFrame 怎么办？
A: 插件自行判断类型并转换；如需要强制 DataFrame，可显式校验后抛异常。

### Q2: 是否可以通过 `params_json` 传入数据地址？
A: 不允许。数据地址必须由 `data_ref` 传入并由服务层解析。

### Q3: 插件输出太大怎么办？
A: 将大对象落盘或抽样后返回，避免 gRPC 传输失败。
