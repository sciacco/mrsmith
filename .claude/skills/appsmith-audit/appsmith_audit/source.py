from __future__ import annotations

import json
import zipfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, Iterable, Optional, Tuple


def normalize_path(path: str) -> str:
    return path.replace("\\", "/")


def iter_source_files(source: Path) -> Iterable[Tuple[str, bytes]]:
    if source.is_file() and source.suffix.lower() == ".zip":
        with zipfile.ZipFile(source, "r") as zf:
            for name in zf.namelist():
                if not name.endswith("/"):
                    yield normalize_path(name), zf.read(name)
    elif source.is_dir():
        for path in source.rglob("*"):
            if path.is_file():
                yield normalize_path(str(path.relative_to(source))), path.read_bytes()
    else:
        raise FileNotFoundError(f"Unsupported source: {source}")


def safe_load_json(raw: bytes) -> Optional[Dict[str, object]]:
    try:
        return json.loads(raw.decode("utf-8"))
    except Exception:
        return None


@dataclass
class SourceBundle:
    source: Path
    files: Dict[str, bytes] = field(default_factory=dict)

    def text(self, path: str) -> str:
        raw = self.files.get(path, b"")
        return raw.decode("utf-8", errors="ignore")

    def json(self, path: str) -> Optional[Dict[str, object]]:
        raw = self.files.get(path)
        return safe_load_json(raw) if raw is not None else None


def load_source(source: Path) -> SourceBundle:
    return SourceBundle(source=source, files=dict(iter_source_files(source)))
