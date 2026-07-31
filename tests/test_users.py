import pytest


pytestmark = pytest.mark.crud


class TestUsers:
    @pytest.mark.smoke
    def test_list_users(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/users")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_user(self, api_client, api_base, random_name):
        user = {
            "username": random_name,
            "password": "test-password-123",
            "provider": "local",
        }
        resp = api_client.post(f"{api_base}/users", json=user)
        assert resp.status_code == 200
        data = resp.json()
        assert data["username"] == random_name

    def test_get_user(self, api_client, api_base, random_name):
        user = {
            "username": random_name,
            "password": "test-password-123",
            "provider": "local",
        }
        create_resp = api_client.post(f"{api_base}/users", json=user)
        assert create_resp.status_code == 200
        user_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/users/{user_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["username"] == random_name

    def test_update_user(self, api_client, api_base, random_name):
        user = {
            "username": random_name,
            "password": "test-password-123",
            "provider": "local",
        }
        create_resp = api_client.post(f"{api_base}/users", json=user)
        assert create_resp.status_code == 200
        user_id = create_resp.json()["id"]

        update_data = {
            "username": random_name,
            "password": "new-password-456",
            "provider": "local",
        }
        resp = api_client.patch(f"{api_base}/users/{user_id}", json=update_data)
        assert resp.status_code == 200

    def test_delete_user(self, api_client, api_base, random_name):
        user = {
            "username": random_name,
            "password": "test-password-123",
            "provider": "local",
        }
        create_resp = api_client.post(f"{api_base}/users", json=user)
        assert create_resp.status_code == 200
        user_id = create_resp.json()["id"]

        resp = api_client.delete(f"{api_base}/users/{user_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/users/{user_id}")
        assert get_resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_get_user_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/users/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_delete_user_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/users/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_user_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/users")
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_user_unauthorized(self, noauth_client, api_base, random_name):
        user = {
            "username": random_name,
            "password": "test-password-123",
            "provider": "local",
        }
        resp = noauth_client.post(f"{api_base}/users", json=user)
        assert resp.status_code == 401

    @pytest.mark.smoke
    def test_get_user_groups(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/users")
        assert resp.status_code == 200
        users = resp.json()
        if users:
            user_id = users[0]["id"]
            groups_resp = api_client.get(f"{api_base}/users/{user_id}/groups")
            assert groups_resp.status_code == 200
