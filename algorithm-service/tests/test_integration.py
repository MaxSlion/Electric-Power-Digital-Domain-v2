# -*- coding: utf-8 -*-
"""
Integration Tests for Algorithm Service
集成测试：测试插件注册和执行流程
"""

import pytest
import sys
import os
import time
import logging

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.framework import AlgorithmRegistry, AlgorithmContext
from core.plugin_loader import load_plugins


class MockReporter:
    """Mock result reporter for integration tests"""
    def __init__(self):
        self.calls = []

    def update(self, task_id, percentage, message):
        self.calls.append({"task_id": task_id, "percentage": percentage, "message": message})


@pytest.fixture(scope="module")
def setup_plugins():
    """Load all plugins for integration testing"""
    load_plugins("plugins")
    return AlgorithmRegistry._registry


def create_context(task_id, params=None):
    """Create test context"""
    reporter = MockReporter()
    logger = logging.getLogger("integration")
    ctx = AlgorithmContext(
        task_id=task_id,
        params=params or {},
        reporter_stub=reporter,
        logger=logger,
        data={"source": "integration_test"}
    )
    return ctx, reporter


class TestPluginDiscovery:
    """Integration tests for plugin discovery"""

    @pytest.mark.integration
    def test_all_modules_discovered(self, setup_plugins):
        """Test all expected modules are discovered"""
        codes = list(setup_plugins.keys())
        
        # Check KBM plugins exist
        kbm_codes = [c for c in codes if c.startswith("KBM-")]
        assert len(kbm_codes) >= 2, f"Expected at least 2 KBM plugins, found: {kbm_codes}"
        
        # Check SCM plugins exist
        scm_codes = [c for c in codes if c.startswith("SCM-")]
        assert len(scm_codes) >= 1, f"Expected at least 1 SCM plugin, found: {scm_codes}"
        
        # Check STM plugins exist
        stm_codes = [c for c in codes if c.startswith("STM-")]
        assert len(stm_codes) >= 1, f"Expected at least 1 STM plugin, found: {stm_codes}"

    @pytest.mark.integration
    def test_get_all_schemes_returns_metadata(self, setup_plugins):
        """Test get_all_schemes returns proper metadata"""
        schemes = AlgorithmRegistry.get_all_schemes()
        
        assert len(schemes) > 0
        for scheme in schemes:
            assert "code" in scheme
            assert "name" in scheme
            assert "resource_type" in scheme


class TestEndToEndWorkflow:
    """End-to-end workflow execution tests"""

    @pytest.mark.integration
    def test_complete_kbm_workflow(self, setup_plugins):
        """Test complete KBM workflow execution"""
        algo = AlgorithmRegistry.get_algorithm("KBM-WF02")
        if algo is None:
            pytest.skip("KBM-WF02 not available")
        
        ctx, reporter = create_context("e2e-kbm", {"mode_k": 5})
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)
        assert len(reporter.calls) > 0, "Expected progress updates during execution"

    @pytest.mark.integration
    def test_complete_scm_workflow(self, setup_plugins):
        """Test complete SCM workflow execution"""
        algo = AlgorithmRegistry.get_algorithm("SCM-WF01")
        if algo is None:
            pytest.skip("SCM-WF01 not available")
        
        ctx, reporter = create_context("e2e-scm")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)
        assert len(reporter.calls) > 0

    @pytest.mark.integration
    def test_complete_stm_workflow(self, setup_plugins):
        """Test complete STM workflow execution"""
        algo = AlgorithmRegistry.get_algorithm("STM-WF01")
        if algo is None:
            pytest.skip("STM-WF01 not available")
        
        ctx, reporter = create_context("e2e-stm")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)
        assert len(reporter.calls) > 0


class TestProgressReporting:
    """Test progress reporting mechanism"""

    @pytest.mark.integration
    def test_progress_updates_received(self, setup_plugins):
        """Test that progress updates are properly reported"""
        algo = AlgorithmRegistry.get_algorithm("SCM-WF02")
        if algo is None:
            pytest.skip("SCM-WF02 not available")
        
        ctx, reporter = create_context("progress-test")
        algo.execute(ctx)
        
        # Verify progress calls happened
        assert len(reporter.calls) > 0
        
        # Verify progress values are reasonable
        for call in reporter.calls:
            assert 0 <= call["percentage"] <= 100
            assert isinstance(call["message"], str)
