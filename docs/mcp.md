# MCP Server

You can optionally start `kat` with an MCP server by using the `--serve-mcp` flag:

```sh
kat --serve-mcp :50165
```

The MCP server allows AI agents to read output from `kat` in a structured way, and test changes when source files are edited (via the file watcher).

The `kat` UI will follow along with whatever tool calls are being made, so that you can easily supervise and redirect agents as needed. The downside of this is that the UI and MCP server are tightly coupled and you may find yourself interfering with your agent unintentionally (or vice versa). If needed, you can always start multiple instances of `kat`, one for the MCP server and one for manual use.

Note: The MCP server will only work with agents able to read output, think, and then make additional calls. For example, "Ask" and "Edit" modes in VSCode will not work properly. I recommend using Claude Sonnet 4.

Note: The MCP server contains instructions that will encourage agents to use `kat`. However, especially for less common project types, your agent may require additional instructions.

### 🧰 Tools

- `list_resources`: List all resources that were rendered by kat.
- `get_resource`: Get a full YAML representation of a specific resource.

### ⚠️ WARNING

Remember that `kat` is meant to be read-only, and is only meant to have access to your local environment. However, nothing is stopping you (or your AI agent) from adding any arbitrary, possibly dangerous configuration.
