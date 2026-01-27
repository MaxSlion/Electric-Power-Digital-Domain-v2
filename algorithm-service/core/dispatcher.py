# -*- coding: utf-8 -*-
"""
Dispatcher module for managing and executing algorithm tasks.
"""

from __future__ import annotations

import logging
import time
import traceback

from .resource_manager import HardwareManager
from .framework import AlgorithmRegistry, AlgorithmContext
from .plugin_loader import load_plugins
from infrastructure.progress_manager import ProgressManager
from infrastructure.rpc_client import ResultReporterClient
from infrastructure.task_store import TaskStore
from infrastructure.logging_config import configure_logging


class ProgressQueueReporter:
    """Reporter that sends progress updates to a queue."""

    def __init__(self, queue, status_proxy=None):
        self._queue = queue
        self._status_proxy = status_proxy

    def _update_status_proxy(self, task_id, updates: dict):
        """Update the status proxy with the given updates."""

        if self._status_proxy is None:
            return
        current = dict(self._status_proxy.get(task_id, {}))
        current.update(updates)
        self._status_proxy[task_id] = current

    def update(self, task_id, percent, message):
        """Send a progress update to the queue."""

        payload = {
            "task_id": task_id,
            "percentage": int(percent),
            "message": message,
            "timestamp": int(time.time() * 1000),
        }
        self._queue.put(payload)
        self._update_status_proxy(
            task_id,
            {
                "percentage": int(percent),
                "message": message,
                "status": "RUNNING",
                "updated_at": int(time.time() * 1000),
            },
        )


def _run_task_in_subprocess(
    scheme_code,
    task_id,
    data_ref,
    params,
    reporter_target,
    progress_queue,
    status_proxy,
    db_queue,
):
    """Run the algorithm task in a subprocess."""

    configure_logging()
    load_plugins()
    algo = AlgorithmRegistry.get_algorithm(scheme_code)
    if not algo:
        logging.error("[Dispatcher] Scheme %s not found in subprocess.", scheme_code)
        return

    logger = logging.getLogger("AlgoService")
    progress_stub = ProgressQueueReporter(progress_queue, status_proxy=status_proxy)
    ctx = AlgorithmContext(task_id, data_ref, params, progress_stub, logger)
    reporter = ResultReporterClient(target=reporter_target)

    try:
        ctx.log(logging.INFO, f"Task Started. Scheme: {algo.meta_info['name']}")
        ctx.report_progress(0, "Initializing...")
        result = algo.execute(ctx)
        ctx.report_progress(100, "Completed")
        if status_proxy is not None:
            progress_stub._update_status_proxy(
                task_id,
                {
                    "percentage": 100,
                    "message": "Completed",
                    "status": "SUCCESS",
                    "updated_at": int(time.time() * 1000),
                },
            )
        progress_queue.put(
            {
                "task_id": task_id,
                "percentage": 100,
                "message": "Completed",
                "timestamp": int(time.time() * 1000),
            }
        )
        if db_queue is not None:
            db_queue.put(
                {
                    "op": "finish",
                    "task_id": task_id,
                    "status": "SUCCESS",
                    "message": "Completed",
                    "error_message": "",
                }
            )
        ctx.log(logging.INFO, "Task Completed")
        reporter.send_result(task_id, status="SUCCESS", data=result)
    except Exception as exc:
        err_msg = str(exc)
        stack = traceback.format_exc()
        ctx.log(logging.ERROR, f"Task Failed: {err_msg}\n{stack}")
        if status_proxy is not None:
            progress_stub._update_status_proxy(
                task_id,
                {
                    "percentage": 100,
                    "message": "Failed",
                    "status": "FAILED",
                    "updated_at": int(time.time() * 1000),
                },
            )
        progress_queue.put(
            {
                "task_id": task_id,
                "percentage": 100,
                "message": "Failed",
                "timestamp": int(time.time() * 1000),
            }
        )
        if db_queue is not None:
            db_queue.put(
                {
                    "op": "finish",
                    "task_id": task_id,
                    "status": "FAILED",
                    "message": "Failed",
                    "error_message": err_msg,
                }
            )
        ctx.log(logging.ERROR, "Task Failed")
        reporter.send_result(task_id, status="FAILED", error=err_msg)


class TaskDispatcher:
    """Dispatcher for managing and executing algorithm tasks."""

    def __init__(self, reporter_client, reporter_target: str | None = None):
        self.hardware = HardwareManager()
        self.reporter = reporter_client
        self.reporter_target = reporter_target

    def dispatch(self, task_id, scheme_code, data_ref, params):
        """Dispatch the algorithm task to the appropriate executor."""

        algo = AlgorithmRegistry.get_algorithm(scheme_code)
        if not algo:
            self._report_error(task_id, f"Scheme {scheme_code} not found")
            return

        manager = ProgressManager.get_instance()
        manager.start_db_writer()
        manager.register_task(task_id, scheme_code, data_ref=data_ref)
        progress_queue = manager.ensure_queue(task_id)
        status_proxy = manager.get_status_proxy()
        db_queue = manager.get_db_queue()
        TaskStore().upsert_task_start(task_id, scheme_code, data_ref=data_ref)

        executor = self.hardware.get_executor(algo.meta_info["resource_type"])

        try:
            if executor is self.hardware.cpu_pool:
                executor.submit(
                    _run_task_in_subprocess,
                    scheme_code,
                    task_id,
                    data_ref,
                    params,
                    self.reporter_target,
                    progress_queue,
                    status_proxy,
                    db_queue,
                )
            else:
                executor.submit(self._safe_runner, algo, task_id, data_ref, params, progress_queue, status_proxy, db_queue)
        except Exception as exc:
            err_msg = str(exc)
            progress_queue.put(
                {
                    "task_id": task_id,
                    "percentage": 100,
                    "message": "Failed",
                    "timestamp": int(time.time() * 1000),
                }
            )
            TaskStore().finish_task(task_id, status="FAILED", message="Failed", error_message=err_msg)
            manager.mark_finished(task_id, "FAILED", message="Failed")
            self.reporter.send_result(task_id, status="FAILED", error=err_msg)

    def _safe_runner(self, algo, task_id, data_ref, params, progress_queue, status_proxy, db_queue):
        """Run the algorithm task safely within the current process."""

        logger = logging.getLogger("AlgoService")
        progress_stub = ProgressQueueReporter(progress_queue, status_proxy=status_proxy)
        ctx = AlgorithmContext(task_id, data_ref, params, progress_stub, logger)

        try:
            ctx.log(logging.INFO, f"Task Started. Scheme: {algo.meta_info['name']}")
            ctx.report_progress(0, "Initializing...")
            result = algo.execute(ctx)
            ctx.report_progress(100, "Completed")
            if status_proxy is not None:
                progress_stub._update_status_proxy(
                    task_id,
                    {
                        "percentage": 100,
                        "message": "Completed",
                        "status": "SUCCESS",
                        "updated_at": int(time.time() * 1000),
                    },
                )
            progress_queue.put(
                {
                    "task_id": task_id,
                    "percentage": 100,
                    "message": "Completed",
                    "timestamp": int(time.time() * 1000),
                }
            )
            if db_queue is not None:
                db_queue.put(
                    {
                        "op": "finish",
                        "task_id": task_id,
                        "status": "SUCCESS",
                        "message": "Completed",
                        "error_message": "",
                    }
                )
            ctx.log(logging.INFO, "Task Completed")
            self.reporter.send_result(task_id, status="SUCCESS", data=result)
        except Exception as exc:
            err_msg = str(exc)
            stack = traceback.format_exc()
            ctx.log(logging.ERROR, f"Task Failed: {err_msg}\n{stack}")
            if status_proxy is not None:
                progress_stub._update_status_proxy(
                    task_id,
                    {
                        "percentage": 100,
                        "message": "Failed",
                        "status": "FAILED",
                        "updated_at": int(time.time() * 1000),
                    },
                )
            progress_queue.put(
                {
                    "task_id": task_id,
                    "percentage": 100,
                    "message": "Failed",
                    "timestamp": int(time.time() * 1000),
                }
            )
            if db_queue is not None:
                db_queue.put(
                    {
                        "op": "finish",
                        "task_id": task_id,
                        "status": "FAILED",
                        "message": "Failed",
                        "error_message": err_msg,
                    }
                )
            ctx.log(logging.ERROR, "Task Failed")
            self.reporter.send_result(task_id, status="FAILED", error=err_msg)

    def _report_error(self, task_id, message):
        """Report an error for the given task."""

        logging.error("[Dispatcher] %s", message)
        TaskStore().upsert_task_start(task_id, scheme_code="", data_ref="")
        TaskStore().finish_task(task_id, status="FAILED", message="Failed", error_message=message)
        logging.error("[Dispatcher] Task Failed: %s", task_id)
        self.reporter.send_result(task_id, status="FAILED", error=message)
