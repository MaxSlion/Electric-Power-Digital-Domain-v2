# -*- coding: utf-8 -*-

"""
Centralized data loader for resolving data_ref into in-memory objects.
"""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path
from typing import Any, Dict, Tuple
from urllib.parse import parse_qs, urlparse, unquote

import pandas as pd


def load_data_ref(data_ref: str | None) -> Tuple[Any, Dict[str, Any]]:
    """
    Resolve data_ref into an in-memory object.

        Supported:
            - File path: CSV/JSON/Parquet/Excel
            - MySQL / Redis via URL
        Returns pandas.DataFrame only; otherwise raises.
    """

    meta: Dict[str, Any] = {}

    data_ref = data_ref or ""

    parsed = urlparse(data_ref)
    scheme = parsed.scheme.lower()

    if scheme in ("mysql", "mysql+pymysql"):
        query_args = parse_qs(parsed.query)
        table = _first(query_args.get("table"))
        sql = _first(query_args.get("query"))
        if not table and not sql:
            raise ValueError("mysql data_ref requires ?table= or ?query=")

        try:
            from sqlalchemy import create_engine  # type: ignore
        except Exception as exc:
            raise ImportError("sqlalchemy is required for mysql data_ref") from exc

        db = parsed.path.lstrip("/")
        netloc = parsed.netloc
        conn_url = f"{scheme}://{netloc}/{db}"
        engine = create_engine(conn_url)
        sql = sql or f"SELECT * FROM {table}"
        df = pd.read_sql_query(sql, engine)
        meta.update({"source": "mysql", "database": db, "table": table, "query": sql})
        return _ensure_df(df, "mysql"), meta

    if scheme == "redis":
        query_args = parse_qs(parsed.query)
        key = _first(query_args.get("key"))
        value_type = (_first(query_args.get("type")) or "string").lower()
        if not key:
            raise ValueError("redis data_ref requires ?key=")
        try:
            import redis  # type: ignore
        except Exception as exc:
            raise ImportError("redis package is required for redis data_ref") from exc

        host = parsed.hostname or "localhost"
        port = parsed.port or 6379
        db = int(parsed.path.lstrip("/") or 0)
        client = redis.Redis(host=host, port=port, db=db, decode_responses=True)

        if value_type == "hash":
            payload = client.hgetall(key)
        elif value_type == "list":
            payload = client.lrange(key, 0, -1)
        else:
            payload = client.get(key)
        payload = _try_json(payload)
        meta.update({"source": "redis", "host": host, "port": port, "db": db, "key": key, "type": value_type})
        df = _ensure_df(payload, "redis")
        return df, meta

    if scheme == "file":
        path = Path(unquote(parsed.path)).expanduser()
    else:
        path = Path(data_ref).expanduser()
    if not path.is_absolute():
        path = (Path.cwd() / path).resolve()
    if not path.exists():
        raise FileNotFoundError(f"data file not found: {path}")

    suffix = path.suffix.lower()
    if suffix in (".csv",):
        df = pd.read_csv(path)
        meta.update({"source": "file", "path": str(path), "format": "csv"})
        return _ensure_df(df, "file") , meta
    if suffix in (".parquet",):
        df = pd.read_parquet(path)
        meta.update({"source": "file", "path": str(path), "format": "parquet"})
        return _ensure_df(df, "file"), meta
    if suffix in (".xlsx", ".xls"):
        df = pd.read_excel(path)
        meta.update({"source": "file", "path": str(path), "format": "excel"})
        return _ensure_df(df, "file"), meta
    if suffix in (".json",):
        with open(path, "r", encoding="utf-8") as f:
            payload = json.load(f)
        df = _ensure_df(payload, "json")
        meta.update({"source": "file", "path": str(path), "format": "json"})
        return df, meta

    raise ValueError(f"Unsupported data file type: {suffix}")


def _first(values):
    return values[0] if values else None


def _try_json(payload):
    if payload is None:
        return None
    if isinstance(payload, (dict, list)):
        return payload
    if isinstance(payload, bytes):
        payload = payload.decode("utf-8")
    if isinstance(payload, str):
        try:
            return json.loads(payload)
        except Exception:
            return payload
    return payload


def _ensure_df(obj: Any, source: str) -> pd.DataFrame:
    if isinstance(obj, pd.DataFrame):
        return obj
    if isinstance(obj, dict):
        return pd.DataFrame([obj])
    if isinstance(obj, list):
        return pd.DataFrame(obj)
    raise ValueError(f"{source} data_ref must resolve to pandas.DataFrame")
