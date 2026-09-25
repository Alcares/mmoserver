# Policy Inference: Where the Trained Bot Runs

This records why a trained policy runs its forward pass in Go on the CPU, with no GPU and no
cgo, and where that answer stops holding. The code it describes is `../backend/internal/bot/` (`MLPPolicy`, the `Policy` interface).

Training is the opposite case and is settled separately: it runs in Python, and a GPU there is
worth considering. See the Architecture section of `BOT_TRAINING.md`.

## The question

Acting is a forward pass through a small MLP. Three things were unclear:

1. Should Go run it at all, rather than calling out to the Python process that trained it?
2. Is the CPU fast enough, at Stage 1 sizes and at the larger networks trading will need?
3. Could Go use the GPU if it were not?

## Should Go run it at all

Yes, and the reasons are structural rather than about speed.

The bot has to play like a human client: step 15 spawns it as a `Client` with a `Send` buffer,
joined through `World.Join`, deciding inside the tick loop. A policy living in another process
cannot sit in that loop without putting an RPC on the critical path of the game server - and a
local round trip measured ~100 µs during the training work, which is ~50x the forward pass
itself, per bot per tick. It would also put a Python runtime in the server's deployment.

The `Policy` interface already makes `ScriptedPolicy` and `MLPPolicy` interchangeable across the
server, the spectator and the sim, which is what lets the scripted baseline act as a control.

## Measurements

Naive scalar Go, `[out][in]` row-major weights, `math.Tanh` between layers, no SIMD, no
batching, zero allocations per pass. Three layers, `in -> hidden -> hidden -> out`. Measured on
a 12-thread desktop CPU; treat the ratios as the durable part, not the absolute numbers.

| network | params | per pass | 10 bots | 50 bots |
|---|---|---|---|---|
| Stage 1, `5 -> 64 -> 64 -> 9` (today) | 5.1k | 1.9 µs | 0.04% | 0.19% |
| Stage 2, realistic `100 -> 256 -> 256 -> 57` | 106k | 37.6 µs | 0.8% | 3.8% |
| generous `200 -> 512 -> 512 -> 57` | 395k | 142 µs | 2.8% | 14% |
| large `512 -> 1024 -> 1024 -> 128` | 1.7M | 624 µs | 12% | 62% |

Percentages are of one 20 Hz tick (50 ms), at `game.MaxPlayers` = 50 and at a more plausible
10 bots. For scale, one `World.Tick` in `BenchmarkStep` is 4.36 µs, so at Stage 1 the policy
costs less than half of simulating the tick that feeds it.

The Stage 2 row is sized from the real domain: position, time, cash, 6 commodities of
inventory, 6 x `len(OrderSizes)` x buy/sell quotes, station positions and distances - about 100
inputs - against 9 directions plus buy/sell for each commodity and order size, 57 actions.

## Why the CPU is enough

**The observation is small.** Large policies exist for large observations. OpenAI Five needed a
4096-unit LSTM because Dota handed it ~16k inputs; AlphaStar was large because it convolved a
spatial map. The `Observer` hands the policy ~100-200 structured floats, already digested. A
2x256 MLP is the ordinary answer at that width, and that is the 3.8% row.

**There is a lot of headroom before exotic measures.** In order of value:

1. **Batch the bots in a world into one matmul.** 50 bots today would be 50 matrix-*vector*
   products, each re-streaming the entire weight matrix from memory. As one `(50 x in) @ (in x
   hidden)` the weights load once and are reused 50 times. At these sizes this is a
   memory-bandwidth problem, not a FLOP problem, so this is where the large multiple is.
2. **Not the activation - measured, and it does not pay.** `math.Tanh` takes and returns
   float64, so every activation costs two conversions, and it looks like an easy win. It is
   not. Against the Stage 1 row, tanh is 1952 ns, ReLU 1768 ns, and removing the activation
   entirely 1543 ns - so the whole of it is 21%, and the best case for replacing it is 1.26x.
   At the Stage 2 row it disappears into run-to-run noise (tanh 37.5 µs, ReLU 38.3 µs),
   because 512 activations are nothing beside ~93k multiply-adds. The matmul dominates
   precisely where the budget would be tight, so batching is the only lever that matters.

   This also means there is no inference argument for choosing ReLU over tanh. The activation
   is a training-quality decision, not a performance one.

## Why not the GPU

**It would be slower.** A kernel launch plus host-to-device and back is a ~5-20 µs floor,
independent of how little work is inside it. Every row of the table above is a batch of at most
50 short vectors; at Stage 1 the entire computation is 1.9 µs, so the overhead alone is 3-10x
the work. GPUs win on large batched matrix work - which is training, not this.

Note that batching is what a GPU needs to amortise that overhead, and it is also the biggest
CPU win. Doing the work that would make a GPU viable makes the CPU fast enough not to need one.

**It would break the deployment.** Every Go path to a GPU - CUDA bindings, ROCm/HIP, ONNX
Runtime - goes through cgo. `../Dockerfile` builds with `CGO_ENABLED=0` into
`gcr.io/distroless/static-debian12`, an image with no libc and no dynamic loader. A GPU would
mean abandoning the static binary, changing base images, and shipping GPU userspace and device
access into the container, to make a few percent of a tick slower.

## Where this stops holding

If a policy ever reaches the 1.7M-parameter row *and* runs for many bots at once - most likely
if Stage 2 adds recurrence over market history rather than from width alone - the budget gets
tight. The order of response is: batch per world, then a float32 activation, then a SIMD or
BLAS-backed matmul, and only then reconsider the process boundary. A GPU is not on that list at
inference; it stays a training question.

## Reproducing

The forward-pass benchmark is not in the repo - it was a standalone module, since `MLPPolicy`
does not exist until step 14. Once it does, the same measurement belongs in
`internal/bot/policy_bench_test.go` so the table above can be re-checked rather than trusted.
