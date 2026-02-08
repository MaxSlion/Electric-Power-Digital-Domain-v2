# -*- coding: utf-8 -*-
"""
Unit Tests for Core Framework Components
功能测试：测试核心框架组件的基本功能
"""

import pytest
import sys
import os
import logging

# Add project root to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.framework import BaseAlgorithm, AlgorithmContext, AlgorithmRegistry


class MockReporter:
    """Mock reporter for testing"""
    def update(self, task_id, percentage, message):
        pass


class TestAlgorithmContext:
    """Test AlgorithmContext functionality"""

    @pytest.mark.unit
    def test_context_initialization(self):
        """Test context can be created with required parameters"""
        logger = logging.getLogger("test")
        reporter = MockReporter()
        ctx = AlgorithmContext(
            task_id="test-task-001",
            params={"threshold": 0.8},
            reporter_stub=reporter,
            logger=logger,
            data={"source": "test_data"}
        )
        assert ctx.task_id == "test-task-001"
        assert ctx.params["threshold"] == 0.8

    @pytest.mark.unit
    def test_context_params_default(self):
        """Test context handles missing params gracefully"""
        logger = logging.getLogger("test")
        reporter = MockReporter()
        ctx = AlgorithmContext(
            task_id="test-task-002",
            params={},
            reporter_stub=reporter,
            logger=logger
        )
        assert ctx.params.get("missing_key", "default") == "default"

    @pytest.mark.unit
    def test_context_log_with_logger(self):
        """Test logging works with a logger"""
        logger = logging.getLogger("test")
        reporter = MockReporter()
        ctx = AlgorithmContext(
            task_id="test-task-003",
            params={},
            reporter_stub=reporter,
            logger=logger
        )
        # Should not raise
        ctx.log(20, "Test message")


class TestAlgorithmRegistry:
    """Test AlgorithmRegistry functionality"""

    @pytest.mark.unit
    def test_registry_discover_plugins(self):
        """Test registry can discover registered plugins"""
        # Import to trigger registration
        from core.plugin_loader import load_plugins
        load_plugins("plugins")
        
        registry_items = AlgorithmRegistry.get_all_schemes()
        assert len(registry_items) > 0, "Should have at least one registered plugin"

    @pytest.mark.unit
    def test_registry_get_by_code(self):
        """Test getting algorithm by code"""
        from core.plugin_loader import load_plugins
        load_plugins("plugins")
        
        # Try to get a known plugin
        algo = AlgorithmRegistry.get_algorithm("KBM-WF02")
        if algo:
            assert algo.meta_info["code"] == "KBM-WF02"

    @pytest.mark.unit
    def test_registry_list_codes(self):
        """Test listing all registered codes"""
        from core.plugin_loader import load_plugins
        load_plugins("plugins")
        
        codes = list(AlgorithmRegistry._registry.keys())
        assert isinstance(codes, list)
        # Check for expected modules
        expected_prefixes = ["KBM-", "SCM-", "STM-"]
        found_prefixes = set()
        for code in codes:
            for prefix in expected_prefixes:
                if code.startswith(prefix):
                    found_prefixes.add(prefix)
        assert len(found_prefixes) >= 2, f"Should have plugins from at least 2 modules, found: {codes}"


class TestBaseAlgorithm:
    """Test BaseAlgorithm abstract class"""

    @pytest.mark.unit
    def test_cannot_instantiate_base(self):
        """Test BaseAlgorithm cannot be instantiated directly"""
        with pytest.raises(TypeError):
            BaseAlgorithm()

    @pytest.mark.unit
    def test_subclass_must_implement_meta_info(self):
        """Test subclass must implement meta_info"""
        class IncompleteAlgo(BaseAlgorithm):
            def execute(self, ctx):
                return {}
        
        with pytest.raises(TypeError):
            IncompleteAlgo()

    @pytest.mark.unit
    def test_subclass_must_implement_execute(self):
        """Test subclass must implement execute"""
        class IncompleteAlgo(BaseAlgorithm):
            @property
            def meta_info(self):
                return {"code": "TEST", "name": "Test"}
        
        with pytest.raises(TypeError):
            IncompleteAlgo()
