# recall_history Tool

A **read-only, conversation-scoped** memory tool the model uses to answer questions
about the current conversation (e.g. "what did I ask first", "summarize our chat").

- **Source:** `agents/tools/recall_history.go` (tool) + `agents/history_provider.go` (Mongo adapter).
- **Not a knowledge source.** It is always available (when memory is on), independent
  of the Web / My Library toggles. PLAN §4.4 / §5.
- **Scope is fixed by the backend.** The tool is built **per request**, closing over
  the store and the current `conversation_id`. The model cannot target another
  conversation, and the tool has **no write/delete** path. PLAN §7.
- **Exact recall.** Reads stored user messages directly — not paraphrased from the
  rolling summary. PLAN §6.

## Input

JSON object:

```jsonc
{
  "mode": "all" | "first_n" | "search",  // required
  "n": 3,                                 // for mode "first_n"
  "query": "binary trees"                 // required for mode "search"
}
```

| Mode | Returns |
|---|---|
| `all` | All of the user's past questions in this conversation, ascending by `seq`. |
| `first_n` | The user's first `n` questions (`n` defaults to 1 if missing/≤0). |
| `search` | User + assistant messages whose `content` contains `query` (case-insensitive, literal — regex metacharacters are escaped). |

## Output

Plain text, one line per message:

```
Found 2 past message(s):
[#1 user] How do binary trees stay balanced?
[#3 user] What about red-black trees?
```

## Edge cases

| Situation | Behaviour |
|---|---|
| No matches | `No matching messages in this conversation.` |
| Memory unavailable (Mongo down → provider nil) | `Conversation memory is unavailable.` |
| `mode: "search"` without `query` | Error: requires a non-empty `query`. |
| Unknown `mode` (e.g. `"delete"`) | Error: `unknown mode "delete"` — there is no write/delete path. |
| Malformed JSON input | Error: `invalid recall_history input (expected JSON object)`. |

## Example ReAct usage

```
Thought: The user is asking what they asked first. I should read the history.
Action: recall_history
Action Input: {"mode": "first_n", "n": 1}
Observation: Found 1 past message(s):
[#1 user] How do binary trees stay balanced?
Final Answer: Your first question was "How do binary trees stay balanced?"
```
