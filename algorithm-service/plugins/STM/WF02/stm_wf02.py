# -*- coding: utf-8 -*-
"""
STM-WF02: 数字孪生推演 - 工作流 02
Demo 实现：决策优化求解
"""

import time
from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext


@AlgorithmRegistry.register
class STMWF02(BaseAlgorithm):
    """数字孪生工作流02 - 决策求解"""

    @property
    def meta_info(self):
        return {
            "code": "STM-WF02",
            "name": "数字孪生 - 决策求解",
            "description": "基于MILP的运行方式决策优化",
            "resource_type": "CPU",
        }

    def execute(self, ctx: AlgorithmContext) -> dict:
        ctx.log(20, f"[STM-WF02] Task started: {ctx.task_id}")
        ctx.report_progress(10, "Building optimization model...")
        time.sleep(0.5)

        ctx.report_progress(40, "Solving MILP...")
        time.sleep(1.5)

        ctx.report_progress(75, "Extracting solution...")
        time.sleep(0.5)

        result = {
            "optimal_cost": 125000.50,
            "generation_schedule": {"G1": 500, "G2": 350, "G3": 400},
            "solver_status": "optimal",
            "gap": 0.001,
            "demo": True,
        }

        ctx.report_progress(100, "Completed")
        return result
