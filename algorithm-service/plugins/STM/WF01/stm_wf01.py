# -*- coding: utf-8 -*-
"""
STM-WF01: 数字孪生推演 - 工作流 01
Demo 实现：场景模拟与推演
"""

import time
from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext


@AlgorithmRegistry.register
class STMWF01(BaseAlgorithm):
    """数字孪生工作流01 - 场景推演"""

    @property
    def meta_info(self):
        return {
            "code": "STM-WF01",
            "name": "数字孪生 - 场景推演",
            "description": "电网运行场景模拟与推演分析",
            "resource_type": "CPU",
        }

    def execute(self, ctx: AlgorithmContext) -> dict:
        ctx.log(20, f"[STM-WF01] Task started: {ctx.task_id}")
        ctx.report_progress(10, "Loading base scenario...")
        time.sleep(0.5)

        ctx.report_progress(30, "Generating variations...")
        time.sleep(0.8)

        ctx.report_progress(60, "Running simulations...")
        time.sleep(1.2)

        ctx.report_progress(85, "Aggregating results...")
        time.sleep(0.5)

        result = {
            "scenarios_simulated": 10,
            "base_load_mw": 1250.5,
            "peak_load_mw": 1450.2,
            "renewable_ratio": 0.35,
            "demo": True,
        }

        ctx.report_progress(100, "Completed")
        return result
