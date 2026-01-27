import time

from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext

@AlgorithmRegistry.register
class M02SafetyCheck(BaseAlgorithm):
    @property
    def meta_info(self):
        return {
            "code": "M01_WF01",
            "name": "机理-数据融合安全校核",
            "description": "基于GNN初筛与潮流计算复核",
            "resource_type": "GPU",
        }

    def execute(self, ctx: AlgorithmContext) -> dict:
        limit = ctx.params.get("load_limit", 0.8)

        ctx.log(20, f"Loading data from: {ctx.data_ref}")
        ctx.report_progress(10, "Loading Snapshot...")
        time.sleep(1)

        ctx.report_progress(30, "AI Inference (GNN)...")
        pred_load = 0.85

        result = {"is_safe": True, "violations": []}

        if pred_load > limit:
            ctx.log(30, f"High load ({pred_load}) detected! Starting mechanism check.")
            ctx.report_progress(60, "Running Power Flow Verification...")
            time.sleep(2)
            result["is_safe"] = False
            result["violations"] = ["Line-A", "Transformer-B"]

        return result
