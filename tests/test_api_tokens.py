import pytest


pytestmark = pytest.mark.crud


class TestApiTokens:
    @pytest.mark.smoke
    def test_list_api_tokens(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/api_tokens")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_list_all_api_tokens(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/api_tokens/all")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_api_token(self, api_client, api_base, random_name):
        token_data = {
            "description": f"Test token {random_name}",
            "enabled": True,
        }
        resp = api_client.post(f"{api_base}/api_tokens", json=token_data)
        assert resp.status_code == 200
        data = resp.json()
        assert "id" in data
        assert "key" in data

    def test_get_api_token(self, api_client, api_base, random_name):
        token_data = {
            "description": f"Test token {random_name}",
            "enabled": True,
        }
        create_resp = api_client.post(f"{api_base}/api_tokens", json=token_data)
        assert create_resp.status_code == 200
        token_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/api_tokens/{token_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["description"] == f"Test token {random_name}"

    def test_update_api_token(self, api_client, api_base, random_name):
        token_data = {
            "description": f"Test token {random_name}",
            "enabled": True,
        }
        create_resp = api_client.post(f"{api_base}/api_tokens", json=token_data)
        assert create_resp.status_code == 200
        token_id = create_resp.json()["id"]

        update_data = {
            "description": f"Updated token {random_name}",
            "enabled": False,
        }
        resp = api_client.patch(f"{api_base}/api_tokens/{token_id}", json=update_data)
        assert resp.status_code == 200
        data = resp.json()
        assert data["enabled"] is False

    def test_delete_api_token(self, api_client, api_base, random_name):
        token_data = {
            "description": f"Test token {random_name}",
            "enabled": True,
        }
        create_resp = api_client.post(f"{api_base}/api_tokens", json=token_data)
        assert create_resp.status_code == 200
        token_id = create_resp.json()["id"]

        resp = api_client.delete(f"{api_base}/api_tokens/{token_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/api_tokens/{token_id}")
        assert get_resp.status_code == 404

    @pytest.mark.negative
    def test_get_api_token_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/api_tokens/00000000-0000-0000-0000-000000000000")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_api_token_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/api_tokens/00000000-0000-0000-0000-000000000000")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_api_tokens_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/api_tokens")
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_api_token_unauthorized(self, noauth_client, api_base, random_name):
        token_data = {"description": f"Test {random_name}", "enabled": True}
        resp = noauth_client.post(f"{api_base}/api_tokens", json=token_data)
        assert resp.status_code == 401
