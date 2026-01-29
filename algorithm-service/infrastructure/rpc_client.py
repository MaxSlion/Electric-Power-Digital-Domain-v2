# -*- coding: utf-8 -*-

"""
gRPC client for reporting algorithm task results.
"""

import json
import logging
from pathlib import Path
from typing import Optional

import grpc

from proto import algorithm_pb2, algorithm_pb2_grpc


class ResultReporterClient:
    """gRPC client to report algorithm task results."""

    def __init__(self, target: Optional[str] = None, grpc_stub=None):
        self._target = target
        self._channel = None
        if grpc_stub is not None:
            self._stub = grpc_stub
        elif target:
            self._channel = grpc.insecure_channel(target)
            self._stub = algorithm_pb2_grpc.ResultReceiverServiceStub(self._channel)
        else:
            self._stub = None

    def send_result(self, task_id, status, data=None, error=None):
        """Send the result of an algorithm task to the result receiver service."""

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
            result_path.write_text(json.dumps(result_payload, ensure_ascii=False, indent=2), encoding="utf-8")
        except Exception as exc:
            logging.error("[Reporter] Failed to write result file: %s", exc)

        payload = algorithm_pb2.TaskResult( # type: ignore
            task_id=task_id,
            status=algorithm_pb2.TaskResult.SUCCESS if status == "SUCCESS" else algorithm_pb2.TaskResult.FAILED, # type: ignore
            result_json=json.dumps(data, ensure_ascii=False) if data is not None else "",
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

