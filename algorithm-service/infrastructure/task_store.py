# -*- coding: utf-8 -*-

"""
SQLite-backed task store for tracking algorithm execution status.
"""

from __future__ import annotations

import sqlite3
import time
from contextlib import closing
from pathlib import Path


class TaskStore:
    """SQLite-backed task store for tracking algorithm execution status."""

    def __init__(self, db_path: str | None = None):
        base_dir = Path(__file__).resolve().parents[1]
        data_dir = base_dir / "data"
        data_dir.mkdir(parents=True, exist_ok=True)
        self._db_path = Path(db_path) if db_path else data_dir / "tasks.db"
        self._conn = None
        self._ensure_db()

    def _connect(self):
        """Establish a connection to the SQLite database."""

        if self._conn is None:
            self._conn = sqlite3.connect(self._db_path, timeout=30, check_same_thread=False)
            self._conn.execute("PRAGMA journal_mode=WAL;")
            self._conn.execute("PRAGMA synchronous=NORMAL;")
        return self._conn

    def _execute_with_retry(self, sql: str, params: tuple = (), retries: int = 3):
        """Execute a SQL statement with retries on database lock."""

        last_exc = None
        for attempt in range(retries + 1):
            try:
                return self._connect().execute(sql, params)
            except sqlite3.OperationalError as exc:
                last_exc = exc
                if "locked" not in str(exc).lower() or attempt >= retries:
                    raise
                time.sleep(0.05 * (2**attempt))
        if last_exc is not None:
            raise last_exc
        raise RuntimeError("Failed to execute SQL statement after retries.")

    def _ensure_db(self):
        """Ensure the tasks table exists in the database."""

        conn = self._connect()
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS tasks (
                task_id TEXT PRIMARY KEY,
                scheme_code TEXT,
                status TEXT,
                percentage INTEGER,
                message TEXT,
                error_message TEXT,
                data_ref TEXT,
                created_at INTEGER,
                updated_at INTEGER
            )
            """
        )
        with closing(conn.execute("PRAGMA table_info(tasks)")) as cursor:
            columns = {row[1] for row in cursor.fetchall()}
        if "error_message" not in columns:
            conn.execute("ALTER TABLE tasks ADD COLUMN error_message TEXT")
        conn.commit()

    def _now_ms(self) -> int:
        """Get the current time in milliseconds."""

        return int(time.time() * 1000)

    def upsert_task_start(self, task_id: str, scheme_code: str, data_ref: str = ""):
        """Insert or update a task to mark its start."""

        now = self._now_ms()
        cursor = self._execute_with_retry(
            """
            INSERT INTO tasks (
                task_id,
                scheme_code,
                status,
                percentage,
                message,
                error_message,
                data_ref,
                created_at,
                updated_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(task_id) DO UPDATE SET
                scheme_code = excluded.scheme_code,
                status = excluded.status,
                percentage = excluded.percentage,
                message = excluded.message,
                error_message = excluded.error_message,
                data_ref = excluded.data_ref,
                updated_at = excluded.updated_at
            """,
            (task_id, scheme_code, "RUNNING", 0, "Initializing", "", data_ref, now, now),
        )
        cursor.connection.commit()

    def update_progress(self, task_id: str, percentage: int, message: str, status: str = "RUNNING"):
        """Update the progress of an existing task."""

        now = self._now_ms()
        cursor = self._execute_with_retry(
            """
            UPDATE tasks
            SET percentage = ?, message = ?, status = ?, updated_at = ?
            WHERE task_id = ?
            """,
            (int(percentage), message, status, now, task_id),
        )
        if cursor.rowcount == 0:
            self.upsert_task_start(task_id, scheme_code="", data_ref="")
        cursor.connection.commit()

    def finish_task(self, task_id: str, status: str, message: str = "Completed", error_message: str = ""):
        """Mark a task as finished."""

        now = self._now_ms()
        cursor = self._execute_with_retry(
            """
            UPDATE tasks
            SET percentage = ?, message = ?, status = ?, error_message = ?, updated_at = ?
            WHERE task_id = ?
            """,
            (100, message, status, error_message, now, task_id),
        )
        if cursor.rowcount == 0:
            self.upsert_task_start(task_id, scheme_code="", data_ref="")
            self.finish_task(task_id, status=status, message=message, error_message=error_message)
            return
        cursor.connection.commit()

    def get_task(self, task_id: str) -> dict:
        """Retrieve the details of a specific task."""

        cursor = self._execute_with_retry(
            """
            SELECT task_id, scheme_code, status, percentage, message, error_message, data_ref, created_at, updated_at
            FROM tasks
            WHERE task_id = ?
            """,
            (task_id,),
        )
        row = cursor.fetchone()
        if not row:
            return {}
        return {
            "task_id": row[0],
            "scheme_code": row[1],
            "status": row[2],
            "percentage": row[3],
            "message": row[4],
            "error_message": row[5],
            "data_ref": row[6],
            "created_at": row[7],
            "updated_at": row[8],
        }

    def list_tasks(self) -> list[dict]:
        """List all tasks in the store."""

        cursor = self._execute_with_retry(
            """
            SELECT task_id, scheme_code, status, percentage, message, error_message, data_ref, created_at, updated_at
            FROM tasks
            ORDER BY updated_at DESC
            """
        )
        rows = cursor.fetchall()
        return [
            {
                "task_id": row[0],
                "scheme_code": row[1],
                "status": row[2],
                "percentage": row[3],
                "message": row[4],
                "error_message": row[5],
                "data_ref": row[6],
                "created_at": row[7],
                "updated_at": row[8],
            }
            for row in rows
        ]
