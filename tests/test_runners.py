import pytest


pytestmark = pytest.mark.crud


class TestRunners:
    def setup_runner(self, random_name):
        return {
            "name": random_name,
            "gitURL": "https://github.com/kriten-io/kriten-examples.git",
            "image": "python:3.11",
            "branch": "main",
            "token": "",
        }

    @pytest.mark.smoke
    def test_list_runners(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/runners")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    # @pytest.mark.smoke
    # def test_create_runner(self, api_client, api_base, random_name):
    #     runner = self.setup_runner(random_name)
    #     resp = api_client.post(f"{api_base}/runners", json=runner)
    #     assert resp.status_code == 200
    #     data = resp.json()
    #     assert data["name"] == random_name

    def test_get_runner(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        print(runner)
        create_resp = api_client.post(f"{api_base}/runners", json=runner)
        print("Status Code: create_resp.status_code")
        assert create_resp.status_code == 200

        resp = api_client.get(f"{api_base}/runners/{random_name}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["name"] == random_name
        assert data["gitURL"] == runner["gitURL"]

    def test_update_runner(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        create_resp = api_client.post(f"{api_base}/runners", json=runner)
        assert create_resp.status_code == 200

        update_data = {
            "name": random_name,
            "gitURL": "https://github.com/example/updated-repo.git",
            "image": "python:3.12",
            "branch": "develop",
        }
        resp = api_client.patch(f"{api_base}/runners/{random_name}", json=update_data)
        assert resp.status_code == 200
        data = resp.json()
        assert data["image"] == "python:3.12"

    def test_delete_runner(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        create_resp = api_client.post(f"{api_base}/runners", json=runner)
        assert create_resp.status_code == 200

        resp = api_client.delete(f"{api_base}/runners/{random_name}")
        assert resp.status_code in (200, 204)

        get_resp = api_client.get(f"{api_base}/runners/{random_name}")
        assert get_resp.status_code == 404

    @pytest.mark.negative
    def test_get_runner_not_found(self, api_client, api_base):
        resp = api_client.get(f"{api_base}/runners/nonexistent-runner")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_update_runner_not_found(self, api_client, api_base):
        resp = api_client.patch(
            f"{api_base}/runners/nonexistent-runner",
            json={"name": "nonexistent-runner", "gitURL": "https://example.com", "image": "python:3.11"},
        )
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_delete_runner_not_found(self, api_client, api_base):
        resp = api_client.delete(f"{api_base}/runners/nonexistent-runner")
        assert resp.status_code == 404

    @pytest.mark.negative
    def test_runner_unauthorized(self, noauth_client, api_base):
        resp = noauth_client.get(f"{api_base}/runners")
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_runner_unauthorized(self, noauth_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp = noauth_client.post(f"{api_base}/runners", json=runner)
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_create_runner_conflict(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        resp1 = api_client.post(f"{api_base}/runners", json=runner)
        assert resp1.status_code == 200

        resp2 = api_client.post(f"{api_base}/runners", json=runner)
        assert resp2.status_code == 409

    @pytest.mark.smoke
    def test_runner_secret_crud(self, api_client, api_base, random_name):
        runner = self.setup_runner(random_name)
        create_resp = api_client.post(f"{api_base}/runners", json=runner)
        assert create_resp.status_code == 200

        secret = {"key1": "value1", "key2": "value2"}
        post_resp = api_client.post(f"{api_base}/runners/{random_name}/secret", json=secret)
        assert post_resp.status_code == 200

        get_resp = api_client.get(f"{api_base}/runners/{random_name}/secret")
        assert get_resp.status_code == 200

        del_resp = api_client.delete(f"{api_base}/runners/{random_name}/secret")
        assert del_resp.status_code in (200, 204)
