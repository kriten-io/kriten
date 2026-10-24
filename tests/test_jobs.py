import pytest


pytestmark = pytest.mark.crud


class TestJobs:
    @pytest.mark.smoke
    def test_list_jobs(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/jobs")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.smoke
    def test_list_jobs_with_filters(self, api_client, api_base):
        resp = api_client.get(
            f"{api_base}/jobs",
            params={"limit": 10, "offset": 0},
        )
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    @pytest.mark.negative
    def test_get_job_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/jobs/nonexistent-job")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_get_job_log_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/jobs/nonexistent-job/log")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_jobs_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/jobs")
        assert resp.status_code == 401

    def test_get_job(self, api_client, api_base, random_name):
        runner_name = f"{random_name}-runner"
        runner_resp = api_client.post(f"{api_base}/runners", json={
            "name": runner_name,
            "gitURL": "https://github.com/example/test-repo.git",
            "image": "python:3.11",
        })
        assert runner_resp.status_code == 200
        runner_id = runner_resp.json()["id"]

        task_resp = api_client.post(f"{api_base}/tasks", json={
            "name": random_name,
            "command": "python -c 'print(\"hello\")'",
            "runner_id": runner_id,
        })
        assert task_resp.status_code == 200
        task_id = task_resp.json()["id"]

        create_resp = api_client.post(f"{api_base}/tasks/{task_id}/run", json={})
        assert create_resp.status_code == 200
        job_name = create_resp.json().get("name", "")

        if job_name:
            resp = api_client.get(f"{api_base}/jobs/{job_name}")
            assert resp.status_code == 200
