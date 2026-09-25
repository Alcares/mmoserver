"""Exporting a trained policy for Go to run.

`model.save()` writes an SB3 checkpoint - pickled tensors, Adam's state, hyperparameters - which
exists so training can resume and which Go cannot read without reimplementing Python's pickle
format. This writes the other artifact: `policy.pb`, holding only what a forward pass needs.

Dropped on the way out: the critic (`value_net` and `mlp_extractor.value_net`), which exists to
compute advantages during training and is never consulted at inference, and the optimizer state,
which is larger than the weights themselves.

The schema lives in `backend/api/proto/bot/v1/policy.proto` and is the single source of truth;
both sides use generated code, so a field rename breaks the build rather than silently reading
zero. It does not, however, guarantee the two sides agree on *meaning* - a transposed weight
matrix still satisfies the schema. That is what the golden test is for.
"""

import os
import re
from pathlib import Path

import torch
from stable_baselines3 import PPO

from bot.v1 import policy_pb2

# torch.nn.Tanh is the only activation the schema can express, so anything else has to fail
# loudly here rather than export weights Go will run through the wrong function.
_ACTIVATIONS = {torch.nn.Tanh: policy_pb2.ACTIVATION_TANH}

_POLICY_LAYER = re.compile(r"^mlp_extractor\.policy_net\.(\d+)\.weight$")


def _layer_prefixes(state_dict) -> list[str]:
    """The policy path's Linear layers, in order, plus the logits head.

    Read out of the state dict rather than hardcoded: `policy_net` numbers its entries by
    position in the Sequential, so a Tanh sits between each pair and a two-layer net is 0 and 2.
    Widening net_arch would renumber them, and hardcoding would then silently drop a layer while
    still producing a file that loads.
    """
    indices = sorted(int(m.group(1)) for k in state_dict if (m := _POLICY_LAYER.match(k)))
    return [f"mlp_extractor.policy_net.{i}" for i in indices] + ["action_net"]


def build(model: PPO) -> policy_pb2.Policy:
    activation = _ACTIVATIONS.get(model.policy.activation_fn)
    if activation is None:
        raise ValueError(
            f"{model.policy.activation_fn.__name__} has no Activation in policy.proto; "
            "add it there and to bot.MLPPolicy before training with it"
        )

    state_dict = model.policy.state_dict()
    layers = []
    for prefix in _layer_prefixes(state_dict):
        weight = state_dict[f"{prefix}.weight"].numpy()
        bias = state_dict[f"{prefix}.bias"].numpy()
        out_features, in_features = weight.shape  # torch.nn.Linear stores [out, in]
        layers.append(
            policy_pb2.Layer(
                in_features=in_features,
                out_features=out_features,
                weight=weight.reshape(-1).tolist(),  # row-major, [out, in] order preserved
                bias=bias.tolist(),
            )
        )

    return policy_pb2.Policy(
        obs_size=model.observation_space.shape[0],
        action_count=int(model.action_space.n),
        activation=activation,
        layers=layers,
    )


def export(model: PPO, path: Path) -> None:
    """Writes the policy atomically, so a spectator reloading it never reads a half-written file."""
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_bytes(build(model).SerializeToString())
    os.replace(tmp, path)
