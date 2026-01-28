import time

from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext
# 此处固定使用上述导入,禁止修改

@AlgorithmRegistry.register
# 此处必须使用注册装饰器,否则插件不会被加载
class M02SafetyCheck(BaseAlgorithm):
    """
    样例算法
    新建类必须继承 BaseAlgorithm 并实现其抽象方法: meta_info 与 execute
    """

    @property
    def meta_info(self):
        return {
            "code": "DEMO01_WF01",
            "name": "样例",
            "description": "基于GNN初筛与潮流计算复核",
            "resource_type": "GPU",
        }
    # 此处为示例元信息,将被服务注册时使用,请根据实际情况修改code/name/description/resource_type字段即可
    # code 为算法唯一标识符,请勿重复
    # name 为算法名称,可读性描述算法功能
    # description 为算法描述,简要说明算法功能与用途
    # resource_type 可选值：CPU / GPU 用于区分算法对计算资源的需求  

    def execute(self, ctx: AlgorithmContext) -> dict:
        """
        核心业务逻辑入口
        此处为该插件算法的主要入口,服务在调度该算法时会调用此方法来执行业务逻辑.

        参数:
          - ctx: AlgorithmContext 上下文对象,包含任务ID、参数、数据等信息.

        返回:
          - dict: 算法执行结果,必须为字典类型,服务会将其序列化为JSON进行上报.

        说明:
          - 通过 `ctx.task_id` 获取当前任务ID,可用于日志记录和结果关联.
          - 通过 `ctx.params` 获取可选输入参数.
          - 通过 `ctx.data` 获取服务层预加载的数据对象,类型通常为 `pandas.DataFrame` .
          - 通过 `ctx.log(level, message)` 记录日志.
          - 通过 `ctx.report_progress(percentage, message)` 上报进度.
          - 返回结果必须为字典类型,禁止返回其他类型（如字符串、列表等）.

        例外:
          - 若发生不可恢复的错误,可抛出异常,服务会捕获并上报错误信息.

        """

        limit = ctx.params.get("load_limit", 0.8) # 从参数中获取负载阈值,默认为0.8

        data_info = "plugin-loaded data" if ctx.data is not None else "(no data)" # 示例获取数据并记录日志

        ctx.log(20, f"Loading data from: {data_info}, origin data: {ctx.data}") # 记录日志
        ctx.report_progress(10, "Loading Snapshot...") # 上报进度
        time.sleep(1)

        ctx.report_progress(30, "AI Inference (GNN)...") # 上报进度
        pred_load = 0.85

        result = {"is_safe": True, "violations": []} # 初始化结果字典

        if pred_load > limit:

            ctx.log(30, f"High load ({pred_load}) detected! Starting mechanism check.") # 记录日志
            ctx.report_progress(60, "Running Power Flow Verification...") # 上报进度
            time.sleep(2)

            result["is_safe"] = False
            result["violations"] = ["Line-A", "Transformer-B"]

        return result # 返回结果字典
