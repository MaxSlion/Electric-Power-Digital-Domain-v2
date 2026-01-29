# -*- coding: utf-8 -*-

"""
Dynamically load all plugin modules from the specified package.
"""

import importlib
import importlib.util
import os
import sys
from pathlib import Path
from typing import Any

from core.framework import AlgorithmRegistry


def _import_module_from_path(module_name: str, module_path: str) -> Any:
    """Import a module given its file path."""

    spec = importlib.util.spec_from_file_location(module_name, module_path)
    if not spec or not spec.loader:
        raise ImportError(f"Cannot load module from {module_path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    return module


def _should_import(module_path: str) -> bool:
    """Check whether a plugin module looks like an algorithm implementation."""

    try:
        content = Path(module_path).read_text(encoding="utf-8")
    except Exception:
        return False
    markers = ("AlgorithmRegistry.register", "BaseAlgorithm")
    return any(marker in content for marker in markers)


def load_plugins(package_name: str = "plugins") -> None:
    """Dynamically load all modules in the specified package."""

    package = importlib.import_module(package_name)
    package_path = os.path.abspath(next(iter(package.__path__)))
    AlgorithmRegistry.set_plugin_root(package_path)

    for root, dirs, files in os.walk(package_path):
        if "__pycache__" in dirs:
            dirs.remove("__pycache__")
        for filename in files:
            if not filename.endswith(".py") or filename == "__init__.py":
                continue
            module_path = os.path.join(root, filename)
            if not _should_import(module_path):
                continue
            rel_path = os.path.relpath(module_path, package_path)
            module_name = rel_path[:-3].replace(os.sep, ".")
            full_name = f"{package_name}.{module_name}"
            if full_name in sys.modules:
                continue
            try:
                importlib.import_module(full_name)
            except ModuleNotFoundError:
                _import_module_from_path(full_name, module_path)
