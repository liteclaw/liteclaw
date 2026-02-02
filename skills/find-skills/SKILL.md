---
name: Find Skills
description: Helper skill to search and discover other AI skills/tools using the 'skills' CLI.
---

# Find Skills

This skill helps you discover and install skills from the open agent skills ecosystem using the `npx skills` command directly.

## Capabilities

You can use the `run_command` tool to execute `skills` CLI commands directly.

### 1. Search for Skills

To find skills related to a topic:

```bash
npx -y skills find "your query"
```

Examples:
- `npx -y skills find "react performance"`
- `npx -y skills find "deploy"`
- `npx -y skills find "testing"`

**Output Interpretation:**
The command will list available skills with their installation command, e.g.:
`vercel-labs/agent-skills@vercel-react-best-practices`

### 2. Install a Skill

Once you found a skill, you can install it using the suggested command.
**Note:** Use `-g` (global) to install it for the user environment, and `-y` to skip confirmation.

```bash
npx -y skills add <package_name> -g -y
```

Example:
`npx -y skills add vercel-labs/agent-skills@vercel-react-best-practices -g -y`

### 3. List Installed Skills

To see what is currently installed:

```bash
npx -y skills list -g
```

## When to use this?

- When the user asks for "tools" or "skills" related to a specific topic (e.g., "help me optimize React").
- When you lack specific knowledge in a domain (e.g. "how do I deploy to Vercel?") and want to check if a specialized skill exists.
