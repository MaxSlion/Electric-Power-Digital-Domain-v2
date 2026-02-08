# -*- coding: utf-8 -*-
"""
Unit Tests for Plugin Execution
功能测试：测试各个插件的执行功能
"""

import pytest
import sys
import os
import logging

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.framework import AlgorithmContext, AlgorithmRegistry
from core.plugin_loader import load_plugins


class MockReporter:
    """Mock reporter for testing - implements update() method"""
    def __init__(self):
        self.progress_calls = []

    def update(self, task_id, percentage, message):
        self.progress_calls.append((task_id, percentage, message))


@pytest.fixture(scope="module")
def loaded_plugins():
    """Fixture to load plugins once per module"""
    load_plugins("plugins")
    return AlgorithmRegistry._registry


def create_test_context(task_id, params=None):
    """Helper to create a test context with correct signature"""
    reporter = MockReporter()
    logger = logging.getLogger("test")
    ctx = AlgorithmContext(
        task_id=task_id,
        params=params or {},
        reporter_stub=reporter,
        logger=logger,
        data={"source": "test_data"}
    )
    return ctx, reporter


class TestKBMPlugins:
    """Test KBM module plugins"""

    @pytest.mark.unit
    def test_kbm_wf02_execution(self, loaded_plugins):
        """Test KBM-WF02 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("KBM-WF02")
        if algo is None:
            pytest.skip("KBM-WF02 not registered")

        ctx, reporter = create_test_context("test-kbm-wf02")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)
        assert "demo" in result or len(result) > 0

    @pytest.mark.unit
    def test_kbm_wf03_execution(self, loaded_plugins):
        """Test KBM-WF03 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("KBM-WF03")
        if algo is None:
            pytest.skip("KBM-WF03 not registered")

        ctx, reporter = create_test_context("test-kbm-wf03")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)


class TestSCMPlugins:
    """Test SCM module plugins"""

    @pytest.mark.unit
    def test_scm_wf01_execution(self, loaded_plugins):
        """Test SCM-WF01 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("SCM-WF01")
        if algo is None:
            pytest.skip("SCM-WF01 not registered")

        ctx, reporter = create_test_context("test-scm-wf01")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)

    @pytest.mark.unit
    def test_scm_wf02_execution(self, loaded_plugins):
        """Test SCM-WF02 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("SCM-WF02")
        if algo is None:
            pytest.skip("SCM-WF02 not registered")

        ctx, reporter = create_test_context("test-scm-wf02")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)

    @pytest.mark.unit
    def test_scm_wf03_execution(self, loaded_plugins):
        """Test SCM-WF03 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("SCM-WF03")
        if algo is None:
            pytest.skip("SCM-WF03 not registered")

        ctx, reporter = create_test_context("test-scm-wf03")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)


class TestSTMPlugins:
    """Test STM module plugins"""

    @pytest.mark.unit
    def test_stm_wf01_execution(self, loaded_plugins):
        """Test STM-WF01 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("STM-WF01")
        if algo is None:
            pytest.skip("STM-WF01 not registered")

        ctx, reporter = create_test_context("test-stm-wf01")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)

    @pytest.mark.unit
    def test_stm_wf02_execution(self, loaded_plugins):
        """Test STM-WF02 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("STM-WF02")
        if algo is None:
            pytest.skip("STM-WF02 not registered")

        ctx, reporter = create_test_context("test-stm-wf02")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)

    @pytest.mark.unit
    def test_stm_wf03_execution(self, loaded_plugins):
        """Test STM-WF03 plugin executes successfully"""
        algo = AlgorithmRegistry.get_algorithm("STM-WF03")
        if algo is None:
            pytest.skip("STM-WF03 not registered")

        ctx, reporter = create_test_context("test-stm-wf03")
        result = algo.execute(ctx)
        
        assert isinstance(result, dict)


class TestPluginMetaInfo:
    """Test plugin meta_info properties"""

    @pytest.mark.unit
    def test_all_plugins_have_required_meta_fields(self, loaded_plugins):
        """Test all plugins have required meta_info fields"""
        required_fields = ["code", "name", "resource_type"]

        for code, algo in loaded_plugins.items():
            meta = algo.meta_info
            for field in required_fields:
                assert field in meta, f"Plugin {code} missing required field: {field}"

    @pytest.mark.unit
    def test_resource_type_valid(self, loaded_plugins):
        """Test resource_type is either CPU or GPU"""
        valid_types = {"CPU", "GPU"}

        for code, algo in loaded_plugins.items():
            resource_type = algo.meta_info.get("resource_type", "")
            assert resource_type in valid_types, f"Plugin {code} has invalid resource_type: {resource_type}"
