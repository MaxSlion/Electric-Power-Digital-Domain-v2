# -*- coding: utf-8 -*-
"""
SCM-WF01: 安全校核 - 工作流 01
Demo 实现：GNN 初筛 + 潮流计算
"""

import time
from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext


@AlgorithmRegistry.register
class SCMWF01(BaseAlgorithm):
    """安全校核工作流01 - Demo实现"""

    @property
    def meta_info(self):
        return {
            "code": "SCM-WF01",
            "name": "安全校核 - 初筛与复核",
            "description": "基于GNN初筛与潮流计算复核的安全校核工作流",
            "resource_type": "GPU",
        }

    def execute(self, ctx: AlgorithmContext) -> dict:
        ctx.log(20, f"[SCM-WF01] Task started: {ctx.task_id}")
        ctx.report_progress(5, "Initializing safety check...")
        time.sleep(0.5)

        ctx.report_progress(20, "Loading grid snapshot...")
        time.sleep(0.5)

        ctx.report_progress(40, "Running GNN inference...")
        time.sleep(1)

        ctx.report_progress(60, "Power flow verification...")
        time.sleep(1)

        ctx.report_progress(80, "Generating report...")
        time.sleep(0.5)

        result = {
            "is_safe": True,
            "violations": [],
            "checked_lines": 120,
            "checked_buses": 85,
            "demo": True,
        }

        ctx.report_progress(100, "Completed")
        ctx.log(20, f"[SCM-WF01] Task completed: {ctx.task_id}")
        return result
