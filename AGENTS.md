## Plan

If you are asked to plan some work, think about and write it to a `PLAN.md` file that looks like:

```
## Goal
Refactor authentication system to support OAuth

## Approach
1. Research OAuth 2.0 flows
2. Design token storage schema
3. Implement authorization server endpoints
4. Update client-side login flow
5. Add tests

## Current Step
Working on step 3 - authorization endpoints
```

## Todos

If you have enough work to do that you want to remember the individual steps. Write a `TODO.md` file that looks like:

```
- [x] Implement user authentication
- [x] Add database migrations
- [ ] Write API documentation
- [ ] Add rate limiting
```

## Browser Usage

If you need to use a browser to test some functionality, use `agent-browser --help` to learn how to do so from the cli.

## Tmux Usage

When you need to start a long running process or use an interactive CLI, use `tmux`.

- `tmux new-session -d -s debug_x "lldb ./x"`
- `tmux send-keys -t debug_x "c main" C-m`
- `tmux capture-pane -t debug_x -p`
- `tmux kill-session -t debug_x`

## Sub-agent

When you need a task acomplished that will generate lots of output tokens like searching for a specific location in a codebase or running a full test suite, use `kagent -p "your instructions"` and you'll get just the final answer or summary. 
