# -*- coding: utf-8 -*-
"""
SCM-WF03: 安全校核 - 工作流 03
Demo 实现：短路电流校核
"""

import time
from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext


@AlgorithmRegistry.register
class SCMWF03(BaseAlgorithm):
    """安全校核工作流03 - 短路电流"""

    @property
    def meta_info(self):
        return {
            "code": "SCM-WF03",
            "name": "安全校核 - 短路电流",
            "description": "短路电流计算与校核",
            "resource_type": "CPU",
        }

    def execute(self, ctx: AlgorithmContext) -> dict:
        ctx.log(20, f"[SCM-WF03] Task started: {ctx.task_id}")
        ctx.report_progress(15, "Collecting bus data...")
        time.sleep(0.5)

        ctx.report_progress(50, "Calculating short-circuit currents...")
        time.sleep(1)

        ctx.report_progress(85, "Checking breaker ratings...")
        time.sleep(0.5)

        result = {
            "buses_checked": 85,
            "max_sc_current_ka": 42.5,
            "over_rating_buses": [],
            "demo": True,
        }

        ctx.report_progress(100, "Completed")
        return result
