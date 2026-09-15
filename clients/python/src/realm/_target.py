"""Resolution of an agent URL into what httpx needs to reach the agent.

The ``url`` of a node in the realm configuration accepts the same forms here:
``http://host:9000``, ``https://host:9000`` and ``unix:///run/realm/agent.sock``.
"""

from __future__ import annotations

from dataclasses import dataclass
from urllib.parse import urlsplit

SOCKET_BASE_URL = "http://realm.local"

_UNIX_EXAMPLE = "unix:///run/realm/agent.sock"


@dataclass(frozen=True)
class Target:
    """Where an agent lives: a base URL and, over a unix socket, its path."""

    base_url: str
    socket: str | None

    @property
    def is_socket(self) -> bool:
        return self.socket is not None


def resolve_target(url: str | None, socket: str | None) -> Target:
    """Resolve the constructor arguments of a client into a :class:`Target`.

    ``socket`` is a shorthand for a ``unix://`` URL; exactly one of the two must
    be given.
    """
    if bool(url) == bool(socket):
        raise ValueError("pass exactly one of url= or socket=")

    if socket is not None:
        return _socket_target(socket)

    assert url is not None  # guaranteed by the check above
    return _url_target(url.strip())


def _url_target(url: str) -> Target:
    parts = urlsplit(url)

    if parts.scheme in ("http", "https"):
        if not parts.netloc:
            raise ValueError(f"invalid agent url '{url}': missing host")
        return Target(base_url=url.rstrip("/"), socket=None)

    if parts.scheme == "unix":
        if parts.netloc:
            raise ValueError(
                f"invalid agent url '{url}': socket path must be absolute, e.g. {_UNIX_EXAMPLE}"
            )
        return _socket_target(parts.path, url=url)

    if not parts.scheme:
        raise ValueError(
            f"invalid agent url '{url}': missing scheme, expected one of http, https or unix"
        )

    raise ValueError(
        f"unsupported agent url scheme '{parts.scheme}', expected one of http, https or unix"
    )


def _socket_target(socket: str, url: str | None = None) -> Target:
    if not socket.startswith("/"):
        subject = f"invalid agent url '{url}'" if url else "invalid socket"
        raise ValueError(f"{subject}: socket path must be absolute, e.g. {_UNIX_EXAMPLE}")
    return Target(base_url=SOCKET_BASE_URL, socket=socket)
