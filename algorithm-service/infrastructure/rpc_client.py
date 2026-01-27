# -*- coding: utf-8 -*-

"""
gRPC client for reporting algorithm task results.
"""

import json
import logging
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

        payload = algorithm_pb2.TaskResult(
            task_id=task_id,
            status=algorithm_pb2.TaskResult.SUCCESS if status == "SUCCESS" else algorithm_pb2.TaskResult.FAILED,
            result_json=json.dumps(data, ensure_ascii=False) if data is not None else "",
            error_message=error or "",
            log_path="",
        )
        logging.info("[Reporter] %s", payload)

        if self._stub is None:
            return
        try:
            self._stub.ReportResult(payload)
        except Exception as exc:
            logging.error("[Reporter] Failed to send result: %s", exc)

