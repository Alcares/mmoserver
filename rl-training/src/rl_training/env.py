"""The training environment: a Stable-Baselines3 VecEnv backed by the Go sim over gRPC.

SB3's VecEnv rather than gymnasium.vector.VectorEnv, deliberately. Gymnasium 1.0 resets an env
on the step *after* it terminates and discards that action; the Go server resets in place and
returns the terminal observation alongside the next episode's first, which is exactly SB3's
convention. Matching SB3 needs no shim, matching Gymnasium would mean rewriting Go's autoreset:

    StepResponse.observations[i]        ->  obs[i]                        (next episode's first)
    StepResponse.final_observations[i]  ->  infos[i]["terminal_observation"]
    terminated | truncated              ->  dones[i]
    truncated and not terminated        ->  infos[i]["TimeLimit.truncated"]

Nothing here scales, clips or normalises anything. Whatever this file did to an observation,
Go's MLPPolicy would have to reproduce at inference, so policy.pb stays the complete
description of the bot by keeping this layer arithmetic-free.
"""

import subprocess
import time
from pathlib import Path

import grpc
import numpy as np
from gymnasium import spaces
from stable_baselines3.common.vec_env import VecEnv

from sim.v1 import env_pb2, env_pb2_grpc

REPO_ROOT = Path(__file__).resolve().parents[3]


class SimVecEnv(VecEnv):
    """One gRPC call per batched step. The vector width is fixed by the first Reset."""

    def __init__(self, num_envs: int, address: str, seed: int = 0):
        self._channel = grpc.insecure_channel(address)
        self._stub = env_pb2_grpc.EnvStub(self._channel)
        self._seed_rng = np.random.default_rng(seed)
        self._actions: np.ndarray | None = None
        self.render_mode = None

        # The first Reset doubles as the handshake: the observation width and the action count
        # arrive from Go, and both are needed before the spaces can be built.
        response = self._reset_rpc(num_envs)

        # Positions are world-clamped and everything is divided by the world size, so x, y land
        # in [0, 1], the offsets in [-1, 1] and the distance in [0, sqrt(2)].
        observation_space = spaces.Box(
            low=-2.0, high=2.0, shape=(response.obs_size,), dtype=np.float32
        )
        super().__init__(num_envs, observation_space, spaces.Discrete(response.action_count))

        self._ep_returns = np.zeros(num_envs, dtype=np.float64)
        self._ep_lengths = np.zeros(num_envs, dtype=np.int64)
        self._start_time = time.perf_counter()

    def reset(self) -> np.ndarray:
        response = self._reset_rpc(self.num_envs)
        self._ep_returns[:] = 0.0
        self._ep_lengths[:] = 0
        return self._stack(response.observations)

    def step_async(self, actions: np.ndarray) -> None:
        self._actions = actions

    def step_wait(self):
        response = self._stub.Step(env_pb2.StepRequest(actions=self._actions.tolist()))

        observations = self._stack(response.observations)
        rewards = np.asarray(response.rewards, dtype=np.float32)
        terminated = np.asarray(response.terminated, dtype=bool)
        truncated = np.asarray(response.truncated, dtype=bool)
        dones = terminated | truncated

        self._ep_returns += rewards
        self._ep_lengths += 1

        infos: list[dict] = [{} for _ in range(self.num_envs)]
        for i in np.flatnonzero(dones):
            info = infos[i]
            info["terminal_observation"] = np.asarray(
                response.final_observations[i].observation, dtype=np.float32
            )
            if truncated[i] and not terminated[i]:
                info["TimeLimit.truncated"] = True
            # What SB3's Monitor would normally supply; it becomes rollout/ep_rew_mean.
            info["episode"] = {
                "r": float(self._ep_returns[i]),
                "l": int(self._ep_lengths[i]),
                "t": time.perf_counter() - self._start_time,
            }
            # The fewest steps this episode could have taken, against which episode["l"] is
            # the metric. It comes from Go because deriving it needs game constants.
            info["optimal_steps"] = response.optimal_steps[i]
            info["is_success"] = bool(terminated[i])

            self._ep_returns[i] = 0.0
            self._ep_lengths[i] = 0

        return observations, rewards, dones, infos

    def seed(self, seed: int | None = None):
        if seed is not None:
            self._seed_rng = np.random.default_rng(seed)
        return [seed] * self.num_envs

    def close(self) -> None:
        self._channel.close()

    def _reset_rpc(self, num_envs: int):
        # Well-separated seeds rather than a contiguous range: Go derives each env's later
        # episodes from its own stream, but the first episode is this seed exactly.
        seeds = self._seed_rng.integers(0, 2**63 - 1, size=num_envs, dtype=np.int64)
        return self._stub.Reset(env_pb2.ResetRequest(seeds=seeds.tolist()))

    @staticmethod
    def _stack(observations) -> np.ndarray:
        return np.array([o.observation for o in observations], dtype=np.float32)

    def _count(self, indices) -> int:
        if indices is None:
            return self.num_envs
        return 1 if isinstance(indices, int) else len(indices)

    # SB3 reaches into per-env Python attributes through the four methods below. The vector
    # lives in Go and has no Python state, so they answer only what SB3 actually asks for.
    def get_attr(self, attr_name: str, indices=None) -> list:
        return [getattr(self, attr_name)] * self._count(indices)

    def set_attr(self, attr_name: str, value, indices=None) -> None:
        raise NotImplementedError("the sim envs hold no Python attributes")

    def env_method(self, method_name: str, *args, indices=None, **kwargs) -> list:
        raise NotImplementedError("the sim envs expose no Python methods")

    def env_is_wrapped(self, wrapper_class, indices=None) -> list[bool]:
        return [False] * self._count(indices)


def launch_sim(socket_path: Path) -> subprocess.Popen:
    """Builds and starts the Go sim, and waits for its socket.

    Training spawns its own rather than attaching to whatever happens to be listening: the build
    is cached and near-free, and it removes the failure where Go changes, the old binary keeps
    running, and a whole run trains against stale code.
    """
    subprocess.run(
        ["go", "-C", "backend", "build", "-o", "bin/sim", "./cmd/sim"],
        cwd=REPO_ROOT,
        check=True,
    )
    process = subprocess.Popen(
        [str(REPO_ROOT / "backend/bin/sim"), "-addr", f"unix://{socket_path}"],
        cwd=REPO_ROOT,
    )
    for _ in range(100):
        if socket_path.exists():
            return process
        if process.poll() is not None:
            raise RuntimeError(f"sim exited with {process.returncode} before listening")
        time.sleep(0.05)
    process.kill()
    raise TimeoutError(f"sim did not create {socket_path} within 5s")
