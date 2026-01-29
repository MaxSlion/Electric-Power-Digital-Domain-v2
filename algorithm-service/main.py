# -*- coding: utf-8 -*-

"""
Main entry point for the Algorithm Service gRPC server.
"""

import logging
import os
import signal

from core.plugin_loader import load_plugins
from core.dispatcher import TaskDispatcher
from infrastructure.logging_config import configure_logging
from infrastructure.progress_manager import ProgressManager
from infrastructure.rpc_client import ResultReporterClient
from infrastructure.grpc_server import serve


def bootstrap() -> TaskDispatcher:
    """Bootstrap the Algorithm Service components."""

    configure_logging()
    load_plugins()

    reporter_target = os.getenv("RESULT_REPORTER_TARGET", "")
    reporter = ResultReporterClient(target=reporter_target or None)
    manager = ProgressManager.get_instance()
    manager.bind_result_sink(reporter)
    manager.start_db_writer()
    dispatcher = TaskDispatcher(reporter, reporter_target=reporter_target or None)

    return dispatcher


def main() -> None:
    """Main function to start the gRPC server."""

    host = os.getenv("ALGO_GRPC_HOST", "0.0.0.0")
    port = int(os.getenv("ALGO_GRPC_PORT", "50051"))

    dispatcher = bootstrap()
    server = serve(dispatcher, host=host, port=port)

    def _shutdown(*_: object) -> None:
        """Handle shutdown signals to gracefully stop the server."""

        logging.info("[gRPC] Shutting down...")
        server.stop(2)

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    server.wait_for_termination()


if __name__ == "__main__":
    main()
