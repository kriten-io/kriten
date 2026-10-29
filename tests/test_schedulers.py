import pytest


pytestmark = pytest.mark.crud

MISSING_ID = "00000000-0000-0000-0000-00000000abfe"

SCHEMA = {
    "type": "object",
    "required": ["env"],
    "properties": {"env": {"type": "string"}},
}


class TestSchedulers:
    def create_task(self, api_client, api_base, random_name, schema=None):
        runner_resp = api_client.post(f"{api_base}/runners", json={
            "name": f"runner-{random_name}",
            "gitURL": "https://github.com/example/test-repo.git",
            "image": "python:3.11",
        })
        assert runner_resp.status_code == 200
        runner_id = runner_resp.json()["id"]

        task = {
            "name": random_name,
            "command": "python -c 'print(\"hello\")'",
            "runner_id": runner_id,
        }
        if schema is not None:
            task["schema"] = schema
        task_resp = api_client.post(f"{api_base}/tasks", json=task)
        assert task_resp.status_code == 200
        return task_resp.json()["id"]

    def scheduler_payload(self, task_id, random_name, **overrides):
        payload = {
            "name": random_name,
            "description": f"Test scheduler {random_name}",
            "task_id": task_id,
            "cron_expr": "*/5 * * * *",
            "extra_vars": {},
        }
        payload.update(overrides)
        return payload

    @pytest.mark.smoke
    def test_list_schedulers(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/schedulers")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_scheduler(self, api_client, api_base, random_name):
        task_id = self.create_task(api_client, api_base, random_name)
        resp = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name),
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["task_id"] == task_id
        assert data["enabled"] is True
        assert data["next_run_at"]
        assert data["owner"]

    @pytest.mark.smoke
    def test_get_update_delete_scheduler(self, api_client, api_base, random_name):
        task_id = self.create_task(api_client, api_base, random_name)
        created = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name),
        ).json()
        scheduler_id = created["id"]

        resp = api_client.get(f"{api_base}/schedulers/{scheduler_id}")
        assert resp.status_code == 200
        assert resp.json()["name"] == random_name

        resp = api_client.put(
            f"{api_base}/schedulers/{scheduler_id}",
            json=self.scheduler_payload(task_id, random_name, cron_expr="0 * * * *"),
        )
        assert resp.status_code == 200
        assert resp.json()["cron_expr"] == "0 * * * *"

        resp = api_client.delete(f"{api_base}/schedulers/{scheduler_id}")
        assert resp.status_code == 200

        resp = api_client.get(f"{api_base}/schedulers/{scheduler_id}")
        assert resp.status_code == 404

    @pytest.mark.smoke
    def test_enable_disable_scheduler(self, api_client, api_base, random_name):
        task_id = self.create_task(api_client, api_base, random_name)
        scheduler_id = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name),
        ).json()["id"]

        resp = api_client.post(f"{api_base}/schedulers/{scheduler_id}/disable")
        assert resp.status_code == 200
        assert resp.json()["enabled"] is False
        assert resp.json().get("next_run_at") is None

        resp = api_client.post(f"{api_base}/schedulers/{scheduler_id}/enable")
        assert resp.status_code == 200
        assert resp.json()["enabled"] is True
        assert resp.json()["next_run_at"]

        api_client.delete(f"{api_base}/schedulers/{scheduler_id}")

    @pytest.mark.negative
    def test_create_scheduler_invalid_cron(self, api_client, api_base, random_name):
        task_id = self.create_task(api_client, api_base, random_name)
        resp = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name, cron_expr="not a cron"),
        )
        assert resp.status_code == 400

    @pytest.mark.negative
    def test_create_scheduler_unknown_task(self, api_client, api_base, random_name):
        resp = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(MISSING_ID, random_name),
        )
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_create_scheduler_extra_vars_fail_schema(self, api_client, api_base, random_name):
        task_id = self.create_task(api_client, api_base, random_name, schema=SCHEMA)
        resp = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name, extra_vars={}),
        )
        assert resp.status_code == 400

        resp = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name, extra_vars={"env": "prod"}),
        )
        assert resp.status_code == 200
        api_client.delete(f"{api_base}/schedulers/{resp.json()['id']}")

    @pytest.mark.negative
    def test_get_unknown_scheduler(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/schedulers/{MISSING_ID}")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_task_with_scheduler_blocked(self, api_client, api_base, random_name):
        task_id = self.create_task(api_client, api_base, random_name)
        scheduler_id = api_client.post(
            f"{api_base}/schedulers",
            json=self.scheduler_payload(task_id, random_name),
        ).json()["id"]

        resp = api_client.delete(f"{api_base}/tasks/{task_id}")
        assert resp.status_code == 409

        api_client.delete(f"{api_base}/schedulers/{scheduler_id}")
        resp = api_client.delete(f"{api_base}/tasks/{task_id}")
        assert resp.status_code == 200
