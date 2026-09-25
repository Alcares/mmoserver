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
from rl_training.export import export

RL_DIR = REPO_ROOT / "rl-training"

EXPORTS_PER_RUN = 50  # export is a model snapshot mid-training


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


class ExportSnapshots(BaseCallback):
    """Writes `policy.pb` every `every` rollouts, so a spectator can watch training improve.

    The unit is rollouts because that is the only rate that means anything: PPO mutates the
    policy once per rollout and then runs its gradient epochs, so two exports inside one rollout
    are byte-identical files. A 2M run at the defaults is 128*128 = 16,384 timesteps per rollout,
    so 122 rollouts total - which is why EXPORTS_PER_RUN of 100 rounds to every rollout here and
    only starts thinning out on longer runs.

    `_on_rollout_start`, not `_on_step`: `_on_step` fires once per *vectorised* step, so any
    frequency set there is silently multiplied by the env count, and both it and
    `_on_rollout_end` run before `train()` - they would export the weights the update is about
    to replace. `_on_rollout_start` runs after it.

    Each firing writes three files from one set of weights, so none of them can drift: a
    numbered `.pb` into the archive, the fixed `policy.pb` a live spectator watches, and a
    numbered `.zip`. The `.zip` is not for Go at all - it is the opponent pool self-play will
    want later (BOT_TRAINING.md "Self-play"), which is why the archive is kept rather than
    overwritten.
    """

    def __init__(self, every: int, policy_path: Path, archive: Path) -> None:
        super().__init__()
        self.every = every
        self.policy_path = policy_path
        self.archive = archive
        self._rollouts = 0

    def _on_rollout_start(self) -> None:
        # Fires before the first rollout too, when no update has happened yet and the weights
        # are still the initialisation.
        if self.model.num_timesteps == 0:
            return

        self._rollouts += 1
        if self._rollouts % self.every:
            return

        self.archive.mkdir(parents=True, exist_ok=True)
        steps = self.model.num_timesteps
        self.model.save(self.archive / f"policy_{steps}.zip")
        # Both the numbered .pb and the fixed one, from the same weights: the archive is what
        # bot.SnapshotPolicy plays through in order, the fixed path what bot.ReloadingPolicy
        # watches for the newest. Which a spectator uses is the server's choice, not ours.
        export(self.model, self.archive / f"policy_{steps}.pb")
        export(self.model, self.policy_path)

    def _on_step(self) -> bool:
        return True


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
    # The SB3 checkpoint above resumes training; this is what Go loads. See export.py.
    parser.add_argument("--policy", type=Path, default=RL_DIR / "policy.pb")
    parser.add_argument("--snapshots", type=Path, default=RL_DIR / "snapshots",
                        help="where the numbered checkpoints behind each export land")
    parser.add_argument("--eval-episodes", type=int, default=2000)
    args = parser.parse_args()

    # Rounded to whole rollouts because a fraction of one exports the same bytes twice.
    rollouts = args.timesteps // (args.envs * args.n_steps)
    every = max(1, round(rollouts / EXPORTS_PER_RUN))
    print(f"{rollouts} rollouts, exporting every {every} -> ~{rollouts // every} snapshots")

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
            model.learn(
                total_timesteps=args.timesteps,
                callback=[EpisodeMetrics(), ExportSnapshots(every, args.policy, args.snapshots)],
            )
            model.save(args.out)
            export(model, args.policy)
            print(f"saved {args.out} and {args.policy}")

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
