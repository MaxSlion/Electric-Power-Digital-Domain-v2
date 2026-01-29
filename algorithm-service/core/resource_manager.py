# -*- coding: utf-8 -*-

"""
Resource Manager for Hardware Detection and Executor Pools
"""

import logging
import multiprocessing
from concurrent.futures import ThreadPoolExecutor, ProcessPoolExecutor
from typing import Any


class HardwareManager:
    """Singleton class to manage hardware resources and executor pools."""

    _instance = None

    def __new__(cls) -> "HardwareManager":
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __init__(self) -> None:
        if getattr(self, "_initialized", False):
            return
        self._initialized = True

        self.has_gpu = False
        self.device_info = "CPU"
        self.gpu_pool = None
        self.cpu_pool = None
        self._detect_hardware()
        self._init_pools()

    def _detect_hardware(self) -> None:
        """Detect available hardware resources."""

        try:
            import torch  # type: ignore

            if torch.cuda.is_available():
                self.has_gpu = True
                count = torch.cuda.device_count()
                name = torch.cuda.get_device_name(0)
                self.device_info = f"{count}x {name}"
                logging.info("[Init] GPU Detected: %s. VRAM Isolation Active.", self.device_info)
                return
        except Exception as exc:
            logging.warning("[Init] GPU detection skipped: %s", exc)

        logging.info("[Init] No GPU Detected. System running in CPU Mode.")

    def _init_pools(self) -> None:
        """Initialize executor pools based on detected hardware."""

        cpu_cores = max(1, multiprocessing.cpu_count() - 2)
        self.cpu_pool = ProcessPoolExecutor(max_workers=cpu_cores)

        if self.has_gpu:
            self.gpu_pool = ThreadPoolExecutor(max_workers=2, thread_name_prefix="GPU_Worker")
        else:
            self.gpu_pool = self.cpu_pool

    def get_executor(self, resource_pref: str) -> Any:
        """Get the appropriate executor pool based on resource preference."""

        if resource_pref == "GPU" and self.has_gpu:
            return self.gpu_pool
        return self.cpu_pool
