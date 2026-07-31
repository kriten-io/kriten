import pytest


pytestmark = pytest.mark.crud


class TestCronJobs:
    def setup_cronjob(self, random_name):
        return {
            "name": random_name,
            "task": f'task-{random_name}',
            "schedule": "*/5 * * * *",
            "extra_vars": {},
            "disable": False,
        }

    @pytest.mark.smoke
    def test_list_cronjobs(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/cronjobs")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_cronjob(self, api_client, api_base, random_name):
        cronjob = self.setup_cronjob(random_name)

        runner_name = cronjob["task"].replace("task", "runner")
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json={
            "name": cronjob["task"],
            "command": "echo hello",
            "runner": runner_name,
        })

        resp = api_client.post(f"{api_base}/cronjobs", json=cronjob)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_get_cronjob(self, api_client, api_base, random_name):
        cronjob = self.setup_cronjob(random_name)

        runner_name = cronjob["task"].replace("task", "runner")
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json={
            "name": cronjob["task"],
            "command": "echo hello",
            "runner": runner_name,
        })
        create_resp = api_client.post(f"{api_base}/cronjobs", json=cronjob)
        assert create_resp.status_code == 200

        resp = api_client.get(f"{api_base}/cronjobs/{random_name}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_cronjob(self, api_client, api_base, random_name):
        cronjob = self.setup_cronjob(random_name)

        runner_name = cronjob["task"].replace("task", "runner")
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json={
            "name": cronjob["task"],
            "command": "echo hello",
            "runner": runner_name,
        })
        create_resp = api_client.post(f"{api_base}/cronjobs", json=cronjob)
        assert create_resp.status_code == 200

        update_data = {
            "name": random_name,
            "task": cronjob["task"],
            "schedule": "*/10 * * * *",
            "disable": True,
        }
        resp = api_client.patch(f"{api_base}/cronjobs/{random_name}", json=update_data)
        assert resp.status_code == 200
        data = resp.json()
        assert data.get("disable") is True or data.get("schedule") == "*/10 * * * *"

    def test_delete_cronjob(self, api_client, api_base, random_name):
        cronjob = self.setup_cronjob(random_name)

        runner_name = cronjob["task"].replace("task", "runner")
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json={
            "name": cronjob["task"],
            "command": "echo hello",
            "runner": runner_name,
        })
        create_resp = api_client.post(f"{api_base}/cronjobs", json=cronjob)
        assert create_resp.status_code == 200

        resp = api_client.delete(f"{api_base}/cronjobs/{random_name}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/cronjobs/{random_name}")
        assert get_resp.status_code == 404

    @pytest.mark.negative
    def test_get_cronjob_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/cronjobs/nonexistent-cronjob")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_update_cronjob_not_found(self, api_client, api_base):
        resp = api_client.patch(
            f"{api_base}/cronjobs/nonexistent-cronjob",
            json={"name": "nonexistent-cronjob", "task": "test-task", "schedule": "*/5 * * * *"},
        )
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_cronjob_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/cronjobs/nonexistent-cronjob")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_cronjob_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/cronjobs")
        assert resp.status_code == 401
    
