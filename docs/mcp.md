# MCP Server (Experimental)

The `kat` MCP server enables AI agents to interact with `kat` programmatically, reading output in a structured format and testing changes through file watching.

You can optionally start `kat` with its MCP server using the `--serve-mcp` flag:

```sh
kat --serve-mcp :50165
```

The UI remains active alongside the MCP server, allowing you to supervise and redirect agent actions in real-time.

> Note: The tight coupling between UI and MCP server may occasionally lead to unintended interference. If this becomes problematic, consider running a separate `kat` instance for the MCP server.

## Use Cases

AI agents perform best when provided well-defined, narrowly-scoped tasks with clear, testable success criteria. When attempting to use AI with Helm, Kustomize, and so on, tasks will often tend to deviate from these characteristics, which can often result in degraded performance or unpredictable behavior.

The `kat` MCP server is designed to make AI usage feasible in these scenarios. It does this by:

- Limiting access to context that is irrelevant to the task
- Forcing your AI to always follow the same rendering/validation pipeline
- Enabling iterative testing without needing human approval

Additionally, as a side effect of limiting access to complete manifests, it will normally reduce token consumption by orders of magnitude.

## Available Tools

- `list_resources`: Lists all resources rendered by `kat`
- `get_resource`: Retrieves the full YAML representation of a specific resource

> Note: Reloading is implicitly allowed if you allow your AI to modify files autonomously.

You **must** use a chat mode that supports iterative tool calling. E.g., "Agent" mode in VS Code. "Ask" and "Edit" modes will not function correctly with these tools. Claude Sonnet 4 or similar models with strong tool-use capabilities are recommended.

The MCP server includes built-in instructions to guide AI agents in using these tools effectively. However, less common project types may require additional custom instructions for optimal performance. If your AI is being difficult, try appending "Use #kat" to your instructions in chat for explicit tool engagement.

> Note: To provide specific tool-use instructions in VS Code, reference tools as `#mcp_kat_<tool_name>`.

## Security Considerations

- While `kat` is designed to operate as a read-only tool with local environment access only, the `kat` configuration does not in any way restrict what tools can be called. Please use caution when editing your config, and never allow your AI to autonomously edit your `kat` configuration files.
- Remember that you are handing over parts of other tools, which may have their own security implications. For example, it would be inadvisable to disable kustomize load restrictions.
- Use caution when combining the `kat` MCP server with other tools that can interact with external systems (e.g. `#fetch`).
- Always review AI-generated changes before applying them to production environments.
