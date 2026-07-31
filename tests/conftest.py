import os
import uuid
import pytest
import requests


@pytest.fixture(scope="session")
def base_url():
    return os.getenv("KRITEN_URL", "http://localhost:8080")


@pytest.fixture(scope="session")
def api_base(base_url):
    return f"{base_url}/api/v1"


@pytest.fixture(scope="session")
def username():
    return os.getenv("KRITEN_USERNAME", "root")


@pytest.fixture(scope="session")
def password():
    return os.getenv("KRITEN_PASSWORD", "")


@pytest.fixture(scope="session")
def provider():
    return os.getenv("KRITEN_PROVIDER", "local")


@pytest.fixture(scope="session")
def jwt_token(api_base, username, password, provider):
    if not password:
        pytest.skip("KRITEN_PASSWORD environment variable not set")
    resp = requests.post(
        f"{api_base}/login",
        json={"username": username, "password": password, "provider": provider},
    )
    assert resp.status_code == 200, (
        f"Login failed: {resp.status_code} {resp.text}"
    )
    data = resp.json()
    return data["token"]


@pytest.fixture(scope="session")
def api_client(jwt_token):
    session = requests.Session()
    session.headers.update({
        "Authorization": f"Bearer {jwt_token}",
        "Content-Type": "application/json",
    })
    return session


@pytest.fixture(scope="session")
def noauth_client():
    session = requests.Session()
    session.headers.update({"Content-Type": "application/json"})
    return session


@pytest.fixture
def random_name():
    return f"pytest-{uuid.uuid4().hex[:8]}"


def assert_error_response(resp, expected_status=None):
    data = resp.json()
    assert "error" in data or "message" in data or "code" in data
    if expected_status:
        assert resp.status_code == expected_status
