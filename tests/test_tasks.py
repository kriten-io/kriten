import pytest
import json


pytestmark = pytest.mark.crud


class TestTasks:
    def setup_task(self, random_name):
        return {
            "name": random_name,
            "command": "python -c 'print(\"hello\")'",
            "runner": f'runner-{random_name}',
            #"synchronous": False,
        }

    @pytest.mark.smoke
    def test_list_tasks(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/tasks")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_create_task(self, api_client, api_base, random_name, request):
        task = self.setup_task(random_name)

        runner_name = task["runner"]
        runner_payload = {
            "name": runner_name,
            "gitURL": "https://github.com/example/test-repo.git",
            "image": "python:3.11",
        }
        api_client.post(f"{api_base}/runners", json=runner_payload)
        resp = api_client.post(f"{api_base}/tasks", json=task)
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_get_task(self, api_client, api_base, random_name):
        task = self.setup_task(random_name)

        runner_name = task["runner"]
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json=task)

        resp = api_client.get(f"{api_base}/tasks/{random_name}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name

    def test_update_task(self, api_client, api_base, random_name):
        task = self.setup_task(random_name)

        runner_name = task["runner"]
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json=task)

        update_data = {
            "name": random_name,
            "command": "python -c 'print(\"updated\")'",
            "runner": runner_name,
        }
        resp = api_client.patch(f"{api_base}/tasks/{random_name}", json=update_data)
        assert resp.status_code == 200
        data = resp.json()
        assert data["command"] == "python -c 'print(\"updated\")'"

    def test_delete_task(self, api_client, api_base, random_name):
        task = self.setup_task(random_name)

        runner_name = task["runner"]
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json=task)

        resp = api_client.delete(f"{api_base}/tasks/{random_name}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/tasks/{random_name}")
        assert get_resp.status_code == 404

    @pytest.mark.negative
    def test_get_task_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/tasks/nonexistent-task")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_update_task_not_found(self, api_client, api_base):
        resp = api_client.patch(
            f"{api_base}/tasks/nonexistent-task",
            json={"name": "nonexistent-task", "command": "echo test", "runner": "test-runner"},
        )
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_task_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/tasks/nonexistent-task")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_task_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/tasks")
        assert resp.status_code == 401

    @pytest.mark.smoke
    def test_task_schema_crud(self, api_client, api_base, random_name):
        task = self.setup_task(random_name)

        runner_name = task["runner"]
        api_client.post(f"{api_base}/runners", json={
            "name": runner_name, "gitURL": "https://example.com", "image": "python:3.11",
        })
        api_client.post(f"{api_base}/tasks", json=task)

        schema = {"type": "object", "properties": {"name": {"type": "string"}}}
        post_resp = api_client.post(f"{api_base}/tasks/{random_name}/schema", json=schema)
        assert post_resp.status_code == 200

        get_resp = api_client.get(f"{api_base}/tasks/{random_name}/schema")
        assert get_resp.status_code == 200

        del_resp = api_client.delete(f"{api_base}/tasks/{random_name}/schema")
        assert del_resp.status_code in (200, 204)
