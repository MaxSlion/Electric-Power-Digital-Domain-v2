# -*- coding: utf-8 -*-

"""
gRPC server implementation for algorithm control service.
"""

import json
import logging

import grpc
from concurrent import futures

from core.framework import AlgorithmRegistry
from core.resource_manager import HardwareManager
from infrastructure.progress_manager import ProgressManager
from infrastructure.task_store import TaskStore
from proto import algorithm_pb2, algorithm_pb2_grpc


class AlgoControlService(algorithm_pb2_grpc.AlgoControlServiceServicer):
    """gRPC service for algorithm control."""

    def __init__(self, dispatcher):
        self.dispatcher = dispatcher
        self.hardware = HardwareManager()

    def GetAvailableSchemes(self, request, context):
        """Returns a list of available algorithm schemes."""

        schemes = [
            algorithm_pb2.SchemeList.Scheme( # type: ignore
                code=meta["code"],
                name=meta["name"],
                resource_type=meta.get("resource_type", "CPU"),
                model=meta.get("model", ""),
                class_name=meta.get("class", ""),
            )
            for meta in AlgorithmRegistry.get_all_schemes()
        ]
        return algorithm_pb2.SchemeList(schemes=schemes) # type: ignore

    def SubmitTask(self, request, context):
        """Submits a new algorithm task for processing."""

        try:
            params = json.loads(request.params_json) if request.params_json else {}
        except json.JSONDecodeError:
            params = {}
        self.dispatcher.dispatch(request.task_id, request.scheme_code, request.data_ref, params)
        return algorithm_pb2.TaskSubmissionResponse(accepted=True, message="Task accepted") # type: ignore

    def CheckHealth(self, request, context):
        """Checks the health status of the service."""

        metrics = {
            "device": self.hardware.device_info,
            "gpu": "available" if self.hardware.has_gpu else "none",
        }
        return algorithm_pb2.HealthStatus(status=algorithm_pb2.HealthStatus.SERVING, metrics=metrics) # type: ignore

    def WatchTaskProgress(self, request, context):
        """Streams progress updates for a given task."""

        manager = ProgressManager.get_instance()
        q = manager.register_watcher(request.task_id)
        try:
            while True:
                data = q.get(timeout=60)
                yield algorithm_pb2.ProgressUpdate(**data) # type: ignore
                status = manager.get_task(request.task_id).get("status")
                if data["percentage"] >= 100 or status in ("SUCCESS", "FAILED"):
                    break
        except Exception:
            logging.info("[Progress] Watcher closed for %s", request.task_id)
        finally:
            manager.close_watcher(request.task_id)

    def ListTasks(self, request, context):
        """Returns a list of known tasks and their last status."""

        tasks = [
            algorithm_pb2.TaskStatus( # type: ignore
                task_id=item.get("task_id", ""),
                scheme_code=item.get("scheme_code", ""),
                status=item.get("status", ""),
                percentage=int(item.get("percentage", 0)),
                message=item.get("message", ""),
                error_message=item.get("error_message", ""),
                updated_at=int(item.get("updated_at", 0)),
            )
            for item in TaskStore().list_tasks()
        ]
        return algorithm_pb2.TaskList(tasks=tasks) # type: ignore

    def GetTaskStatus(self, request, context):
        """Returns the status of a specific task from the task store."""

        item = TaskStore().get_task(request.task_id)
        return algorithm_pb2.TaskStatus( # type: ignore
            task_id=item.get("task_id", ""),
            scheme_code=item.get("scheme_code", ""),
            status=item.get("status", ""),
            percentage=int(item.get("percentage", 0)),
            message=item.get("message", ""),
            error_message=item.get("error_message", ""),
            updated_at=int(item.get("updated_at", 0)),
        )


def serve(dispatcher, host: str = "0.0.0.0", port: int = 50051):
    """Starts the gRPC server for the AlgoControlService."""

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    algorithm_pb2_grpc.add_AlgoControlServiceServicer_to_server(AlgoControlService(dispatcher), server)
    server.add_insecure_port(f"{host}:{port}")
    server.start()
    logging.info("[gRPC] AlgoControlService started on %s:%s", host, port)
    return server
