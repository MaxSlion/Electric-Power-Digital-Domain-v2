# -*- coding: utf-8 -*-
"""
SCM-WF02: 安全校核 - 工作流 02
Demo 实现：N-1 安全分析
"""

import time
from core.framework import BaseAlgorithm, AlgorithmRegistry, AlgorithmContext


@AlgorithmRegistry.register
class SCMWF02(BaseAlgorithm):
    """安全校核工作流02 - N-1分析"""

    @property
    def meta_info(self):
        return {
            "code": "SCM-WF02",
            "name": "安全校核 - N-1分析",
            "description": "N-1安全分析与评估",
            "resource_type": "CPU",
        }

    def execute(self, ctx: AlgorithmContext) -> dict:
        ctx.log(20, f"[SCM-WF02] Task started: {ctx.task_id}")
        ctx.report_progress(10, "Loading contingencies...")
        time.sleep(0.5)

        ctx.report_progress(40, "Running N-1 analysis...")
        time.sleep(1.5)

        ctx.report_progress(70, "Evaluating results...")
        time.sleep(0.5)

        result = {
            "contingencies_checked": 150,
            "violations_found": 2,
            "critical_lines": ["Line-A", "Line-B"],
            "demo": True,
        }

        ctx.report_progress(100, "Completed")
        return result
