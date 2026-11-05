import pytest

MISSING_ID = "00000000-0000-0000-0000-000000000000"


pytestmark = pytest.mark.crud


class TestGroups:
    @pytest.mark.smoke
    def test_list_groups(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/groups")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_group(self, api_client, api_base, random_name):
        group = {"name": random_name, 
                 "provider": "local",
                 "user_ids": [],
                 "role_ids": []}
        resp = api_client.post(f"{api_base}/groups", json=group)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_get_group(self, api_client, api_base, random_name):
        group = {"name": random_name, 
                         "provider": "local",
                         "user_ids": [],
                         "role_ids": []}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/groups/{group_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_group(self, api_client, api_base, random_name):
        group = {"name": random_name, 
                         "provider": "local",
                         "user_ids": [],
                         "role_ids": []}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        update_data = {"name": random_name, 
                         "provider": "local",
                         "user_ids": [],
                         "role_ids": []}
        resp = api_client.patch(f"{api_base}/groups/{group_id}", json=update_data)
        assert resp.status_code == 200

    def test_delete_group(self, api_client, api_base, random_name):
        group = {"name": random_name, 
                         "provider": "local",
                         "user_ids": [],
                         "role_ids": []}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        resp = api_client.delete(f"{api_base}/groups/{group_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/groups/{group_id}")
        assert get_resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_get_group_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/groups/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_delete_group_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/groups/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_group_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/groups")
        assert resp.status_code == 401

    @pytest.mark.smoke
    def test_group_users(self, api_client, api_base, random_name):
        group = {"name": random_name, 
                         "provider": "local",
                         "user_ids": [],
                         "role_ids": []}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        list_resp = api_client.get(f"{api_base}/groups/{group_id}/users")
        assert list_resp.status_code == 200
        assert isinstance(list_resp.json(), list)

    def create_group(self, api_client, api_base, name):
        resp = api_client.post(f"{api_base}/groups", json={
            "name": name,
            "provider": "local",
            "user_ids": [],
            "role_ids": [],
        })
        assert resp.status_code == 200
        return resp.json()["id"]

    def create_user(self, api_client, api_base, name, provider="local"):
        resp = api_client.post(f"{api_base}/users", json={
            "username": name,
            "password": "test-password-123",
            "provider": provider,
        })
        assert resp.status_code == 200
        return resp.json()["id"]

    def create_role(self, api_client, api_base, name):
        resp = api_client.post(f"{api_base}/roles", json={
            "name": name,
            "access": "read",
            "resource": "runners",
            "resource_ids": ["*"],
        })
        assert resp.status_code == 200
        return resp.json()["id"]

    @pytest.mark.crud
    def test_add_and_remove_users_by_id(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        user_id = self.create_user(api_client, api_base, random_name)

        add = api_client.post(f"{api_base}/groups/{group_id}/users", json=[user_id])
        assert add.status_code == 200
        assert user_id in add.json()["user_ids"]

        members = api_client.get(f"{api_base}/groups/{group_id}/users").json()
        assert [m["id"] for m in members] == [user_id]
        assert members[0]["username"] == random_name

        remove = api_client.delete(f"{api_base}/groups/{group_id}/users", json=[user_id])
        assert remove.status_code == 200
        assert user_id not in (remove.json().get("user_ids") or [])

    @pytest.mark.crud
    def test_add_and_remove_roles_by_id(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        role_id = self.create_role(api_client, api_base, random_name)

        add = api_client.post(f"{api_base}/groups/{group_id}/roles", json=[role_id])
        assert add.status_code == 200
        assert role_id in add.json()["role_ids"]

        remove = api_client.delete(f"{api_base}/groups/{group_id}/roles", json=[role_id])
        assert remove.status_code == 200
        assert role_id not in (remove.json().get("role_ids") or [])

    @pytest.mark.negative
    def test_add_user_malformed_id(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        resp = api_client.post(f"{api_base}/groups/{group_id}/users", json=["not-a-uuid"])
        assert resp.status_code == 400

    @pytest.mark.negative
    def test_add_user_unknown_id(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        resp = api_client.post(f"{api_base}/groups/{group_id}/users", json=[MISSING_ID])
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_add_user_provider_mismatch(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        ad_user_id = self.create_user(api_client, api_base, random_name, provider="active_directory")
        resp = api_client.post(f"{api_base}/groups/{group_id}/users", json=[ad_user_id])
        assert resp.status_code == 400

    @pytest.mark.negative
    def test_add_users_rejects_object_body(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        resp = api_client.post(f"{api_base}/groups/{group_id}/users",
                               json=[{"username": random_name, "provider": "local"}])
        assert resp.status_code == 400

    @pytest.mark.negative
    def test_add_role_unknown_id(self, api_client, api_base, random_name):
        group_id = self.create_group(api_client, api_base, random_name)
        resp = api_client.post(f"{api_base}/groups/{group_id}/roles", json=[MISSING_ID])
        assert resp.status_code == 404
