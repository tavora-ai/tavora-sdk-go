# Agents and Skills

## What Are Agents?

Agents are autonomous AI runtimes that can reason over multiple steps, use tools, and make decisions. Unlike simple chat completions, agents can:

- Break complex tasks into subtasks
- Call tools and skills to gather information or take actions
- Iterate on results and self-correct
- Maintain context across multiple reasoning steps

## Running an Agent

### Via the SDK
```go
session, err := client.RunAgent(ctx, tavora.RunAgentInput{
    Prompt: "Research the latest pricing changes and summarize them",
})
```

### Via the Dashboard
1. Go to "Agents" in the sidebar
2. View past agent sessions and their step-by-step reasoning
3. Use the Studio to analyze and debug agent behavior

## Agent Sessions

Each agent run creates a session that records:
- The initial prompt
- Each reasoning step and tool call
- Token usage and timing
- The final output

View sessions in the dashboard under Agents > Sessions.

## Skills

Skills extend what agents can do. There are two types:

### Prompt Skills
A prompt skill is a reusable prompt template that agents can invoke:

1. Go to Space Config > Skills
2. Click "Create Skill"
3. Enter a name, description, and prompt template
4. The agent will use the skill when its description matches the task

Example: A "summarize" skill with the prompt "Summarize the following text in 3 bullet points: {{input}}"

### Webhook Skills
Webhook skills call an external HTTP endpoint:

1. Create a skill with type "webhook"
2. Provide the webhook URL and any required headers
3. The agent sends a JSON payload to your endpoint and uses the response

Use cases: looking up customer data, checking inventory, triggering workflows.

## MCP Servers

Model Context Protocol (MCP) servers provide tools that agents can use. Configure MCP servers in Space Config > MCP Servers.

Each MCP server exposes a set of tools. The agent automatically discovers available tools and uses them when relevant.

## Scheduled Runs

Run agents on a recurring schedule:

1. Go to Space Config > Scheduled Runs
2. Click "Create" and enter a cron expression (e.g., `0 9 * * MON` for every Monday at 9 AM)
3. Provide the agent prompt
4. The agent runs automatically and results are logged as sessions

Use cases: daily report generation, periodic data checks, automated monitoring.

## Troubleshooting

**Agent session shows "failed" status:**
- Check the session details for error messages
- Ensure any webhook skills are accessible and returning valid responses
- Verify MCP servers are running and reachable

**Agent not using expected skills:**
- Review the skill description — agents match skills by description, so make it clear and specific
- Check that the skill is enabled in the current space
