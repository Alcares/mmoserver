"""PPO on the movement task.

The bar is ScriptedPolicy, and the honest target is to match it rather than beat it: progress
per tick is 0.6 * cos(angle between the action and the bearing), so the nearest of 8 directions
is already the best available action and there is nothing for a network to out-think. What this
run buys is the pipeline - observation contract, autoreset, reward, gradients, export - checked
while a known-good answer still exists to check against. See docs/BOT_TRAINING.md step 13.

Run `python -m rl_training.baseline` first. If scripted does not measure ~1.05 through the
wrapper, fix that before reading anything into a training curve.
"""

from __future__ import annotations

import argparse
import tempfile
from pathlib import Path

import numpy as np
from stable_baselines3 import PPO
from stable_baselines3.common.callbacks import BaseCallback

from rl_training.baseline import measure, scripted
from rl_training.env import REPO_ROOT, SimVecEnv, launch_sim

RL_DIR = REPO_ROOT / "rl-training"


class EpisodeMetrics(BaseCallback):
    """Logs what the roadmap asks for: success rate and steps taken over steps needed.

    SB3's own rollout/ep_rew_mean tracks the shaped reward, which can rise while the bot still
    fails; these two are the numbers that actually say whether it arrives, and how directly.
    """

    def __init__(self) -> None:
        super().__init__()
        self._ratios: list[float] = []
        self._successes: list[bool] = []

    def _on_step(self) -> bool:
        for info in self.locals["infos"]:
            if "episode" not in info:
                continue
            self._ratios.append(info["episode"]["l"] / info["optimal_steps"])
            self._successes.append(info["is_success"])
        return True

    def _on_rollout_end(self) -> None:
        if not self._ratios:
            return
        self.logger.record("bot/success_rate", float(np.mean(self._successes)))
        self.logger.record("bot/steps_over_optimal", float(np.mean(self._ratios)))
        self.logger.record("bot/steps_over_optimal_worst", float(np.max(self._ratios)))
        self._ratios.clear()
        self._successes.clear()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--envs", type=int, default=128)
    parser.add_argument("--timesteps", type=int, default=2_000_000)
    parser.add_argument("--n-steps", type=int, default=128, help="per env, so the rollout is envs*n_steps")
    parser.add_argument("--seed", type=int, default=0)
    parser.add_argument("--device", default="auto")
    # Anchored to the project: `uv run --project` does not change the working directory, so a
    # relative default would drop runs/ and policy.zip wherever make happened to be invoked.
    parser.add_argument("--logdir", type=Path, default=RL_DIR / "runs")
    parser.add_argument("--out", type=Path, default=RL_DIR / "policy.zip")
    parser.add_argument("--eval-episodes", type=int, default=2000)
    args = parser.parse_args()

    with tempfile.TemporaryDirectory() as tmp:
        socket = Path(tmp) / "sim.sock"
        sim = launch_sim(socket)
        try:
            env = SimVecEnv(args.envs, f"unix://{socket}", seed=args.seed)
            model = PPO(
                "MlpPolicy",
                env,
                n_steps=args.n_steps,
                seed=args.seed,
                device=args.device,
                # Spelled out rather than left to SB3's default, because the width is part of
                # what step 14 has to reimplement in Go.
                policy_kwargs={"net_arch": {"pi": [64, 64], "vf": [64, 64]}},
                tensorboard_log=str(args.logdir),
                verbose=1,
            )
            model.learn(total_timesteps=args.timesteps, callback=EpisodeMetrics())
            model.save(args.out)
            print(f"saved {args.out}")

            # Held-out seeds and argmax actions: a sampling policy fumbles the last step near
            # the goal, and the reported number should be the one the exported policy will hit.
            def act(observations, _rng):
                actions, _ = model.predict(observations, deterministic=True)
                return actions

            for name, policy in (("trained", act), ("scripted", scripted)):
                stats = measure(env, policy, args.eval_episodes, seed=args.seed + 1_000_000)
                print(
                    f"{name:9} success {stats['success_rate']:6.1%}  "
                    f"steps/optimal mean {stats['ratio_mean']:.3f}  "
                    f"worst {stats['ratio_worst']:.3f}"
                )
            env.close()
        finally:
            sim.terminate()
            sim.wait()


if __name__ == "__main__":
    main()
