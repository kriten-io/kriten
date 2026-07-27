import pytest


pytestmark = pytest.mark.crud


class TestRoleBindings:
    def setup_runner(self, random_name):
        return {
            "name": random_name,
            "gitURL": "https://github.com/kriten-io/kriten-examples.git",
            "image": "python:3.11",
            "branch": "main",
            "token": "",
        }
    @pytest.mark.smoke
    def test_list_role_bindings(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/role_bindings")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_role_binding(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200

        role_name = f"{random_name}-role"
        resp = api_client.post(f"{api_base}/roles", json={
            "name": role_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        })
        assert resp.status_code == 200
        data = resp.json()
        role_id = data["id"]

        group = {"name": random_name, "provider": "local"}
        resp = api_client.post(f"{api_base}/groups", json=group)
        assert resp.status_code == 200
        data = resp.json()
        group_id = data["id"]

        binding = {
            "name": random_name,
            "role_id": role_id,
            "group_id": group_id,
        }

        resp = api_client.post(f"{api_base}/role_bindings", json=binding)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_get_role_binding(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200

        role_name = f"{random_name}-role"
        resp = api_client.post(f"{api_base}/roles", json={
            "name": role_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        })
        assert resp.status_code == 200
        data = resp.json()
        role_id = data["id"]

        group = {"name": random_name, "provider": "local"}
        resp = api_client.post(f"{api_base}/groups", json=group)
        assert resp.status_code == 200
        data = resp.json()
        group_id = data["id"]

        binding = {
            "name": random_name,
            "role_id": role_id,
            "group_id": group_id,
        }
        create_resp = api_client.post(f"{api_base}/role_bindings", json=binding)
        assert create_resp.status_code == 200
        binding_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/role_bindings/{binding_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_role_binding(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200

        role_name = f"{random_name}-role"
        resp = api_client.post(f"{api_base}/roles", json={
            "name": role_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        })
        assert resp.status_code == 200
        data = resp.json()
        role_id = data["id"]

        group = {"name": random_name, "provider": "local"}
        resp = api_client.post(f"{api_base}/groups", json=group)
        assert resp.status_code == 200
        data = resp.json()
        group_id = data["id"]

        binding = {
            "name": random_name,
            "role_id": role_id,
            "group_id": group_id,
        }

        create_resp = api_client.post(f"{api_base}/role_bindings", json=binding)
        assert create_resp.status_code == 200
        binding_id = create_resp.json()["id"]

        update_data = {
            "name": random_name,
            "role_id": role_id,
            "group_id": group_id,
        }
        resp = api_client.patch(f"{api_base}/role_bindings/{binding_id}", json=update_data)
        assert resp.status_code == 200

    def test_delete_role_binding(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = api_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 200

        role_name = f"{random_name}-role"
        resp = api_client.post(f"{api_base}/roles", json={
            "name": role_name,
            "access": "read",
            "resource": "runners",
            "resource_ids": [f"{runner["name"]}"],
        })
        assert resp.status_code == 200
        data = resp.json()
        role_id = data["id"]

        group = {"name": random_name, "provider": "local"}
        resp = api_client.post(f"{api_base}/groups", json=group)
        assert resp.status_code == 200
        data = resp.json()
        group_id = data["id"]

        binding = {
            "name": random_name,
            "role_id": role_id,
            "group_id": group_id,
        }
        create_resp = api_client.post(f"{api_base}/role_bindings", json=binding)
        assert create_resp.status_code == 200
        binding_id = create_resp.json()["id"]

        resp = api_client.delete(f"{api_base}/role_bindings/{binding_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/role_bindings/{binding_id}")
        assert get_resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_get_role_binding_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/role_bindings/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_delete_role_binding_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/role_bindings/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_role_binding_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/role_bindings")
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_role_binding_unauthorized(self, noauth_client, api_base, random_name):
        binding = {
            "name": random_name,
            "role_id": "00000000-0000-0000-0000-000000000000",
            "group_id": "00000000-0000-0000-0000-000000000000",
        }
        resp = noauth_client.post(f"{api_base}/role_bindings", json=binding)
        assert resp.status_code == 401
