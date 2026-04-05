from __future__ import annotations

from dataclasses import asdict, dataclass, field, is_dataclass
from typing import Any, Dict, List, Optional


def _serialize(value: Any) -> Any:
    if is_dataclass(value):
        return {key: _serialize(item) for key, item in asdict(value).items()}
    if isinstance(value, dict):
        return {key: _serialize(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_serialize(item) for item in value]
    if isinstance(value, set):
        return sorted(_serialize(item) for item in value)
    return value


def empty_findings() -> Dict[str, List[str]]:
    return {
        "business_logic": [],
        "frontend_orchestration": [],
        "presentation": [],
        "ambiguities": [],
    }


@dataclass
class BindingInfo:
    expression: str
    references: List[str] = field(default_factory=list)
    category_hint: str = "unknown"
    source_key: str = ""
    action_calls: List[str] = field(default_factory=list)
    js_calls: List[str] = field(default_factory=list)
    widget_refs: List[str] = field(default_factory=list)


@dataclass
class EventStep:
    step_type: str
    name: str
    detail: str = ""


@dataclass
class EventHandler:
    event_name: str
    expression: str
    action_calls: List[str] = field(default_factory=list)
    js_calls: List[str] = field(default_factory=list)
    widget_refs: List[str] = field(default_factory=list)
    steps: List[EventStep] = field(default_factory=list)
    notes: List[str] = field(default_factory=list)


@dataclass
class WidgetInfo:
    name: str
    widget_type: str
    path: str
    role: str = "unknown"
    bindings: List[BindingInfo] = field(default_factory=list)
    event_keys: List[str] = field(default_factory=list)
    event_handlers: List[EventHandler] = field(default_factory=list)
    visibility_bindings: List[str] = field(default_factory=list)
    disabled_bindings: List[str] = field(default_factory=list)
    default_bindings: List[str] = field(default_factory=list)


@dataclass
class QueryAnalysis:
    operation_type: str = "UNKNOWN"
    mode: str = "unknown"
    entities: List[str] = field(default_factory=list)
    parameters: List[str] = field(default_factory=list)
    backend_responsibility: str = "unknown"
    endpoint: Optional[str] = None
    http_method: Optional[str] = None
    logic_category: str = "unknown"
    notes: List[str] = field(default_factory=list)


@dataclass
class ActionInfo:
    name: str
    action_type: str
    path: str
    plugin_id: Optional[str] = None
    datasource: Optional[str] = None
    execute_on_load: Optional[bool] = None
    body: Optional[str] = None
    bindings: List[BindingInfo] = field(default_factory=list)
    raw_config_keys: List[str] = field(default_factory=list)
    parameters: List[str] = field(default_factory=list)
    references: List[str] = field(default_factory=list)
    query_analysis: QueryAnalysis = field(default_factory=QueryAnalysis)
    proxy_of: Optional[str] = None


@dataclass
class JSMethodInfo:
    name: str
    args: List[str] = field(default_factory=list)
    async_method: bool = False
    body: str = ""
    references: List[str] = field(default_factory=list)
    query_calls: List[str] = field(default_factory=list)
    js_calls: List[str] = field(default_factory=list)
    widget_refs: List[str] = field(default_factory=list)
    logic_patterns: List[str] = field(default_factory=list)
    role: str = "unknown"
    notes: List[str] = field(default_factory=list)


@dataclass
class JSObjectInfo:
    name: str
    path: str
    methods: List[JSMethodInfo] = field(default_factory=list)
    references: List[str] = field(default_factory=list)
    findings: List[str] = field(default_factory=list)


@dataclass
class DatasourceInfo:
    name: str
    plugin_id: Optional[str] = None
    plugin_type: str = "unknown"
    path: str = ""
    actions: List[str] = field(default_factory=list)
    notes: List[str] = field(default_factory=list)


@dataclass
class DependencyNode:
    node_id: str
    node_type: str
    name: str
    page: Optional[str] = None
    path: str = ""


@dataclass
class DependencyEdge:
    source: str
    target: str
    relation: str
    page: Optional[str] = None
    evidence: str = ""


@dataclass
class EventFlow:
    name: str
    trigger: str
    source_type: str
    source_name: str
    steps: List[EventStep] = field(default_factory=list)
    notes: List[str] = field(default_factory=list)


@dataclass
class DomainEntityInfo:
    name: str
    source_names: List[str] = field(default_factory=list)
    evidence: List[str] = field(default_factory=list)
    operations: List[str] = field(default_factory=list)
    pages: List[str] = field(default_factory=list)
    confidence: str = "medium"
    role: str = "supporting"


@dataclass
class PageInfo:
    name: str
    path: str
    purpose: str = ""
    page_type: str = "unknown"
    main_entity: Optional[str] = None
    main_entity_confidence: str = "ambiguous"
    candidate_entities: List[str] = field(default_factory=list)
    actions_available: List[str] = field(default_factory=list)
    page_load_actions: List[str] = field(default_factory=list)
    widgets: List[WidgetInfo] = field(default_factory=list)
    actions: List[ActionInfo] = field(default_factory=list)
    js_objects: List[JSObjectInfo] = field(default_factory=list)
    event_flows: List[EventFlow] = field(default_factory=list)
    dependencies: List[DependencyEdge] = field(default_factory=list)
    findings: List[str] = field(default_factory=list)
    findings_by_category: Dict[str, List[str]] = field(default_factory=empty_findings)


@dataclass
class AppInventory:
    source: str
    source_type: str
    application_name: Optional[str] = None
    declared_pages: List[str] = field(default_factory=list)
    pages: List[PageInfo] = field(default_factory=list)
    datasources: List[DatasourceInfo] = field(default_factory=list)
    entities: List[DomainEntityInfo] = field(default_factory=list)
    dependency_graph: Dict[str, List[Any]] = field(
        default_factory=lambda: {"nodes": [], "edges": []}
    )
    global_findings: List[str] = field(default_factory=list)
    findings_by_category: Dict[str, List[str]] = field(default_factory=empty_findings)

    def to_dict(self) -> Dict[str, Any]:
        return _serialize(self)
