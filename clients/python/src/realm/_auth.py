"""Bearer token resolution, mirroring the Go client (cmd/client/client.go)."""

from __future__ import annotations

import os
from pathlib import Path

REALMRC = ".realmrc"


def resolve_token() -> str | None:
    """Return the bearer token, or None when the agent runs without auth.

    Looked up the same way the ``realm`` CLI does: the ``REALM_BEARER``
    environment variable first, then ``~/.realmrc``.
    """
    token = os.environ.get("REALM_BEARER")
    if token:
        return token

    try:
        rc = Path.home() / REALMRC
        return rc.read_text().strip() or None
    except OSError:
        # No home directory, unreadable file, or no token file at all.
        return None
