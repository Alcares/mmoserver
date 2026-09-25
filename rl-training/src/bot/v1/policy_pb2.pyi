from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Activation(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ACTIVATION_UNSPECIFIED: _ClassVar[Activation]
    ACTIVATION_TANH: _ClassVar[Activation]
ACTIVATION_UNSPECIFIED: Activation
ACTIVATION_TANH: Activation

class Policy(_message.Message):
    __slots__ = ("obs_size", "action_count", "activation", "layers")
    OBS_SIZE_FIELD_NUMBER: _ClassVar[int]
    ACTION_COUNT_FIELD_NUMBER: _ClassVar[int]
    ACTIVATION_FIELD_NUMBER: _ClassVar[int]
    LAYERS_FIELD_NUMBER: _ClassVar[int]
    obs_size: int
    action_count: int
    activation: Activation
    layers: _containers.RepeatedCompositeFieldContainer[Layer]
    def __init__(self, obs_size: _Optional[int] = ..., action_count: _Optional[int] = ..., activation: _Optional[_Union[Activation, str]] = ..., layers: _Optional[_Iterable[_Union[Layer, _Mapping]]] = ...) -> None: ...

class Layer(_message.Message):
    __slots__ = ("in_features", "out_features", "weight", "bias")
    IN_FEATURES_FIELD_NUMBER: _ClassVar[int]
    OUT_FEATURES_FIELD_NUMBER: _ClassVar[int]
    WEIGHT_FIELD_NUMBER: _ClassVar[int]
    BIAS_FIELD_NUMBER: _ClassVar[int]
    in_features: int
    out_features: int
    weight: _containers.RepeatedScalarFieldContainer[float]
    bias: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, in_features: _Optional[int] = ..., out_features: _Optional[int] = ..., weight: _Optional[_Iterable[float]] = ..., bias: _Optional[_Iterable[float]] = ...) -> None: ...
