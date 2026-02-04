# -*- coding: utf-8 -*-

"""
Process Manager for Forceful Task Termination Support.

This module provides direct process management to enable force-kill
of running algorithm tasks, replacing ProcessPoolExecutor for CPU tasks.
"""

from __future__ import annotations

import logging
import multiprocessing
import os
import signal
import threading
import time
from dataclasses import dataclass, field
from multiprocessing import Process
from typing import Any, Callable, Dict, Optional


@dataclass
class ManagedProcess:
    """Container for a managed subprocess with metadata."""

    process: Process
    task_id: str
    started_at: float = field(default_factory=time.time)
    cancel_requested: bool = False


class ProcessManager:
    """
    Manager for algorithm task processes with force-kill support.
    
    Unlike ProcessPoolExecutor, this class maintains direct references
    to subprocess handles, enabling SIGTERM/SIGKILL for stuck tasks.
    """

    _instance: Optional["ProcessManager"] = None
    _lock = threading.Lock()

    # Grace period before SIGKILL after SIGTERM (seconds)
    TERMINATE_GRACE_PERIOD = 5.0

    def __new__(cls) -> "ProcessManager":
        with cls._lock:
            if cls._instance is None:
                cls._instance = super().__new__(cls)
            return cls._instance

    def __init__(self) -> None:
        if getattr(self, "_initialized", False):
            return
        self._initialized = True

        self._processes: Dict[str, ManagedProcess] = {}
        self._processes_lock = threading.Lock()
        self._max_workers = max(1, multiprocessing.cpu_count() - 2)
        self._semaphore = threading.Semaphore(self._max_workers)
        self._cleanup_thread: Optional[threading.Thread] = None
        self._shutdown = False

        logging.info(
            "[ProcessManager] Initialized with max_workers=%d", self._max_workers
        )

    def submit(
        self,
        target: Callable[..., Any],
        task_id: str,
        *args: Any,
        **kwargs: Any,
    ) -> bool:
        """
        Submit a task to run in a new subprocess.
        
        Args:
            target: The function to execute in the subprocess.
            task_id: Unique task identifier for tracking.
            *args: Positional arguments for target.
            **kwargs: Keyword arguments for target.
            
        Returns:
            True if submitted successfully, False if at capacity.
        """
        if self._shutdown:
            logging.warning("[ProcessManager] Rejecting task %s: shutdown in progress", task_id)
            return False

        # Use semaphore to limit concurrent processes
        acquired = self._semaphore.acquire(blocking=False)
        if not acquired:
            logging.warning(
                "[ProcessManager] At capacity (%d workers), task %s queued",
                self._max_workers,
                task_id,
            )
            # Block until a slot is available
            self._semaphore.acquire(blocking=True)

        try:
            process = Process(target=target, args=args, kwargs=kwargs, daemon=False)
            process.start()

            managed = ManagedProcess(process=process, task_id=task_id)
            with self._processes_lock:
                self._processes[task_id] = managed

            logging.info(
                "[ProcessManager] Started task %s (pid=%d)",
                task_id,
                process.pid,
            )

            # Start monitor thread for this process
            monitor = threading.Thread(
                target=self._monitor_process,
                args=(task_id,),
                daemon=True,
                name=f"ProcessMonitor-{task_id[:8]}",
            )
            monitor.start()

            return True
        except Exception as exc:
            self._semaphore.release()
            logging.error("[ProcessManager] Failed to start task %s: %s", task_id, exc)
            return False

    def _monitor_process(self, task_id: str) -> None:
        """Monitor a process and clean up when it exits."""
        managed = None
        with self._processes_lock:
            managed = self._processes.get(task_id)

        if managed is None:
            return

        try:
            managed.process.join()  # Wait for process to finish
        finally:
            self._cleanup(task_id)
            self._semaphore.release()

    def _cleanup(self, task_id: str) -> None:
        """Remove a task from tracking."""
        with self._processes_lock:
            managed = self._processes.pop(task_id, None)
            if managed:
                logging.info(
                    "[ProcessManager] Task %s finished (exitcode=%s)",
                    task_id,
                    managed.process.exitcode,
                )

    def cancel(self, task_id: str, force: bool = False) -> Dict[str, Any]:
        """
        Cancel/terminate a running task.
        
        Args:
            task_id: The task to cancel.
            force: If True, immediately SIGKILL. If False, try SIGTERM first.
            
        Returns:
            Dict with keys: accepted, message, status, pid
        """
        with self._processes_lock:
            managed = self._processes.get(task_id)

        if managed is None:
            return {
                "accepted": False,
                "message": "Task not found or already finished",
                "status": "NOT_FOUND",
                "pid": None,
            }

        process = managed.process
        pid = process.pid

        if not process.is_alive():
            return {
                "accepted": False,
                "message": "Task already finished",
                "status": "FINISHED",
                "pid": pid,
            }

        managed.cancel_requested = True

        if force:
            return self._force_kill(task_id, process, pid)
        else:
            return self._graceful_terminate(task_id, process, pid)

    def _graceful_terminate(
        self, task_id: str, process: Process, pid: int
    ) -> Dict[str, Any]:
        """Try SIGTERM first, then SIGKILL after grace period."""
        try:
            if os.name == "nt":
                # Windows: terminate() is roughly equivalent to SIGTERM
                process.terminate()
            else:
                # Unix: send SIGTERM
                os.kill(pid, signal.SIGTERM)

            logging.info("[ProcessManager] Sent SIGTERM to task %s (pid=%d)", task_id, pid)

            # Start a background thread to force-kill if needed
            def _escalate() -> None:
                time.sleep(self.TERMINATE_GRACE_PERIOD)
                if process.is_alive():
                    logging.warning(
                        "[ProcessManager] Task %s did not terminate, sending SIGKILL",
                        task_id,
                    )
                    self._force_kill(task_id, process, pid)

            threading.Thread(target=_escalate, daemon=True).start()

            return {
                "accepted": True,
                "message": f"SIGTERM sent, will force-kill after {self.TERMINATE_GRACE_PERIOD}s",
                "status": "TERMINATING",
                "pid": pid,
            }
        except Exception as exc:
            logging.error(
                "[ProcessManager] Failed to terminate task %s: %s", task_id, exc
            )
            return {
                "accepted": False,
                "message": f"Failed to terminate: {exc}",
                "status": "ERROR",
                "pid": pid,
            }

    def _force_kill(
        self, task_id: str, process: Process, pid: int
    ) -> Dict[str, Any]:
        """Immediately kill the process."""
        try:
            if os.name == "nt":
                # Windows: kill() is equivalent to TerminateProcess
                process.kill()
            else:
                # Unix: SIGKILL
                os.kill(pid, signal.SIGKILL)

            logging.info("[ProcessManager] Sent SIGKILL to task %s (pid=%d)", task_id, pid)

            return {
                "accepted": True,
                "message": "Force killed",
                "status": "KILLED",
                "pid": pid,
            }
        except Exception as exc:
            logging.error(
                "[ProcessManager] Failed to kill task %s: %s", task_id, exc
            )
            return {
                "accepted": False,
                "message": f"Failed to kill: {exc}",
                "status": "ERROR",
                "pid": pid,
            }

    def is_running(self, task_id: str) -> bool:
        """Check if a task is currently running."""
        with self._processes_lock:
            managed = self._processes.get(task_id)
            if managed is None:
                return False
            return managed.process.is_alive()

    def get_running_tasks(self) -> list[str]:
        """Get list of currently running task IDs."""
        with self._processes_lock:
            return [
                tid for tid, m in self._processes.items() if m.process.is_alive()
            ]

    def shutdown(self, wait: bool = True, cancel_pending: bool = False) -> None:
        """
        Shutdown the process manager.
        
        Args:
            wait: If True, wait for running tasks to complete.
            cancel_pending: If True, terminate all running tasks.
        """
        self._shutdown = True

        if cancel_pending:
            with self._processes_lock:
                task_ids = list(self._processes.keys())
            for task_id in task_ids:
                self.cancel(task_id, force=True)

        if wait:
            with self._processes_lock:
                processes = [m.process for m in self._processes.values()]
            for p in processes:
                p.join(timeout=10)

        logging.info("[ProcessManager] Shutdown complete")


# Singleton accessor
def get_process_manager() -> ProcessManager:
    """Get the singleton ProcessManager instance."""
    return ProcessManager()
