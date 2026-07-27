import pytest


pytestmark = pytest.mark.crud


class TestAuditLogs:
    @pytest.mark.smoke
    def test_list_audit_logs(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/audit_logs")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    def test_get_audit_log(self, api_client, api_base):
        list_resp = api_client.get(f"{api_base}/audit_logs")
        assert list_resp.status_code == 200
        logs = list_resp.json()
        if logs:
            log_id = logs[0]["id"]
            resp = api_client.get(f"{api_base}/audit_logs/{log_id}")
            assert resp.status_code == 200
            data = resp.json()
            assert "id" in data

    @pytest.mark.negative
    def test_get_audit_log_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/audit_logs/00000000-0000-0000-0000-000000000000")
        assert resp.status_code in (404, 500)

    @pytest.mark.negative
    def test_audit_logs_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/audit_logs")
        assert resp.status_code == 401
