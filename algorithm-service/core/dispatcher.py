# -*- coding: utf-8 -*-

"""
Dispatcher module for managing and executing algorithm tasks.
"""

from __future__ import annotations

import logging
import threading
import time
import traceback
from typing import Any, Dict, Optional

from .resource_manager import HardwareManager
from .framework import AlgorithmRegistry, AlgorithmContext
from .plugin_loader import load_plugins
from .process_manager import get_process_manager, ProcessManager
from infrastructure.progress_manager import ProgressManager
from infrastructure.data_loader import load_data_ref
from infrastructure.rpc_client import ResultReporterClient
from infrastructure.task_store import TaskStore
from infrastructure.logging_config import configure_logging


class TaskCancelled(Exception):
    """Raised when a task is cancelled."""


class ProgressQueueReporter:
    """Reporter that sends progress updates to a queue."""

    def __init__(self, queue: Any, status_proxy: Optional[Any] = None) -> None:
        self._queue = queue
        self._status_proxy = status_proxy

    def _update_status_proxy(self, task_id: str, updates: Dict[str, Any]) -> None:
        """Update the status proxy with the given updates."""

        if self._status_proxy is None:
            return
        current = dict(self._status_proxy.get(task_id, {}))
        current.update(updates)
        self._status_proxy[task_id] = current

    def update(self, task_id: str, percent: int, message: str) -> None:
        """Send a progress update to the queue."""

        if self._status_proxy is not None:
            current = dict(self._status_proxy.get(task_id, {}))
            if current.get("status") in ("CANCEL_REQUESTED", "CANCELLED"):
                raise TaskCancelled("Cancellation requested")

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
    scheme_code: str,
    task_id: str,
    data_ref: str,
    params: Dict[str, Any],
    reporter_target: Optional[str],
    progress_queue: Any,
    status_proxy: Optional[Any],
    db_queue: Optional[Any],
) -> None:
    """Run the algorithm task in a subprocess."""

    def _check_cancel() -> None:
        if status_proxy is None:
            return
        current = dict(status_proxy.get(task_id, {}))
        if current.get("status") in ("CANCEL_REQUESTED", "CANCELLED"):
            raise TaskCancelled("Cancellation requested")

    configure_logging()
    load_plugins()
    algo = AlgorithmRegistry.get_algorithm(scheme_code)
    if not algo:
        logging.error("[Dispatcher] Scheme %s not found in subprocess.", scheme_code)
        return

    logger = logging.getLogger("AlgoService")
    progress_stub = ProgressQueueReporter(progress_queue, status_proxy=status_proxy)
    reporter = ResultReporterClient(target=reporter_target)

    try:
        _check_cancel()
        data, _ = load_data_ref(data_ref)
        ctx = AlgorithmContext(task_id, params, progress_stub, logger, data=data)
        ctx.log(logging.INFO, f"Task Started. Scheme: {algo.meta_info['name']}")
        ctx.report_progress(0, "Initializing...")
        _check_cancel()
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
    except TaskCancelled as exc:
        message = str(exc) or "Cancelled"
        if status_proxy is not None:
            progress_stub._update_status_proxy(
                task_id,
                {
                    "percentage": 100,
                    "message": message,
                    "status": "CANCELLED",
                    "updated_at": int(time.time() * 1000),
                },
            )
        progress_queue.put(
            {
                "task_id": task_id,
                "percentage": 100,
                "message": message,
                "timestamp": int(time.time() * 1000),
            }
        )
        if db_queue is not None:
            db_queue.put(
                {
                    "op": "finish",
                    "task_id": task_id,
                    "status": "CANCELLED",
                    "message": message,
                    "error_message": "",
                }
            )
        ctx.log(logging.INFO, "Task Cancelled")
        reporter.send_result(task_id, status="CANCELLED", error=message)
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

    def __init__(self, reporter_client: ResultReporterClient, reporter_target: Optional[str] = None) -> None:
        self.hardware = HardwareManager()
        self.reporter = reporter_client
        self.reporter_target = reporter_target
        self._tasks: Dict[str, Any] = {}
        self._tasks_lock = threading.Lock()

    def cancel_task(self, task_id: str, force: bool = False) -> Dict[str, Any]:
        """
        Request cancellation of a task.
        
        Args:
            task_id: The task to cancel.
            force: If True, immediately force-kill the process (SIGKILL).
                   If False, try cooperative cancel first, then SIGTERM.
        
        Returns:
            Dict with accepted, message, status keys.
        """
        manager = ProgressManager.get_instance()
        current = manager.get_task(task_id)
        status = (current.get("status") or "").upper()
        if status in ("SUCCESS", "FAILED", "CANCELLED"):
            return {"accepted": False, "message": "Task already finished", "status": status}

        # Try cooperative cancel first (for ThreadPool GPU tasks)
        future = None
        with self._tasks_lock:
            future = self._tasks.get(task_id)

        if future is not None and future.cancel():
            self._mark_cancelled(task_id, message="Cancelled before start")
            return {"accepted": True, "message": "Cancelled", "status": "CANCELLED"}

        # Try process-level termination for CPU tasks
        proc_mgr = get_process_manager()
        if proc_mgr.is_running(task_id):
            result = proc_mgr.cancel(task_id, force=force)
            if result["accepted"]:
                # Mark task as cancelled in progress manager
                if result["status"] == "KILLED":
                    self._mark_cancelled(task_id, message="Force killed")
                    return {"accepted": True, "message": "Force killed", "status": "CANCELLED"}
                else:
                    # TERMINATING - process will be killed soon
                    manager.request_cancel(task_id, message="Terminating")
                    TaskStore().update_progress(task_id, 0, "Terminating", status="CANCEL_REQUESTED")
                    # Start a watcher to mark cancelled after process dies
                    self._watch_termination(task_id, proc_mgr)
                    return {"accepted": True, "message": result["message"], "status": "TERMINATING"}
            else:
                return result

        # Fall back to cooperative cancel
        manager.request_cancel(task_id, message="Cancel requested")
        TaskStore().update_progress(task_id, 0, "Cancel requested", status="CANCEL_REQUESTED")
        return {"accepted": True, "message": "Cancel requested", "status": "CANCEL_REQUESTED"}

    def _watch_termination(self, task_id: str, proc_mgr: ProcessManager) -> None:
        """Watch for process termination and mark task as cancelled."""
        import threading

        def _watcher() -> None:
            # Wait up to 10 seconds for process to die
            for _ in range(20):
                if not proc_mgr.is_running(task_id):
                    self._mark_cancelled(task_id, message="Terminated")
                    return
                time.sleep(0.5)
            # If still running after 10s, force kill
            proc_mgr.cancel(task_id, force=True)
            self._mark_cancelled(task_id, message="Force killed")

        threading.Thread(target=_watcher, daemon=True, name=f"TermWatcher-{task_id[:8]}").start()

    def dispatch(self, task_id: str, scheme_code: str, data_ref: str, params: Dict[str, Any]) -> None:
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
        if executor is None:
            self._report_error(task_id, "Executor not available for resource type")
            return

        try:
            if executor is self.hardware.cpu_pool:
                # Use ProcessManager for CPU tasks (supports force-kill)
                proc_mgr = get_process_manager()
                proc_mgr.submit(
                    _run_task_in_subprocess,
                    task_id,
                    scheme_code,
                    task_id,
                    data_ref,
                    params,
                    self.reporter_target,
                    progress_queue,
                    status_proxy,
                    db_queue,
                )
                # No future for direct process management
                future = None
            else:
                # GPU tasks still use ThreadPoolExecutor
                future = executor.submit(
                    self._safe_runner,
                    algo,
                    task_id,
                    data_ref,
                    params,
                    progress_queue,
                    status_proxy,
                    db_queue
                )
            if future is not None:
                with self._tasks_lock:
                    self._tasks[task_id] = future
                future.add_done_callback(lambda _f, t=task_id: self._cleanup_task(t))
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

    def _safe_runner(
        self,
        algo: Any,
        task_id: str,
        data_ref: str,
        params: Dict[str, Any],
        progress_queue: Any,
        status_proxy: Optional[Any],
        db_queue: Optional[Any],
    ) -> None:
        """Run the algorithm task safely within the current process."""

        def _check_cancel() -> None:
            if status_proxy is None:
                return
            current = dict(status_proxy.get(task_id, {}))
            if current.get("status") in ("CANCEL_REQUESTED", "CANCELLED"):
                raise TaskCancelled("Cancellation requested")

        logger = logging.getLogger("AlgoService")
        progress_stub = ProgressQueueReporter(progress_queue, status_proxy=status_proxy)

        try:
            _check_cancel()
            data, _ = load_data_ref(data_ref)
            ctx = AlgorithmContext(task_id, params, progress_stub, logger, data=data)
            ctx.log(logging.INFO, f"Task Started. Scheme: {algo.meta_info['name']}")
            ctx.report_progress(0, "Initializing...")
            _check_cancel()
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
        except TaskCancelled as exc:
            message = str(exc) or "Cancelled"
            if status_proxy is not None:
                progress_stub._update_status_proxy(
                    task_id,
                    {
                        "percentage": 100,
                        "message": message,
                        "status": "CANCELLED",
                        "updated_at": int(time.time() * 1000),
                    },
                )
            progress_queue.put(
                {
                    "task_id": task_id,
                    "percentage": 100,
                    "message": message,
                    "timestamp": int(time.time() * 1000),
                }
            )
            if db_queue is not None:
                db_queue.put(
                    {
                        "op": "finish",
                        "task_id": task_id,
                        "status": "CANCELLED",
                        "message": message,
                        "error_message": "",
                    }
                )
            ctx.log(logging.INFO, "Task Cancelled")
            self.reporter.send_result(task_id, status="CANCELLED", error=message)
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

    def _report_error(self, task_id: str, message: str) -> None:
        """Report an error for the given task."""

        logging.error("[Dispatcher] %s", message)
        TaskStore().upsert_task_start(task_id, scheme_code="", data_ref="")
        TaskStore().finish_task(task_id, status="FAILED", message="Failed", error_message=message)
        logging.error("[Dispatcher] Task Failed: %s", task_id)
        self.reporter.send_result(task_id, status="FAILED", error=message)

    def _cleanup_task(self, task_id: str) -> None:
        with self._tasks_lock:
            self._tasks.pop(task_id, None)

    def _mark_cancelled(self, task_id: str, message: str) -> None:
        manager = ProgressManager.get_instance()
        manager.mark_finished(task_id, "CANCELLED", message=message)
        TaskStore().finish_task(task_id, status="CANCELLED", message=message, error_message="")
        self.reporter.send_result(task_id, status="CANCELLED", error=message)
