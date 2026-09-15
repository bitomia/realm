"""Python client for the Realm agent API.

    from realm import Realm

    with Realm(socket="/run/realm/agent.sock") as agent:
        print(agent.version())
        for deployment in agent.loads():
            print(deployment["load_name"], deployment["deployment_status"]["status"])

Responses are the agent's JSON, decoded into plain dicts and lists.
"""

from __future__ import annotations

from ._client import AsyncRealm, Realm
from ._errors import NodeAlreadyConfigured, NodeNotConfigured, RealmError

__version__ = "0.1.0"

__all__ = [
    "AsyncRealm",
    "NodeAlreadyConfigured",
    "NodeNotConfigured",
    "Realm",
    "RealmError",
    "__version__",
]
