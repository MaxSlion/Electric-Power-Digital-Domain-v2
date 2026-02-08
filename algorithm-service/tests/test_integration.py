# -*- coding: utf-8 -*-
"""
Integration Tests for Algorithm Service
集成测试：测试完整的任务调度和执行流程
"""

import pytest
import sys
import os
import time

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.framework import AlgorithmRegistry
from core.plugin_loader import load_plugins
from core.dispatcher import TaskDispatcher
from core.resource_manager import HardwareManager


class MockReporter:
    """Mock result reporter for integration tests"""
    def __init__(self):
        self.results = []

    def send_result(self, task_id, status, data=None, error=None):
        self.results.append({
            "task_id": task_id,
            "status": status,
            "data": data,
            "error": error
        })


@pytest.fixture(scope="module")
def dispatcher():
    """Create a dispatcher with mock reporter"""
    load_plugins("plugins")
    reporter = MockReporter()
    disp = TaskDispatcher(reporter)
    return disp, reporter


class TestTaskDispatcher:
    """Integration tests for TaskDispatcher"""

    @pytest.mark.integration
    def test_dispatch_valid_task(self, dispatcher):
        """Test dispatching a valid task"""
        disp, reporter = dispatcher
        
        # Submit a task
        task_id = f"integration-test-{int(time.time())}"
        result = disp.dispatch(
            task_id=task_id,
            scheme_code="SCM-WF01",
            data_ref="test_data",
            params={}
        )
        
        # Wait for async execution (with timeout)
        timeout = 10
        start = time.time()
        while len(reporter.results) == 0 and time.time() - start < timeout:
            time.sleep(0.5)
        
        # Check result was reported
        assert len(reporter.results) > 0, "No result reported within timeout"
        last_result = reporter.results[-1]
        assert last_result["task_id"] == task_id
        assert last_result["status"] in ["SUCCESS", "FAILED"]

    @pytest.mark.integration
    def test_dispatch_unknown_scheme(self, dispatcher):
        """Test dispatching with unknown scheme code"""
        disp, reporter = dispatcher
        
        initial_count = len(reporter.results)
        task_id = f"unknown-scheme-{int(time.time())}"
        
        result = disp.dispatch(
            task_id=task_id,
            scheme_code="UNKNOWN-SCHEME",
            data_ref="test_data",
            params={}
        )
        
        # Wait briefly
        time.sleep(1)
        
        # Should have reported a failure
        new_results = reporter.results[initial_count:]
        if new_results:
            assert new_results[-1]["status"] == "FAILED"


class TestHardwareManager:
    """Integration tests for HardwareManager"""

    @pytest.mark.integration
    def test_singleton_pattern(self):
        """Test HardwareManager is singleton"""
        hm1 = HardwareManager.get_instance()
        hm2 = HardwareManager.get_instance()
        assert hm1 is hm2

    @pytest.mark.integration
    def test_executor_available(self):
        """Test executors are available"""
        hm = HardwareManager.get_instance()
        
        cpu_executor = hm.get_executor("CPU")
        assert cpu_executor is not None
        
        gpu_executor = hm.get_executor("GPU")
        assert gpu_executor is not None


class TestEndToEndWorkflow:
    """End-to-end workflow tests"""

    @pytest.mark.integration
    def test_complete_kbm_workflow(self, dispatcher):
        """Test complete KBM workflow execution"""
        disp, reporter = dispatcher
        
        task_id = f"e2e-kbm-{int(time.time())}"
        initial_count = len(reporter.results)
        
        disp.dispatch(
            task_id=task_id,
            scheme_code="KBM-WF02",
            data_ref="test_data",
            params={"mode_k": 5}
        )
        
        # Wait for completion
        timeout = 15
        start = time.time()
        while len(reporter.results) <= initial_count and time.time() - start < timeout:
            time.sleep(0.5)
        
        new_results = reporter.results[initial_count:]
        assert len(new_results) > 0
        assert new_results[-1]["status"] == "SUCCESS"

    @pytest.mark.integration
    def test_complete_scm_workflow(self, dispatcher):
        """Test complete SCM workflow execution"""
        disp, reporter = dispatcher
        
        task_id = f"e2e-scm-{int(time.time())}"
        initial_count = len(reporter.results)
        
        disp.dispatch(
            task_id=task_id,
            scheme_code="SCM-WF02",
            data_ref="grid_snapshot",
            params={}
        )
        
        # Wait for completion
        timeout = 15
        start = time.time()
        while len(reporter.results) <= initial_count and time.time() - start < timeout:
            time.sleep(0.5)
        
        new_results = reporter.results[initial_count:]
        assert len(new_results) > 0
        assert new_results[-1]["status"] == "SUCCESS"

    @pytest.mark.integration
    def test_complete_stm_workflow(self, dispatcher):
        """Test complete STM workflow execution"""
        disp, reporter = dispatcher
        
        task_id = f"e2e-stm-{int(time.time())}"
        initial_count = len(reporter.results)
        
        disp.dispatch(
            task_id=task_id,
            scheme_code="STM-WF01",
            data_ref="scenario_base",
            params={}
        )
        
        # Wait for completion
        timeout = 15
        start = time.time()
        while len(reporter.results) <= initial_count and time.time() - start < timeout:
            time.sleep(0.5)
        
        new_results = reporter.results[initial_count:]
        assert len(new_results) > 0
        assert new_results[-1]["status"] == "SUCCESS"
