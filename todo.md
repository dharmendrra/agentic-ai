# TODO

## Deferred from conversational-retrieval plan

- [ ] **SSE streaming for answers + reasoning steps.**
  Current design returns a single JSON answer (`AgentResponse`). The frontend
  (`agents/static/app.js`) already has cursor/streaming hooks unused. Investigate
  streaming the Final Answer token-by-token, and optionally each ReAct
  Thought/Action/Observation step, over SSE — similar to how the `:8081` PDF
  endpoint already streams (`event: sources`, see `agents/docs/PDF_ENDPOINT_SSE_FORMAT.md`).
  Decide: stream from the agent's `/api/agent/query`, and how it interacts with
  the per-turn conversation persistence (persist after stream completes).

See `PLAN_CONVERSATIONAL_RETRIEVAL.md` for the full plan.
