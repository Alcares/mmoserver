"""Baselines measured through the wrapper, to check it against numbers Go already knows.

The scripted baseline is the point of this file. Go's ScriptedPolicy terminates every episode in
1.05x optimal on average and 1.14x at worst; if the same policy driven through gRPC from Python
does not reproduce that, the wrapper is wrong, and it is far cheaper to learn that here than
from a PPO run that fails to converge for unrelated reasons.

The policy below picks a heading and never stops. Go's stops inside TradeRange, but the episode
has already terminated by then, so the two agree step for step - and this way Python needs no
copy of TradeRange or the world size. The duplicated arctan2 is deliberate: an independent
second implementation is what makes the comparison worth running.
"""

import argparse
import tempfile
from pathlib import Path

import numpy as np

from rl_training.env import SimVecEnv, launch_sim


def scripted(observations: np.ndarray, _rng: np.random.Generator) -> np.ndarray:
    """The nearest of the 8 directions, as bot.ScriptedPolicy computes it."""
    dx, dy = observations[:, 2], observations[:, 3]
    # arctan2(dx, -dy) is the bearing clockwise from north, the order the Action constants are
    # declared in, so the sector is the action with ActionStop taken off.
    sector = np.rint(np.arctan2(dx, -dy) / (np.pi / 4)).astype(np.int64)
    return (sector % 8) + 1


def uniform_random(action_count: int):
    """The floor any trained policy has to clear.

    Takes the action count from the env rather than hardcoding 9, which is the number
    ResetResponse.action_count exists to supply: Stage 2 appends buy and sell actions after the
    8 directions, and a hardcoded bound would quietly keep sampling movement only.
    """

    def act(observations: np.ndarray, rng: np.random.Generator) -> np.ndarray:
        return rng.integers(0, action_count, size=len(observations))

    return act


def measure(env: SimVecEnv, policy, episodes: int, seed: int = 0) -> dict:
    rng = np.random.default_rng(seed)
    # Seed the env too, so a measurement is reproducible and evaluation can be held out from
    # the seeds training drew.
    env.seed(seed)
    observations = env.reset()
    ratios: list[float] = []
    successes: list[bool] = []

    while len(ratios) < episodes:
        observations, _, _, infos = env.step(policy(observations, rng))
        for info in infos:
            if "episode" not in info:
                continue
            ratios.append(info["episode"]["l"] / info["optimal_steps"])
            successes.append(info["is_success"])

    return {
        "episodes": len(ratios),
        "success_rate": float(np.mean(successes)),
        "ratio_mean": float(np.mean(ratios)),
        "ratio_worst": float(np.max(ratios)),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--envs", type=int, default=64)
    parser.add_argument("--episodes", type=int, default=2000)
    parser.add_argument("--seed", type=int, default=0)
    args = parser.parse_args()

    with tempfile.TemporaryDirectory() as tmp:
        socket = Path(tmp) / "sim.sock"
        sim = launch_sim(socket)
        try:
            env = SimVecEnv(args.envs, f"unix://{socket}", seed=args.seed)
            policies = (("scripted", scripted), ("random", uniform_random(env.action_space.n)))
            for name, policy in policies:
                stats = measure(env, policy, args.episodes, seed=args.seed)
                print(
                    f"{name:9} success {stats['success_rate']:6.1%}  "
                    f"steps/optimal mean {stats['ratio_mean']:.3f}  "
                    f"worst {stats['ratio_worst']:.3f}  "
                    f"({stats['episodes']} episodes)"
                )
            env.close()
        finally:
            sim.terminate()
            sim.wait()


if __name__ == "__main__":
    main()
