from dataclasses import dataclass, field
from typing import Any


@dataclass
class HTTPError:
    code: int
    message: str


@dataclass
class Credentials:
    username: str
    password: str
    provider: str = "local"


@dataclass
class Runner:
    name: str
    gitURL: str
    image: str
    branch: str = ""
    secret: dict[str, str] = field(default_factory=dict)
    token: str = ""


@dataclass
class Task:
    name: str
    command: str
    runner: str
    schema: dict[str, Any] = field(default_factory=dict)
    synchronous: bool = False


@dataclass
class Job:
    id: str = ""
    owner: str = ""
    json_data: dict[str, Any] = field(default_factory=dict)
    start_time: str = ""
    completion_time: str = ""
    stdout: str = ""
    completed: int = 0
    failed: int = 0


@dataclass
class CronJob:
    name: str
    task: str
    schedule: str
    owner: str = ""
    extra_vars: dict[str, Any] = field(default_factory=dict)
    disable: bool = False


@dataclass
class User:
    username: str
    password: str
    provider: str = "local"
    id: str = ""
    created_at: str = ""
    updated_at: str = ""
    groups: list[str] = field(default_factory=list)


@dataclass
class Group:
    name: str
    provider: str = "local"
    id: str = ""
    created_at: str = ""
    updated_at: str = ""
    users: list[str] = field(default_factory=list)


@dataclass
class GroupUser:
    id: str
    name: str
    provider: str = "local"


@dataclass
class Role:
    name: str
    access: str
    resource: str
    resource_ids: list[str]
    id: str = ""
    created_at: str = ""
    updated_at: str = ""


@dataclass
class RoleBinding:
    name: str
    role_name: str
    subject_kind: str
    subject_name: str
    subject_provider: str
    id: str = ""
    role_id: str = ""
    subject_id: str = ""
    created_at: str = ""
    updated_at: str = ""


@dataclass
class ApiToken:
    description: str = ""
    enabled: bool = True
    expires: str = ""
    id: str = ""
    key: str = ""
    owner: str = ""
    created_at: str = ""
    updated_at: str = ""


@dataclass
class Webhook:
    description: str = ""
    id: str = ""
    owner: str = ""
    secret: str = ""
    task: str = ""
    created_at: str = ""
    updated_at: str = ""


@dataclass
class AuditLog:
    id: str = ""
    event_category: str = ""
    event_target: str = ""
    event_type: str = ""
    status: str = ""
    provider: str = ""
    user_id: str = ""
    username: str = ""
    created_at: str = ""
    updated_at: str = ""
