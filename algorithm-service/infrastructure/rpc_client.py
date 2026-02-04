# -*- coding: utf-8 -*-

"""
gRPC client for reporting algorithm task results.
"""

import json
import logging
from pathlib import Path
from typing import Any, Optional

import grpc

from proto import algorithm_pb2, algorithm_pb2_grpc


class ResultReporterClient:
    """gRPC client to report algorithm task results."""

    def __init__(self, target: Optional[str] = None, grpc_stub: Optional[Any] = None) -> None:
        self._target = target
        self._channel = None
        if grpc_stub is not None:
            self._stub = grpc_stub
        elif target:
            options = [
                ("grpc.max_send_message_length", 100 * 1024 * 1024),
                ("grpc.max_receive_message_length", 100 * 1024 * 1024),
            ]
            self._channel = grpc.insecure_channel(target, options=options)
            self._stub = algorithm_pb2_grpc.ResultReceiverServiceStub(self._channel)
        else:
            self._stub = None

    def send_result(self, task_id: str, status: str, data: Any = None, error: Optional[str] = None) -> None:
        """Send the result of an algorithm task to the result receiver service."""

        def _json_safe(obj: Any) -> Any:
            # Convert non-serializable objects to JSON-safe formats

            try:
                import pandas as pd
                if isinstance(obj, pd.Timestamp):
                    return obj.isoformat()
                if isinstance(obj, pd.DataFrame):
                    return obj.to_dict(orient="records")
                if isinstance(obj, pd.Series):
                    return obj.to_list()
            except Exception:
                pass
            try:
                import numpy as np
                if isinstance(obj, (np.integer,)):
                    return int(obj)
                if isinstance(obj, (np.floating,)):
                    return float(obj)
                if isinstance(obj, (np.ndarray,)):
                    return obj.tolist()
            except Exception:
                pass
            return str(obj)

        result_dir = Path(__file__).resolve().parents[1] / "result"
        result_dir.mkdir(parents=True, exist_ok=True)
        result_path = result_dir / f"{task_id}.json"
        result_payload = {
            "task_id": task_id,
            "status": status,
            "data": data,
            "error": error or "",
        }
        try:
            result_path.write_text(
                json.dumps(result_payload, ensure_ascii=False, indent=2, default=_json_safe),
                encoding="utf-8",
            )
        except Exception as exc:
            logging.error("[Reporter] Failed to write result file: %s", exc)

        if status == "SUCCESS":
            result_status = algorithm_pb2.TaskResult.SUCCESS
        elif status == "CANCELLED":
            result_status = algorithm_pb2.TaskResult.CANCELLED
        else:
            result_status = algorithm_pb2.TaskResult.FAILED

        result_json = json.dumps(data, ensure_ascii=False, default=_json_safe) if data is not None else ""

        payload = algorithm_pb2.TaskResult( # type: ignore
            task_id=task_id,
            status=result_status, # type: ignore
            result_json=result_json,
            error_message=error or "",
            log_path=str(result_path),
        )
        logging.info("[Reporter] %s", payload)

        if self._stub is None:
            return

        try:
            self._stub.ReportResult(payload)
        except Exception as exc:
            logging.error("[Reporter] Failed to send result: %s", exc)

