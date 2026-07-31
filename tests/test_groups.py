import pytest


pytestmark = pytest.mark.crud


class TestGroups:
    @pytest.mark.smoke
    def test_list_groups(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/groups")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_group(self, api_client, api_base, random_name):
        group = {"name": random_name, "provider": "local"}
        resp = api_client.post(f"{api_base}/groups", json=group)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_get_group(self, api_client, api_base, random_name):
        group = {"name": random_name, "provider": "local"}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/groups/{group_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_group(self, api_client, api_base, random_name):
        group = {"name": random_name, "provider": "local"}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        update_data = {"name": random_name, "provider": "local"}
        resp = api_client.patch(f"{api_base}/groups/{group_id}", json=update_data)
        assert resp.status_code == 200

    def test_delete_group(self, api_client, api_base, random_name):
        group = {"name": random_name, "provider": "local"}
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
        group = {"name": random_name, "provider": "local"}
        create_resp = api_client.post(f"{api_base}/groups", json=group)
        assert create_resp.status_code == 200
        group_id = create_resp.json()["id"]

        list_resp = api_client.get(f"{api_base}/groups/{group_id}/users")
        assert list_resp.status_code == 200
        assert isinstance(list_resp.json(), list)
