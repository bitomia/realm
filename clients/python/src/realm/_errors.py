"""Errors raised by the client."""

from __future__ import annotations


class RealmError(Exception):
    """A non-200 response from the agent.

    The agent reports failures as a plain-text body (``http.Error``), so the
    body is the message. ``status`` carries the HTTP status code.
    """

    def __init__(self, status: int, body: str) -> None:
        super().__init__(body or f"HTTP {status}")
        self.status = status
        self.body = body


class NodeNotConfigured(RealmError):
    """The agent has no node configuration loaded.

    ``GET /node/config`` answers 204 when nothing is loaded; the client turns
    that into this error so callers do not have to special-case an empty body.
    """


class NodeAlreadyConfigured(RealmError):
    """A node configuration is already loaded (409 from ``POST /node/config``)."""
