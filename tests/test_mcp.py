import pytest


pytestmark = pytest.mark.crud

MISSING_ID = "00000000-0000-0000-0000-000000000000"
PROTOCOL_VERSION = "2025-06-18"
MCP_HEADERS = {
    "Accept": "application/json, text/event-stream",
    "Mcp-Protocol-Version": PROTOCOL_VERSION,
}

SCHEMA = {
    "type": "object",
    "description": "Restarts a service in an environment",
    "required": ["env"],
    "properties": {"env": {"type": "string"}},
}


class TestMcp:
    def create_runner(self, api_client, api_base, random_name):
        resp = api_client.post(f"{api_base}/runners", json={
            "name": f"runner-{random_name}",
            "gitURL": "https://github.com/example/test-repo.git",
            "image": "python:3.11",
        })
        assert resp.status_code == 200
        return resp.json()["id"]

    def create_task(self, api_client, api_base, name, runner_id, schema=None, description=None):
        task = {
            "name": name,
            "command": "python -c 'print(\"hello\")'",
            "runner_id": runner_id,
        }
        if schema is not None:
            task["schema"] = schema
        if description is not None:
            task["description"] = description
        resp = api_client.post(f"{api_base}/tasks", json=task)
        assert resp.status_code == 200
        return resp.json()["id"]

    def rpc(self, api_client, mcp_url, method, params=None, req_id=1):
        body = {"jsonrpc": "2.0", "id": req_id, "method": method}
        if params is not None:
            body["params"] = params
        resp = api_client.post(mcp_url, json=body, headers=MCP_HEADERS)
        assert resp.status_code == 200, resp.text
        return resp.json()

    def call_tool(self, api_client, mcp_url, name, arguments, req_id=1):
        return self.rpc(api_client, mcp_url, "tools/call", {
            "name": name,
            "arguments": arguments,
        }, req_id=req_id)

    @pytest.fixture
    def mcp_url(self, api_base):
        return f"{api_base}/mcp"

    @pytest.mark.smoke
    def test_mcp_requires_auth(self, noauth_client, mcp_url):
        resp = noauth_client.post(mcp_url, json={"jsonrpc": "2.0", "id": 1, "method": "ping"}, headers=MCP_HEADERS)
        assert resp.status_code == 401

    @pytest.mark.negative
    def test_mcp_rejects_cookie_auth(self, noauth_client, mcp_url, jwt_token):
        resp = noauth_client.post(
            mcp_url,
            json={"jsonrpc": "2.0", "id": 1, "method": "ping"},
            headers={**MCP_HEADERS, "Cookie": f"token={jwt_token}"},
        )
        assert resp.status_code == 401

    @pytest.mark.smoke
    def test_initialize_and_tools_list(self, api_client, mcp_url):
        init = self.rpc(api_client, mcp_url, "initialize", {
            "protocolVersion": PROTOCOL_VERSION,
            "capabilities": {},
            "clientInfo": {"name": "pytest", "version": "1"},
        })
        assert init["result"]["serverInfo"]["name"] == "kriten"

        tools = self.rpc(api_client, mcp_url, "tools/list", req_id=2)["result"]["tools"]
        names = {tool["name"] for tool in tools}
        assert {"list_tasks", "get_task", "run_task"} <= names

    @pytest.mark.smoke
    def test_list_tasks_only_returns_tasks_with_schema(self, api_client, api_base, mcp_url, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        with_schema = f"{random_name}-schema"
        without_schema = f"{random_name}-plain"
        task_id = self.create_task(api_client, api_base, with_schema, runner_id, schema=SCHEMA,
                                   description="Restart the web service")
        self.create_task(api_client, api_base, without_schema, runner_id)

        resp = self.call_tool(api_client, mcp_url, "list_tasks", {})
        assert "error" not in resp
        tasks = resp["result"]["structuredContent"]["tasks"]
        names = {task["name"] for task in tasks}
        assert with_schema in names
        assert without_schema not in names

        found = next(task for task in tasks if task["name"] == with_schema)
        assert found["id"] == task_id
        assert found["description"] == "Restart the web service"
        assert found["schema"]["description"] == SCHEMA["description"]
        assert "command" not in found
        assert "runner_id" not in found

    @pytest.mark.smoke
    def test_get_task_returns_schema(self, api_client, api_base, mcp_url, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task_id = self.create_task(api_client, api_base, random_name, runner_id, schema=SCHEMA)

        resp = self.call_tool(api_client, mcp_url, "get_task", {"id": task_id})
        result = resp["result"]
        assert not result.get("isError")
        assert result["structuredContent"]["id"] == task_id
        assert result["structuredContent"]["schema"]["required"] == ["env"]

    @pytest.mark.negative
    def test_get_task_without_schema_is_refused(self, api_client, api_base, mcp_url, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task_id = self.create_task(api_client, api_base, random_name, runner_id)

        resp = self.call_tool(api_client, mcp_url, "get_task", {"id": task_id})
        assert resp["result"]["isError"] is True

    @pytest.mark.negative
    def test_run_task_without_schema_is_refused(self, api_client, api_base, mcp_url, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task_id = self.create_task(api_client, api_base, random_name, runner_id)

        resp = self.call_tool(api_client, mcp_url, "run_task", {"id": task_id, "extra_vars": {}})
        assert resp["result"]["isError"] is True

    @pytest.mark.negative
    def test_run_task_invalid_extra_vars_is_refused(self, api_client, api_base, mcp_url, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task_id = self.create_task(api_client, api_base, random_name, runner_id, schema=SCHEMA)

        resp = self.call_tool(api_client, mcp_url, "run_task", {"id": task_id, "extra_vars": {}})
        result = resp["result"]
        assert result["isError"] is True
        assert "schema validation error" in result["content"][0]["text"]

    @pytest.mark.negative
    def test_get_unknown_task_is_refused(self, api_client, mcp_url):
        resp = self.call_tool(api_client, mcp_url, "get_task", {"id": MISSING_ID})
        assert resp["result"]["isError"] is True

    @pytest.mark.negative
    def test_get_job_unknown_is_refused(self, api_client, mcp_url):
        resp = self.call_tool(api_client, mcp_url, "get_job", {"id": MISSING_ID})
        assert resp["result"]["isError"] is True

    @pytest.mark.negative
    def test_get_job_wait_out_of_range_is_refused(self, api_client, mcp_url):
        resp = self.call_tool(api_client, mcp_url, "get_job", {"id": MISSING_ID, "wait_seconds": 31})
        result = resp["result"]
        assert result["isError"] is True
        assert "wait_seconds" in result["content"][0]["text"]

    @pytest.mark.crud
    def test_run_then_get_job_result(self, api_client, api_base, mcp_url, random_name):
        runner_id = self.create_runner(api_client, api_base, random_name)
        task_id = self.create_task(api_client, api_base, random_name, runner_id,
                                   schema={"type": "object"})

        run = self.call_tool(api_client, mcp_url, "run_task", {"id": task_id, "extra_vars": {}})["result"]
        assert not run.get("isError"), run
        job_id = run["structuredContent"]["id"]
        assert run["structuredContent"]["status"] in {"running", "completed", "failed"}

        job = self.call_tool(api_client, mcp_url, "get_job", {"id": job_id, "wait_seconds": 30})["result"]
        assert not job.get("isError"), job
        result = job["structuredContent"]
        assert result["id"] == job_id
        assert result["status"] in {"running", "completed", "failed"}
        assert "stdout" in result or result["status"] == "running"
