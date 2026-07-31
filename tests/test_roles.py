import pytest


pytestmark = pytest.mark.crud


class TestRoles:
    def setup_runner(self, random_name):
        return {
            "name": random_name,
            "gitURL": "https://github.com/kriten-io/kriten-examples.git",
            "image": "python:3.11",
            "branch": "main",
            "token": "",
        }
    @pytest.mark.smoke
    def test_list_roles(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/roles")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_role(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200
        role = {
            "name": random_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        }
        resp = api_client.post(f"{api_base}/roles", json=role)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_get_role(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200
        role = {
            "name": random_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        }
        create_resp = api_client.post(f"{api_base}/roles", json=role)
        assert create_resp.status_code == 200
        role_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/roles/{role_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_role(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200
        role = {
            "name": random_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        }
        create_resp = api_client.post(f"{api_base}/roles", json=role)
        assert create_resp.status_code == 200
        role_id = create_resp.json()["id"]

        update_data = {
            "name": random_name,
            "access": "write",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        }
        resp = api_client.patch(f"{api_base}/roles/{role_id}", json=update_data)
        assert resp.status_code == 200
        data = resp.json()
        assert data["access"] == "write"

    def test_delete_role(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200
        role = {
            "name": random_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        }
        create_resp = api_client.post(f"{api_base}/roles", json=role)
        assert create_resp.status_code == 200
        role_id = create_resp.json()["id"]

        resp = api_client.delete(f"{api_base}/roles/{role_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/roles/{role_id}")
        assert get_resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_get_role_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/roles/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_update_role_not_found(self, api_client, api_base):
        resp = api_client.patch(
            f"{api_base}/roles/00000000-0000-0000-0000-000000000000",
            json={"name": "test", "access": "read", "resource": "runners", "resource_ids": ["*"]},
        )
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_delete_role_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/roles/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_role_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/roles")
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_role_unauthorized(self, noauth_client, api_base, random_name):
        role = {
            "name": random_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": ["*"],
        }
        resp = noauth_client.post(f"{api_base}/roles", json=role)
        assert resp.status_code == 401
