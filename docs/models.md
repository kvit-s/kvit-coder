# Models

A model in kvit-coder is one row of the `models:` list in `config.yaml`. Each
row names an endpoint, the model id that endpoint expects, the wire protocol,
the context window, and the effort levels the UI offers as `:e1`, `:e2`, and
so on. `:m1` is the first row. The row whose model id matches `llm.model` is
the one a new session starts on. Every key is defined in
[configuration.md](configuration.md). This page is how to fill a row from
OpenCode, which has a pay-per-request catalog (Zen) and a separate low-cost
subscription (Go).

The model ids below were taken from OpenCode on 28 September 2026 and change
over time; the Zen and Go pages linked below are the current list.

## OpenCode

OpenCode publishes two gateways. Zen is a pay-per-request catalog at
`https://opencode.ai/zen/v1`, with a handful of models that are free to call
for a limited time. Go is a separate subscription, ten dollars a month, at
`https://opencode.ai/zen/go/v1`. The steps for obtaining a key are on
[OpenCode Zen](https://opencode.ai/docs/zen) and
[OpenCode Go](https://opencode.ai/docs/go). The console is
[opencode.ai/zen](https://opencode.ai/zen).

Both gateways use the same kind of key. Sign in, and for Zen add the billing
details the console asks for before it will show a key. Copy the key into
the environment, not into the file:

```bash
export OPENCODE_API_KEY="the key from the console"
```

A Go subscription is started from that same console. OpenCode allows one Go
subscriber per workspace. After the Go usage included in the month is spent,
the free models on the account still answer. The model ids and which URL
each one uses are on the Go page, and the machine-readable list is
`https://opencode.ai/zen/go/v1/models`.

The path at the end of OpenCode's endpoint URL decides `api_backend` and
where `base_url` stops:

| OpenCode endpoint ends in | `base_url` | `api_backend` |
|---|---|---|
| `/v1/responses` | `https://opencode.ai/zen/v1` or `https://opencode.ai/zen/go/v1` | `responses` |
| `/v1/chat/completions` | the same | `chat_completions` |
| `/v1/messages` | `https://opencode.ai/zen` or `https://opencode.ai/zen/go` | `messages` |

The messages case stops before `/v1` because kvit-coder appends
`/v1/messages` itself. A `base_url` of `https://opencode.ai/zen/v1` with
`api_backend: messages` would request `/v1/v1/messages`. Chat completions on
these gateways read effort from the top-level `reasoning_effort` field, so
set `effort_field: reasoning_effort`. The responses protocol sends
`reasoning.effort` and does not use `effort_field`.

OpenCode Go asks a client to name itself and to send a stable session id on
`x-opencode-session`, one value per conversation, so a prompt cache stays on
one backend. Put both on the `llm:` block. Headers there are sent with every
model. `${KVIT_RUN_ID}` is an id derived from the session name.

```yaml
llm:
  api_key_env: "OPENCODE_API_KEY"
  headers:
    - "User-Agent=kvit-coder"
    - "x-opencode-session=kvit-coder-${KVIT_RUN_ID}"
```

### Free Zen models

These models are called with the Zen key and are free for a limited time, as
OpenCode's Zen page describes it. The page also says what each provider does
with prompts. Muse Spark 1.3 Contributor Free, for example, is offered at a
discount in exchange for permission to train on prompts and completions.
Read that section before sending private code to a free model.

| Model | Id | Protocol |
|---|---|---|
| Muse Spark 1.3 Contributor Free | `muse-spark-1.3-contributor-free` | responses |
| Big Pickle | `big-pickle` | chat completions |
| Space Bunny Free | `space-bunny-free` | chat completions |
| LongCat 2.5 Preview Free | `longcat-2.5-preview-free` | chat completions |
| MiMo-V2.6-Flash Free | `mimo-v2.6-flash-free` | chat completions |
| MiMo-V2.5 Free | `mimo-v2.5-free` | chat completions |
| Ling 3.0 Flash Fin Free | `ling-3.0-flash-fin-free` | chat completions |
| Nemotron 3 Ultra Free | `nemotron-3-ultra-free` | chat completions |
| Nemotron 3.5 Lightning Free | `nemotron-3.5-lightning-free` | chat completions |
| Jev 1.13 Free | `jev-1.13-free` | not supported here |

Jev is served at `/v1/systemone`, which kvit-coder does not speak.

```yaml
models:
  - id: spark-zen
    name: "Muse Spark 1.3 Zen Free"
    model: "muse-spark-1.3-contributor-free"
    base_url: "https://opencode.ai/zen/v1"
    api_backend: "responses"
    api_key_env: "OPENCODE_API_KEY"
    context: 1048576
    efforts:
      - value: minimal
      - value: low
      - value: medium
      - value: high
        default: true
      - value: xhigh
```

Paid Zen models use the same base URL and the same key. Pick the id and the
protocol from the endpoint table on the Zen page. A model whose endpoint is
`https://opencode.ai/zen/v1/messages` uses `base_url: "https://opencode.ai/zen"`
and `api_backend: messages`.

### Go subscription

Subscribe from the Zen console, then point a row at
`https://opencode.ai/zen/go/v1` and the model id from the Go page. The id
here is the bare id, such as `kimi-k3`. OpenCode's own config writes that
same id as `opencode-go/kimi-k3`.

Muse Spark on Go is served at `/responses`. Kimi K3 on the same subscription
is served at `/chat/completions`. Asking Kimi for the responses protocol
fails, because that path is not served for it.

```yaml
models:
  - id: spark-go
    name: "Muse Spark 1.3 Go"
    model: "muse-spark-1.3-contributor"
    base_url: "https://opencode.ai/zen/go/v1"
    api_backend: "responses"
    api_key_env: "OPENCODE_API_KEY"
    context: 1048576
    efforts:
      - value: minimal
      - value: low
      - value: medium
      - value: high
      - value: xhigh
        default: true

  - id: kimi-go
    name: "Kimi K3 (Go)"
    model: "kimi-k3"
    base_url: "https://opencode.ai/zen/go/v1"
    api_backend: "chat_completions"
    effort_field: "reasoning_effort"
    api_key_env: "OPENCODE_API_KEY"
    context: 1048576
    efforts:
      - value: minimal
      - value: low
      - value: medium
      - value: high
        default: true
      - value: xhigh
```

The other Go models follow the same pattern. Grok and the GPT Luna models on
the Go page use `/responses`. GLM, DeepSeek, MiMo, Hy, Space Bunny Free, and
LongCat use `/chat/completions`. MiniMax and the Qwen models on that page use
`/messages`, so their `base_url` is `https://opencode.ai/zen/go`.
Space Bunny Free and LongCat 2.5 Preview Free are included with Go for a
limited time and are also on the free Zen list above, at the Zen base URL.
Muse Spark Contributor on Go is limited to some regions, which the Go page
states next to that model.
