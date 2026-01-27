# -*- coding: utf-8 -*-
"""
Logging configuration for the algorithm service.
"""

import logging
from logging.handlers import RotatingFileHandler
from pathlib import Path


def configure_logging(log_dir: str = "logs", level: int = logging.INFO):
    """Configure logging for the algorithm service."""
    
    Path(log_dir).mkdir(parents=True, exist_ok=True)
    log_file = Path(log_dir) / "algo_service.log"

    formatter = logging.Formatter(
        "[%(asctime)s] [%(levelname)s] [%(name)s] %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )

    handler = RotatingFileHandler(
        log_file,
        maxBytes=10 * 1024 * 1024,
        backupCount=30,
        encoding="utf-8",
    )
    handler.setFormatter(formatter)

    root = logging.getLogger()
    root.setLevel(level)
    root.handlers.clear()
    root.addHandler(handler)
    root.addHandler(logging.StreamHandler())
