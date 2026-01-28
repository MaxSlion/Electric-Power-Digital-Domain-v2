# -*- coding: utf-8 -*-

"""
Framework module defining the base classes and registry for algorithms.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
import logging
import os
import sys


class AlgorithmContext:
    """Context provided to algorithms during execution."""

    def __init__(self, task_id, data_ref, params, reporter_stub, logger):
        self.task_id = task_id
        self.data_ref = data_ref
        self.params = params
        self._reporter = reporter_stub
        self._logger = logger

    def log(self, level, message):
        """Log a message with the given severity level."""

        full_msg = f"[{self.task_id}] {message}"
        self._logger.log(level, full_msg)

    def report_progress(self, percentage, message):
        """Report progress of the algorithm execution."""

        self._reporter.update(self.task_id, percentage, message)


class BaseAlgorithm(ABC):
    """Abstract base class for all algorithms."""

    @property
    @abstractmethod
    def meta_info(self) -> dict:
        """Return metadata for service discovery."""

    @abstractmethod
    def execute(self, ctx: AlgorithmContext) -> dict:
        """Core business logic entrypoint."""


class AlgorithmRegistry:
    """Registry for managing available algorithms."""

    _registry = {}
    _plugin_root = None
    _plugin_meta = {}

    @classmethod
    def set_plugin_root(cls, plugin_root: str):
        """Set the root directory for plugin discovery."""

        cls._plugin_root = os.path.abspath(plugin_root) if plugin_root else None

    @classmethod
    def _derive_model(cls, module_file: str | None):
        """Derive the model name from the module file path."""

        if not module_file or not cls._plugin_root:
            return None
        try:
            rel_dir = os.path.relpath(os.path.dirname(module_file), cls._plugin_root)
        except ValueError:
            return None
        if rel_dir in (".", ""):
            return None
        parts = rel_dir.split(os.sep)
        return "-".join(parts)

    @classmethod
    def register(cls, algorithm_cls):
        """Decorator to register an algorithm class."""

        instance = algorithm_cls()
        code = instance.meta_info["code"]
        cls._registry[code] = instance

        module = sys.modules.get(algorithm_cls.__module__)
        module_file = getattr(module, "__file__", None)
        cls._plugin_meta[code] = {
            "class": algorithm_cls.__name__,
            "module_file": module_file,
        }

        logging.info("[Registry] Algorithm registered: %s (%s)", code, instance.meta_info["name"])
        return algorithm_cls

    @classmethod
    def get_algorithm(cls, code):
        """Retrieve an algorithm instance by its code."""

        return cls._registry.get(code)

    @classmethod
    def get_all_schemes(cls):
        """Retrieve metadata for all registered algorithms."""

        schemes = []
        for code, algo in cls._registry.items():
            meta = dict(algo.meta_info)
            extra = cls._plugin_meta.get(code, {})
            model = meta.get("model") or cls._derive_model(extra.get("module_file"))
            if model:
                meta.setdefault("model", model)
            meta.setdefault("class", extra.get("class", algo.__class__.__name__))
            schemes.append(meta)
        return schemes
