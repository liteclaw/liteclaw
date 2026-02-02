# LiteClaw System Prompt Builder

This document describes how LiteClaw builds the system prompt for AI agents.

## Structure

The system prompt is assembled from several components:

1. **Identity** - Agent name and persona
2. **Capabilities** - Available tools and their descriptions
3. **Skills** - Loaded skill descriptions from SKILL.md files
4. **Context** - Current session context and metadata
5. **Runtime** - Current time, timezone, environment info

## Skills Section

The skills section follows the same format as Clawdbot:

```xml
<available_skills>
  <skill>
    <name>skill-name</name>
    <description>When to use this skill...</description>
    <location>/path/to/SKILL.md</location>
  </skill>
</available_skills>
```

The agent is instructed to:
1. Scan available skills before responding
2. Read the appropriate SKILL.md file when a skill applies
3. Follow the instructions in the skill file

## Tools Section

Tools are described with their names and JSON Schema parameters:

```
Available tools:
- exec: Execute shell commands
- read: Read file contents
- write: Write to files
- grep: Search for patterns in files
- find: Find files by name
- browser: Control the browser
- web_search: Search the web
```

## Implementation

See `internal/agent/prompt/builder.go` for the Go implementation.
