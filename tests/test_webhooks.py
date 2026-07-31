import pytest


pytestmark = pytest.mark.crud


class TestWebhooks:
    @pytest.mark.smoke
    def test_list_webhooks(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/webhooks")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_list_all_webhooks(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/webhooks/all")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_webhook(self, api_client, api_base, random_name):
        webhook = {
            "description": f"Test webhook {random_name}",
            "task": random_name.replace("webhook", "task"),
            "secret": "test-secret-key",
        }
        resp = api_client.post(f"{api_base}/webhooks", json=webhook)
        assert resp.status_code == 200
        data = resp.json()
        assert "id" in data

    def test_get_webhook(self, api_client, api_base, random_name):
        webhook = {
            "description": f"Test webhook {random_name}",
            "task": random_name.replace("webhook", "task"),
        }
        create_resp = api_client.post(f"{api_base}/webhooks", json=webhook)
        assert create_resp.status_code == 200
        webhook_id = create_resp.json()["id"]

        resp = api_client.get(f"{api_base}/webhooks/{webhook_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["id"] == webhook_id

    def test_delete_webhook(self, api_client, api_base, random_name):
        webhook = {
            "description": f"Test webhook {random_name}",
            "task": random_name.replace("webhook", "task"),
        }
        create_resp = api_client.post(f"{api_base}/webhooks", json=webhook)
        assert create_resp.status_code == 200
        webhook_id = create_resp.json()["id"]

        resp = api_client.delete(f"{api_base}/webhooks/{webhook_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/webhooks/{webhook_id}")
        assert get_resp.status_code == 404

    @pytest.mark.negative
    def test_get_webhook_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/webhooks/00000000-0000-0000-0000-000000000000")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_webhook_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/webhooks/00000000-0000-0000-0000-000000000000")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_webhooks_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/webhooks")
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_webhook_unauthorized(self, noauth_client, api_base, random_name):
        webhook = {
            "description": f"Test {random_name}",
            "task": "test-task",
        }
        resp = noauth_client.post(f"{api_base}/webhooks", json=webhook)
        assert resp.status_code == 401

    @pytest.mark.skip(reason="POST /webhooks/run/{id} uses Signature auth, not Bearer")
    def test_run_webhook(self, api_client, api_base, random_name):
        webhook = {
            "description": f"Test {random_name}",
            "task": "test-task",
        }
        create_resp = api_client.post(f"{api_base}/webhooks", json=webhook)
        assert create_resp.status_code == 200
        webhook_id = create_resp.json()["id"]

        resp = api_client.post(f"{api_base}/webhooks/run/{webhook_id}", json={})
        assert resp.status_code == 200
