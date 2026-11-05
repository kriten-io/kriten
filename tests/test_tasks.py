import pytest


pytestmark = pytest.mark.crud

MISSING_ID = "00000000-0000-0000-0000-000000000000"


class TestTasks:
    def create_runner(self, api_client, api_base, random_name):
        resp = api_client.post(f"{api_base}/runners", json={
            "name": f"runner-{random_name}",
            "gitURL": "https://github.com/example/test-repo.git",
            "image": "python:3.11",
        })
        assert resp.status_code == 200
        return resp.json()["id"]

    def setup_task(self, random_name, runner_id):
        return {
            "name": random_name,
            "command": "python -c 'print(\"hello\")'",
            "runner_id": runner_id,
        }

    def create_task(self, api_client, api_base, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task = self.setup_task(random_name, runner_id)
        resp = api_client.post(f"{api_base}/tasks", json=task)
        assert resp.status_code == 200
        return task, resp.json()["id"]

    @pytest.mark.smoke
    def test_list_tasks(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/tasks")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_task(self, api_client, api_base, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task = self.setup_task(random_name, runner_id)
        resp = api_client.post(f"{api_base}/tasks", json=task)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name
        assert data["runner_id"] == runner_id

    def test_get_task(self, api_client, api_base, random_name):
        _, task_id = self.create_task(api_client, api_base, random_name)

        resp = api_client.get(f"{api_base}/tasks/{task_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_task(self, api_client, api_base, random_name):
        task, task_id = self.create_task(api_client, api_base, random_name)

        update_data = {
            "name": random_name,
            "command": "python -c 'print(\"updated\")'",
            "runner_id": task["runner_id"],
        }
        resp = api_client.patch(f"{api_base}/tasks/{task_id}", json=update_data)
        assert resp.status_code == 200
        data = resp.json()
        assert data["command"] == "python -c 'print(\"updated\")'"

    def test_delete_task(self, api_client, api_base, random_name):
        _, task_id = self.create_task(api_client, api_base, random_name)

        resp = api_client.delete(f"{api_base}/tasks/{task_id}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/tasks/{task_id}")
        assert get_resp.status_code == 404

    @pytest.mark.negative
    def test_get_task_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/tasks/{MISSING_ID}")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_get_task_invalid_id(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/tasks/not-a-uuid")
        assert resp.status_code == 400

    @pytest.mark.negative
    def test_update_task_not_found(self, api_client, api_base):
        resp = api_client.patch(
            f"{api_base}/tasks/{MISSING_ID}",
            json={"name": "nonexistent-task", "command": "echo test", "runner_id": MISSING_ID},
        )
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_task_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/tasks/{MISSING_ID}")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_task_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/tasks")
        assert resp.status_code == 401

    @pytest.mark.smoke
    def test_task_schema_crud(self, api_client, api_base, random_name):
        _, task_id = self.create_task(api_client, api_base, random_name)

        schema = {"type": "object", "properties": {"name": {"type": "string"}}}
        post_resp = api_client.post(f"{api_base}/tasks/{task_id}/schema", json=schema)
        assert post_resp.status_code == 200

        get_resp = api_client.get(f"{api_base}/tasks/{task_id}/schema")
        assert get_resp.status_code == 200

        del_resp = api_client.delete(f"{api_base}/tasks/{task_id}/schema")
        assert del_resp.status_code in (200, 204)
