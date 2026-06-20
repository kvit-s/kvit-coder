"""csvparse — a small hand-rolled CSV reader.

Public API lives in `csvparse.public`. Import `parse_csv` from there.
"""
from .public import parse_csv

__all__ = ["parse_csv"]
