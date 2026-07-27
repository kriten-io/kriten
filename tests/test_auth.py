import pytest


@pytest.mark.smoke
class TestAuth:
    def test_login_success(self, api_base, username, password, provider):
        resp = pytest.importorskip("requests").post(
            f"{api_base}/login",
            json={"username": username, "password": password, "provider": provider},
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "token" in data
        assert len(data["token"]) > 0

    def test_login_invalid_password(self, api_base, username, provider):
        resp = pytest.importorskip("requests").post(
            f"{api_base}/login",
            json={"username": username, "password": "wrongpassword", "provider": provider},
        )
        assert resp.status_code == 401
        assert "error" in resp.json()

    def test_login_missing_fields(self, api_base):
        resp = pytest.importorskip("requests").post(
            f"{api_base}/login",
            json={},
        )
        assert resp.status_code == 400
        assert "error" in resp.json()

    def test_login_invalid_provider(self, api_base, username, password):
        resp = pytest.importorskip("requests").post(
            f"{api_base}/login",
            json={"username": username, "password": password, "provider": "nonexistent"},
        )
        assert resp.status_code == 400
        data = resp.json()
        assert "error" in data or "providers" in data

    @pytest.mark.negative
    def test_refresh_without_auth(self, api_base, noauth_client):
        resp = noauth_client.get(f"{api_base}/refresh")
        assert resp.status_code == 400 or resp.status_code == 401

    @pytest.mark.smoke
    def test_refresh_with_token(self, api_base, jwt_token):
        resp = pytest.importorskip("requests").get(
            f"{api_base}/refresh",
            headers={"Authorization": f"Bearer {jwt_token}"},
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "token" in data
