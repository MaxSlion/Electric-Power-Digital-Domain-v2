# -*- coding: utf-8 -*-

"""
Progress Manager for Tracking Algorithm Task Progress
"""

import logging
import multiprocessing
import threading
import time
from multiprocessing.managers import SyncManager
from typing import Any, Dict, Optional


class ProgressManager:
    """Singleton class to manage progress tracking for algorithm tasks."""

    _instance = None

    def __init__(self) -> None:
        self._queues = {}
        self._result_sink = None
        self._manager = None
        self._status = None
        self._db_queue = None
        self._db_thread = None
        self._db_stats = {"success": 0, "fail": 0, "last_error": ""}
        self._db_stats_lock = threading.Lock()

    def _get_manager(self) -> SyncManager:
        """Get or create the multiprocessing manager."""

        if self._manager is None:
            self._manager = multiprocessing.Manager()
        if self._status is None:
            self._status = self._manager.dict()
        return self._manager

    def _new_queue(self) -> Any:
        """Create a new shared queue."""

        return self._get_manager().Queue()

    def _get_status_store(self) -> Any:
        """Get the shared status store."""

        self._get_manager()
        if self._status is None:
            self._status = self._manager.dict() if self._manager else {}
        return self._status

    def _update_status(self, task_id: str, updates: Dict[str, Any]) -> None:
        """Update the status of a task in the status store."""

        store = self._get_status_store()
        current = dict(store.get(task_id, {}))
        current.update(updates)
        store[task_id] = current

    @classmethod
    def get_instance(cls) -> "ProgressManager":
        """Get the singleton instance of ProgressManager."""

        if not cls._instance:
            cls._instance = cls()
        return cls._instance

    def bind_result_sink(self, sink: Any) -> None:
        """Bind a result sink to report final task results."""

        self._result_sink = sink

    def register_task(self, task_id: str, scheme_code: str, data_ref: str = "") -> None:
        """Register a task in the status store."""

        now = int(time.time() * 1000)
        self._update_status(
            task_id,
            {
                "task_id": task_id,
                "scheme_code": scheme_code,
                "status": "QUEUED",
                "percentage": 0,
                "message": "Queued",
                "data_ref": data_ref,
                "updated_at": now,
            },
        )

    def record_progress(self, task_id: str, percent: int, message: str) -> None:
        """Record progress in the status store."""

        current = self.get_task(task_id)
        if current.get("status") in ("SUCCESS", "FAILED", "CANCELLED"):
            return
        now = int(time.time() * 1000)
        status = "RUNNING"
        if current.get("status") == "CANCEL_REQUESTED":
            status = "CANCEL_REQUESTED"
        self._update_status(
            task_id,
            {
                "percentage": int(percent),
                "message": message,
                "status": status,
                "updated_at": now,
            },
        )

    def mark_finished(self, task_id: str, status: str, message: str = "Completed") -> None:
        """Mark a task as finished in the status store."""

        now = int(time.time() * 1000)
        self._update_status(
            task_id,
            {
                "percentage": 100,
                "message": message,
                "status": status,
                "updated_at": now,
            },
        )

    def request_cancel(self, task_id: str, message: str = "Cancel requested") -> None:
        """Mark a task as cancel requested."""

        now = int(time.time() * 1000)
        self._update_status(
            task_id,
            {
                "message": message,
                "status": "CANCEL_REQUESTED",
                "updated_at": now,
            },
        )

    def is_cancel_requested(self, task_id: str) -> bool:
        """Check if a task has cancellation requested."""

        current = self.get_task(task_id)
        return current.get("status") in ("CANCEL_REQUESTED", "CANCELLED")

    def get_task(self, task_id: str) -> Dict[str, Any]:
        """Return the last known status for a task."""

        store = self._get_status_store()
        return dict(store.get(task_id, {}))

    def list_tasks(self) -> list[Dict[str, Any]]:
        """Return all known tasks with their status."""

        store = self._get_status_store()
        return [dict(value) for value in store.values()]

    def get_status_proxy(self) -> Any:
        """Return the shared status store proxy."""

        return self._get_status_store()

    def register_watcher(self, task_id: str) -> Any:
        """Register a watcher for a specific task ID."""

        if task_id in self._queues:
            return self._queues[task_id]
        q = self._new_queue()
        self._queues[task_id] = q

        status = self.get_task(task_id)
        if status:
            payload = {
                "task_id": task_id,
                "percentage": int(status.get("percentage", 0)),
                "message": status.get("message", ""),
                "timestamp": int(time.time() * 1000),
            }
            q.put(payload)
        return q

    def ensure_queue(self, task_id: str) -> Any:
        """Ensure a shared queue exists for cross-process progress updates."""

        if task_id in self._queues:
            return self._queues[task_id]
        q = self._new_queue()
        self._queues[task_id] = q
        return q

    def update(self, task_id: str, percent: int, message: str) -> None:
        """Update the progress of a specific task."""

        payload = {
            "task_id": task_id,
            "percentage": int(percent),
            "message": message,
            "timestamp": int(time.time() * 1000),
        }
        self.record_progress(task_id, percent, message)
        if task_id in self._queues:
            self._queues[task_id].put(payload)

    def close_watcher(self, task_id: str) -> None:
        """Close the watcher for a specific task ID."""

        if task_id in self._queues:
            del self._queues[task_id]

    def report_result(self, task_id: str, status: str, data: Any = None, error: Optional[str] = None) -> None:
        """Report the final result of a task to the result sink."""

        if self._result_sink:
            self._result_sink.send_result(task_id, status=status, data=data, error=error)

    def _get_db_queue(self) -> Any:
        """Get or create the shared DB writer queue."""

        manager = self._get_manager()
        if self._db_queue is None:
            self._db_queue = manager.Queue()
        return self._db_queue

    def start_db_writer(self) -> None:
        """Start a background DB writer thread in the main process."""

        if multiprocessing.current_process().name != "MainProcess":
            return
        if self._db_thread and self._db_thread.is_alive():
            return

        def _worker(db_queue: Any) -> None:
            from infrastructure.task_store import TaskStore

            store = TaskStore()
            while True:
                try:
                    event = db_queue.get()
                except (EOFError, BrokenPipeError, OSError):
                    logging.info("[TaskStore] DB writer exiting: queue closed")
                    break
                if not isinstance(event, dict):
                    continue
                op = event.get("op")
                attempts = 0
                while True:
                    try:
                        if op == "finish":
                            store.finish_task(
                                event.get("task_id", ""),
                                status=event.get("status", "FAILED"),
                                message=event.get("message", "Completed"),
                                error_message=event.get("error_message", ""),
                            )
                        elif op == "progress":
                            store.update_progress(
                                event.get("task_id", ""),
                                int(event.get("percentage", 0)),
                                event.get("message", ""),
                                status=event.get("status", "RUNNING"),
                            )
                        else:
                            logging.warning("[TaskStore] Unknown event op: %s", op)
                        with self._db_stats_lock:
                            self._db_stats["success"] += 1
                        break
                    except Exception as exc:
                        attempts += 1
                        if attempts >= 3:
                            logging.error("[TaskStore] Persist failed after retries: %s", exc)
                            with self._db_stats_lock:
                                self._db_stats["fail"] += 1
                                self._db_stats["last_error"] = str(exc)
                            break
                        time.sleep(0.05 * (2**attempts))

        self._db_thread = threading.Thread(
            target=_worker,
            args=(self._get_db_queue(),),
            name="TaskStoreWriter",
            daemon=True,
        )
        self._db_thread.start()

    def get_db_queue(self) -> Any:
        """Return the shared DB writer queue."""

        return self._get_db_queue()

    def get_db_stats(self) -> Dict[str, Any]:
        """Return DB writer success/failure counters."""

        with self._db_stats_lock:
            return dict(self._db_stats)
