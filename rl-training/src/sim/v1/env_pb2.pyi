from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Observation(_message.Message):
    __slots__ = ("observation",)
    OBSERVATION_FIELD_NUMBER: _ClassVar[int]
    observation: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, observation: _Optional[_Iterable[float]] = ...) -> None: ...

class ResetRequest(_message.Message):
    __slots__ = ("seeds",)
    SEEDS_FIELD_NUMBER: _ClassVar[int]
    seeds: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, seeds: _Optional[_Iterable[int]] = ...) -> None: ...

class ResetResponse(_message.Message):
    __slots__ = ("observations", "obs_size")
    OBSERVATIONS_FIELD_NUMBER: _ClassVar[int]
    OBS_SIZE_FIELD_NUMBER: _ClassVar[int]
    observations: _containers.RepeatedCompositeFieldContainer[Observation]
    obs_size: int
    def __init__(self, observations: _Optional[_Iterable[_Union[Observation, _Mapping]]] = ..., obs_size: _Optional[int] = ...) -> None: ...

class StepRequest(_message.Message):
    __slots__ = ("actions",)
    ACTIONS_FIELD_NUMBER: _ClassVar[int]
    actions: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, actions: _Optional[_Iterable[int]] = ...) -> None: ...

class StepResponse(_message.Message):
    __slots__ = ("observations", "rewards", "terminated", "truncated", "final_observations")
    OBSERVATIONS_FIELD_NUMBER: _ClassVar[int]
    REWARDS_FIELD_NUMBER: _ClassVar[int]
    TERMINATED_FIELD_NUMBER: _ClassVar[int]
    TRUNCATED_FIELD_NUMBER: _ClassVar[int]
    FINAL_OBSERVATIONS_FIELD_NUMBER: _ClassVar[int]
    observations: _containers.RepeatedCompositeFieldContainer[Observation]
    rewards: _containers.RepeatedScalarFieldContainer[float]
    terminated: _containers.RepeatedScalarFieldContainer[bool]
    truncated: _containers.RepeatedScalarFieldContainer[bool]
    final_observations: _containers.RepeatedCompositeFieldContainer[Observation]
    def __init__(self, observations: _Optional[_Iterable[_Union[Observation, _Mapping]]] = ..., rewards: _Optional[_Iterable[float]] = ..., terminated: _Optional[_Iterable[bool]] = ..., truncated: _Optional[_Iterable[bool]] = ..., final_observations: _Optional[_Iterable[_Union[Observation, _Mapping]]] = ...) -> None: ...
