from __future__ import annotations

import re
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence, Set, Tuple

from .models import (
    ActionInfo,
    AppInventory,
    BindingInfo,
    DatasourceInfo,
    DependencyEdge,
    DependencyNode,
    DomainEntityInfo,
    EventFlow,
    EventHandler,
    EventStep,
    JSMethodInfo,
    JSObjectInfo,
    PageInfo,
    QueryAnalysis,
    WidgetInfo,
    empty_findings,
)
from .source import SourceBundle, load_source

BINDING_RE = re.compile(r"\{\{([\s\S]*?)\}\}")
REFERENCE_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\b")
REFERENCE_PATH_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*(?:(?:\?\.|\.)[A-Za-z_][A-Za-z0-9_]*)+)\b")
RUN_CALL_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\.run\s*\(")
JS_CALL_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(")
APPSMITH_CALL_RE = re.compile(
    r"\b(showModal|closeModal|openModal|navigateTo|showAlert|storeValue|clearStore|resetWidget|download)\s*\("
)
SQL_ENTITY_RE = re.compile(
    r"\b(?:from|join|update|into|delete\s+from)\s+([A-Za-z0-9_\.\"`\[\]-]+)",
    re.IGNORECASE,
)
SQL_PARAMETER_RE = re.compile(
    r":([A-Za-z_][A-Za-z0-9_]*)|@([A-Za-z_][A-Za-z0-9_]*)|\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?"
)
STRING_LITERAL_RE = re.compile(r"(['\"`])(?:\\.|(?!\1).)*\1")

KNOWN_KEYWORDS = {
    "true",
    "false",
    "null",
    "undefined",
    "return",
    "if",
    "else",
    "for",
    "while",
    "switch",
    "case",
    "const",
    "let",
    "var",
    "function",
    "await",
    "async",
    "new",
    "this",
    "typeof",
    "instanceof",
    "in",
    "of",
}
IGNORE_REFERENCE_PREFIXES = {
    "moment",
    "Intl",
    "Math",
    "Date",
    "Object",
    "Array",
    "String",
    "Number",
    "Boolean",
    "JSON",
    "console",
    "Promise",
    "encodeURIComponent",
    "decodeURIComponent",
    "appsmith",
    "storeValue",
    "navigateTo",
    "showAlert",
    "closeModal",
    "openModal",
    "showModal",
    "resetWidget",
    "clearStore",
    "download",
    "postWindowMessage",
    "setInterval",
    "clearInterval",
    "setTimeout",
    "clearTimeout",
}
IGNORE_JS_METHOD_NAMES = {
    "map",
    "filter",
    "reduce",
    "sort",
    "forEach",
    "includes",
    "push",
    "then",
    "catch",
    "finally",
    "setSelectedOptions",
    "setSelectedOptionValues",
    "setText",
    "setValue",
    "setData",
    "trim",
    "toLowerCase",
    "toUpperCase",
}
ENTITY_STOPWORDS = {
    "query",
    "select",
    "insert",
    "update",
    "delete",
    "disable",
    "enable",
    "get",
    "set",
    "save",
    "new",
    "edit",
    "all",
    "list",
    "modal",
    "button",
    "table",
    "form",
    "container",
    "widget",
    "data",
    "details",
    "mockup",
    "main",
    "public",
    "utils",
    "copy",
    "run",
    "id",
    "by",
    "crm",
    "object",
    "objects",
    "association",
    "associations",
    "file",
    "files",
    "note",
    "notes",
    "render",
    "download",
    "container",
    "text",
    "image",
    "button",
    "group",
    "label",
    "home",
    "app",
    "title",
    "pending",
    "pdf",
    "upload",
    "create",
    "associate",
    "nuova",
    "elenco",
    "dettaglio",
    "report",
    "approval",
}
PRESENTATION_WIDGET_TYPES = {"TEXT_WIDGET", "ICON_WIDGET", "IMAGE_WIDGET", "DIVIDER_WIDGET"}
TABLE_WIDGET_TYPES = {"TABLE_WIDGET", "TABLE_WIDGET_V2"}
FORM_WIDGET_TYPES = {
    "FORM_WIDGET",
    "INPUT_WIDGET_V2",
    "SELECT_WIDGET",
    "MULTI_SELECT_WIDGET",
    "MULTI_SELECT_WIDGET_V2",
    "CURRENCY_INPUT_WIDGET",
    "SWITCH_WIDGET",
}
DASHBOARD_WIDGET_TYPES = {"CHART_WIDGET", "STATBOX_WIDGET"}
GENERIC_WIDGET_ENTITY_NAMES = {
    "container",
    "container1",
    "container2",
    "text",
    "text1",
    "text2",
    "image",
    "image1",
    "button",
    "button1",
    "buttongroup1",
    "maincontainer",
    "tabs1",
}
GENERIC_API_SEGMENTS = {
    "crm",
    "objects",
    "object",
    "associations",
    "association",
    "files",
    "file",
    "notes",
    "note",
    "api",
    "internal",
    "generate",
}
SQL_PSEUDO_ENTITIES = {"lateral", "jsonb_array_elements", "unnest", "generate_series"}
CRM_INFRASTRUCTURE_NOUNS = {
    "owner", "contact", "company", "pipeline", "stage",
    "deal", "account", "opportunity", "lead", "campaign",
    "territory", "property", "team", "role",
}
TECHNICAL_ENTITY_PREFIXES = {"hub", "hubspot"}
TECHNICAL_ENTITY_TOKENS = {
    "api",
    "appsmith",
    "button",
    "chart",
    "container",
    "icon",
    "image",
    "jsobject",
    "label",
    "modal",
    "pipeline",
    "rest",
    "table",
    "text",
    "widget",
}


def first_present(data: Dict[str, Any], *keys: str, default: Optional[str] = None) -> Optional[str]:
    for key in keys:
        value = data.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return default


def infer_page_name(path: str, data: Optional[Dict[str, Any]] = None) -> str:
    if data:
        unpublished = data.get("unpublishedPage")
        if isinstance(unpublished, dict):
            explicit = first_present(unpublished, "name")
            if explicit:
                return explicit
        explicit = first_present(data, "pageName", "name")
        if explicit:
            return explicit
    parts = Path(path).parts
    if "pages" in parts:
        index = parts.index("pages")
        if index + 1 < len(parts):
            return parts[index + 1]
    return Path(path).stem


def get_page_bucket(inventory: AppInventory, page_name: str, page_path: str) -> PageInfo:
    for page in inventory.pages:
        if page.name == page_name:
            return page
    page = PageInfo(name=page_name, path=page_path)
    inventory.pages.append(page)
    return page


def unique_ordered(values: Iterable[str]) -> List[str]:
    seen: Set[str] = set()
    ordered: List[str] = []
    for value in values:
        if value and value not in seen:
            ordered.append(value)
            seen.add(value)
    return ordered


def split_identifier(value: str) -> List[str]:
    parts = re.sub(r"([a-z0-9])([A-Z])", r"\1 \2", value)
    parts = re.sub(r"[^A-Za-z0-9]+", " ", parts)
    return [part for part in parts.split() if part]


def singularize(token: str) -> str:
    lowered = token.lower()
    if lowered.endswith("ies") and len(lowered) > 4:
        return token[:-3] + "y"
    if lowered.endswith("s") and not lowered.endswith("ss") and len(lowered) > 3:
        return token[:-1]
    return token


def normalize_entity_label(value: str) -> str:
    tokens = [token for token in split_identifier(value) if token.lower() not in ENTITY_STOPWORDS]
    while len(tokens) > 1 and singularize(tokens[0]).lower() in TECHNICAL_ENTITY_PREFIXES:
        tokens = tokens[1:]
    if not tokens:
        return ""
    cleaned = [
        singularize(token)
        for token in tokens
        if not re.fullmatch(r"(?:chart|widget|table|button|text|image|icon|container|modal|statbox)\d*", token, re.IGNORECASE)
    ]
    if not cleaned:
        return ""
    return " ".join(token.capitalize() for token in cleaned)


def infer_query_name_from_path(path: str) -> str:
    return Path(path).parent.name


def infer_jsobject_name_from_path(path: str) -> str:
    return Path(path).parent.name


def clean_sql_entity(raw: str) -> str:
    entity = raw.strip().strip(";")
    entity = entity.replace('"', "").replace("`", "").replace("[", "").replace("]", "")
    parts = [part for part in entity.split(".") if part and part.lower() not in {"dbo", "public"}]
    return parts[-1] if parts else entity


def clean_api_path(path: str) -> str:
    return re.sub(r"\s+", " ", path.strip())


def masked_char(char: str) -> str:
    return "\n" if char == "\n" else " "


def scrub_string_literals(expression: str) -> str:
    chars = list(expression)
    length = len(chars)

    def consume_quoted(start: int, quote: str) -> int:
        chars[start] = masked_char(chars[start])
        index = start + 1
        while index < length:
            current = chars[index]
            chars[index] = masked_char(current)
            if current == "\\" and index + 1 < length:
                index += 1
                chars[index] = masked_char(chars[index])
            elif current == quote:
                return index + 1
            index += 1
        return index

    def consume_template_expression(start: int) -> int:
        depth = 1
        index = start
        while index < length and depth > 0:
            current = chars[index]
            if current == "/" and index + 1 < length and chars[index + 1] == "*":
                index += 2
                while index < length:
                    if chars[index] == "*" and index + 1 < length and chars[index + 1] == "/":
                        index += 2
                        break
                    index += 1
                continue
            if current == "/" and index + 1 < length and chars[index + 1] == "/":
                index += 2
                while index < length and chars[index] != "\n":
                    index += 1
                continue
            if current in {"'", '"'}:
                index = consume_quoted(index, current)
                continue
            if current == "`":
                index = consume_template(index)
                continue
            if current == "{":
                depth += 1
            elif current == "}":
                depth -= 1
                if depth == 0:
                    return index + 1
            index += 1
        return index

    def consume_template(start: int) -> int:
        chars[start] = masked_char(chars[start])
        index = start + 1
        while index < length:
            current = chars[index]
            if current == "\\" and index + 1 < length:
                chars[index] = masked_char(current)
                index += 1
                chars[index] = masked_char(chars[index])
                index += 1
                continue
            if current == "`":
                chars[index] = masked_char(current)
                return index + 1
            if current == "$" and index + 1 < length and chars[index + 1] == "{":
                chars[index] = masked_char(chars[index])
                index += 1
                index = consume_template_expression(index + 1)
                continue
            chars[index] = masked_char(current)
            index += 1
        return index

    index = 0
    while index < length:
        current = chars[index]
        if current in {"'", '"'}:
            index = consume_quoted(index, current)
            continue
        if current == "`":
            index = consume_template(index)
            continue
        index += 1
    return "".join(chars)


def canonicalize_reference_path(value: str) -> str:
    return value.replace("?.", ".")


def extract_reference_paths(expression: str) -> List[str]:
    refs: List[str] = []
    seen: Set[str] = set()
    for match in REFERENCE_PATH_RE.finditer(scrub_string_literals(expression)):
        path = canonicalize_reference_path(match.group(1))
        root = path.split(".", 1)[0]
        if (root in KNOWN_KEYWORDS and root != "this") or root in IGNORE_REFERENCE_PREFIXES or root.isdigit():
            continue
        if path not in seen:
            refs.append(path)
            seen.add(path)
    return refs


def extract_references(expression: str) -> List[str]:
    refs: List[str] = []
    seen: Set[str] = set()
    scrubbed = scrub_string_literals(expression)
    for path in extract_reference_paths(scrubbed):
        root = path.split(".", 1)[0]
        if root == "this":
            continue
        if root not in seen:
            refs.append(root)
            seen.add(root)
    remainder = REFERENCE_PATH_RE.sub(" ", scrubbed)
    for token in REFERENCE_RE.findall(remainder):
        if token in KNOWN_KEYWORDS or token in IGNORE_REFERENCE_PREFIXES or token.isdigit():
            continue
        if token not in seen:
            refs.append(token)
            seen.add(token)
    return refs


def dedupe_bindings(bindings: Sequence[BindingInfo]) -> List[BindingInfo]:
    seen: Set[Tuple[str, str]] = set()
    deduped: List[BindingInfo] = []
    for binding in bindings:
        key = (binding.source_key, binding.expression)
        if key in seen:
            continue
        deduped.append(binding)
        seen.add(key)
    return deduped


def next_significant_char(expression: str, index: int) -> str:
    while index < len(expression):
        if not expression[index].isspace():
            return expression[index]
        index += 1
    return ""


def previous_significant_char(expression: str, index: int) -> str:
    while index >= 0:
        if not expression[index].isspace():
            return expression[index]
        index -= 1
    return ""


def extract_bare_input_identifiers(expression: str) -> List[str]:
    scrubbed = scrub_string_literals(expression)
    remainder = REFERENCE_PATH_RE.sub(lambda match: " " * (match.end() - match.start()), scrubbed)
    refs: List[str] = []
    seen: Set[str] = set()
    for match in REFERENCE_RE.finditer(remainder):
        token = match.group(1)
        if token in KNOWN_KEYWORDS or token in IGNORE_REFERENCE_PREFIXES or token.isdigit():
            continue
        next_char = next_significant_char(remainder, match.end())
        prev_char = previous_significant_char(remainder, match.start() - 1)
        if next_char == ":" and prev_char in {"", "{", ",", "("}:
            continue
        if token not in seen:
            refs.append(token)
            seen.add(token)
    return refs


def extract_input_references(bindings: Sequence[BindingInfo]) -> List[str]:
    refs: List[str] = []
    for binding in bindings:
        refs.extend(extract_reference_paths(binding.expression))
        refs.extend(extract_bare_input_identifiers(binding.expression))
    return unique_ordered(refs)


def mask_js_comments(expression: str) -> str:
    chars = list(expression)
    index = 0
    length = len(chars)
    while index < length:
        current = chars[index]
        if current == "/" and index + 1 < length and chars[index + 1] == "/":
            chars[index] = masked_char(chars[index])
            index += 1
            chars[index] = masked_char(chars[index])
            index += 1
            while index < length and chars[index] != "\n":
                chars[index] = masked_char(chars[index])
                index += 1
            continue
        if current == "/" and index + 1 < length and chars[index + 1] == "*":
            chars[index] = masked_char(chars[index])
            index += 1
            chars[index] = masked_char(chars[index])
            index += 1
            while index < length:
                current = chars[index]
                chars[index] = masked_char(current)
                if current == "*" and index + 1 < length and chars[index + 1] == "/":
                    index += 1
                    chars[index] = masked_char(chars[index])
                    index += 1
                    break
                index += 1
            continue
        index += 1
    return "".join(chars)


def find_matching_brace(parse_view: str, brace_start: int) -> int:
    depth = 0
    for index in range(brace_start, len(parse_view)):
        char = parse_view[index]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return index
    return len(parse_view)


def ranges_overlap(left: Tuple[int, int], right: Tuple[int, int]) -> bool:
    return left[0] < right[1] and right[0] < left[1]


def parse_call_steps(expression: str) -> List[EventStep]:
    matches: List[Tuple[int, EventStep]] = []
    for match in RUN_CALL_RE.finditer(expression):
        matches.append((match.start(), EventStep(step_type="query", name=match.group(1))))
    for match in JS_CALL_RE.finditer(expression):
        owner, method = match.groups()
        if owner in IGNORE_REFERENCE_PREFIXES or method == "run" or method in IGNORE_JS_METHOD_NAMES:
            continue
        matches.append(
            (
                match.start(),
                EventStep(step_type="js_method", name=f"{owner}.{method}", detail=f"JSObject method `{owner}.{method}`"),
            )
        )
    for match in APPSMITH_CALL_RE.finditer(expression):
        matches.append(
            (
                match.start(),
                EventStep(step_type="appsmith", name=match.group(1), detail="Appsmith built-in interaction"),
            )
        )
    matches.sort(key=lambda item: item[0])
    ordered: List[EventStep] = []
    seen: Set[Tuple[str, str]] = set()
    for _, step in matches:
        key = (step.step_type, step.name)
        if key not in seen:
            ordered.append(step)
            seen.add(key)
    return ordered


def classify_binding(expression: str, source_key: str) -> str:
    key = source_key.lower()
    expr = expression.lower()
    if "visible" in key or "hidden" in key or "style" in key or "color" in key:
        return "presentation"
    if "disabled" in key or key.startswith("on") or "click" in key or "submit" in key:
        return "frontend_orchestration"
    if any(token in expr for token in ["delete", "update", "insert", "create", "status", "role", "permission", "approve"]):
        return "business_logic"
    if any(token in expr for token in ["selectedrow", "selectedoptionvalue", "text", "isvalid", "trigger", ".run("]):
        return "frontend_orchestration"
    return "unknown"


def extract_bindings(value: Any, source_key: str = "") -> List[BindingInfo]:
    bindings: List[BindingInfo] = []
    if isinstance(value, str):
        for match in BINDING_RE.finditer(value):
            expression = match.group(1).strip()
            steps = parse_call_steps(expression)
            bindings.append(
                BindingInfo(
                    expression=expression,
                    references=extract_references(expression),
                    category_hint=classify_binding(expression, source_key),
                    source_key=source_key,
                    action_calls=[step.name for step in steps if step.step_type == "query"],
                    js_calls=[step.name for step in steps if step.step_type == "js_method"],
                )
            )
    elif isinstance(value, dict):
        for key, item in value.items():
            bindings.extend(extract_bindings(item, key))
    elif isinstance(value, list):
        for item in value:
            bindings.extend(extract_bindings(item, source_key))
    return bindings


def infer_widget_role(widget_type: str, name: str) -> str:
    if widget_type in TABLE_WIDGET_TYPES:
        return "tabular_data"
    if widget_type in FORM_WIDGET_TYPES or "form" in name.lower():
        return "data_entry"
    if widget_type in DASHBOARD_WIDGET_TYPES:
        return "dashboard"
    if "modal" in name.lower():
        return "dialog"
    if widget_type in PRESENTATION_WIDGET_TYPES:
        return "presentation"
    if "button" in widget_type.lower() or "btn" in name.lower():
        return "trigger"
    return "unknown"


def parse_event_handlers(widget_data: Dict[str, Any]) -> List[EventHandler]:
    handlers: List[EventHandler] = []
    for key, value in widget_data.items():
        if not isinstance(value, str):
            continue
        lowered = key.lower()
        if not (lowered.startswith("on") or lowered.endswith("action")):
            continue
        bindings = extract_bindings(value, key)
        expression = bindings[0].expression if bindings else value.strip()
        steps = parse_call_steps(expression)
        notes: List[str] = []
        if ".then(" in expression or "await " in expression:
            notes.append("Asynchronous chaining detected in widget event handler.")
        if len([step for step in steps if step.step_type in {"query", "js_method"}]) > 1:
            notes.append("Handler triggers multiple actions and likely coordinates a workflow.")
        handlers.append(
            EventHandler(
                event_name=key,
                expression=expression,
                action_calls=unique_ordered(step.name for step in steps if step.step_type == "query"),
                js_calls=unique_ordered(step.name for step in steps if step.step_type == "js_method"),
                steps=steps,
                notes=notes,
            )
        )
    return handlers


def parse_widget(path: str, data: Dict[str, Any]) -> WidgetInfo:
    bindings = extract_bindings(data)
    return WidgetInfo(
        name=first_present(data, "widgetName", "name", default=Path(path).stem) or Path(path).stem,
        widget_type=first_present(data, "type", "widgetType", default="unknown") or "unknown",
        path=path,
        role=infer_widget_role(
            first_present(data, "type", "widgetType", default="unknown") or "unknown",
            first_present(data, "widgetName", "name", default=Path(path).stem) or Path(path).stem,
        ),
        bindings=bindings,
        event_keys=sorted(key for key in data.keys() if key.lower().startswith("on") or "action" in key.lower()),
        event_handlers=parse_event_handlers(data),
        visibility_bindings=[binding.expression for binding in bindings if binding.source_key.lower() in {"visible", "isvisible"}],
        disabled_bindings=[binding.expression for binding in bindings if "disabled" in binding.source_key.lower()],
        default_bindings=[binding.expression for binding in bindings if "default" in binding.source_key.lower()],
    )


def detect_sql_operation(sql: str) -> str:
    lowered = sql.lower()
    matches = list(re.finditer(r"\b(select|insert|update|delete)\b", lowered))
    return matches[0].group(1).upper() if matches else "UNKNOWN"


def extract_sql_entities(sql: str) -> List[str]:
    entities = []
    for match in SQL_ENTITY_RE.finditer(sql):
        entity = clean_sql_entity(match.group(1))
        if not entity:
            continue
        if entity.lower() in SQL_PSEUDO_ENTITIES or "(" in entity:
            continue
        entities.append(entity)
    return unique_ordered(entities)


def extract_sql_parameters(sql: str, bindings: Sequence[BindingInfo]) -> List[str]:
    params = [token for groups in SQL_PARAMETER_RE.findall(sql) for token in groups if token]
    params.extend(extract_input_references(bindings))
    return unique_ordered(params)


def describe_backend_responsibility(mode: str, entities: Sequence[str], action_type: str) -> str:
    if entities:
        entity_list = ", ".join(entities)
        if mode == "read":
            return f"Backend/API should expose read access for {entity_list}."
        if mode == "write":
            return f"Backend/API should own persistence workflow for {entity_list}."
        return f"Backend/API responsibility should be clarified for {entity_list}."
    if action_type == "API":
        return "Backend/API contract exists, but the domain responsibility is ambiguous."
    if action_type == "DB":
        return "Direct database access is embedded in Appsmith and should move behind backend APIs."
    return "Responsibility requires manual review."


def analyze_db_action(name: str, body: str, bindings: Sequence[BindingInfo]) -> QueryAnalysis:
    operation = detect_sql_operation(body)
    mode = "read" if operation == "SELECT" else "write" if operation in {"INSERT", "UPDATE", "DELETE"} else "unknown"
    entities = extract_sql_entities(body)
    notes: List[str] = []
    if mode == "write":
        notes.append("Write query is triggered from the Appsmith client.")
    if "{{" in body:
        notes.append("Dynamic bindings are embedded directly inside the SQL body.")
    if not entities:
        notes.append("No SQL entities were confidently extracted.")
    logic_category = "business_logic" if mode == "write" or " where " in body.lower() else "frontend_orchestration"
    return QueryAnalysis(
        operation_type=operation,
        mode=mode,
        entities=entities,
        parameters=extract_sql_parameters(body, bindings),
        backend_responsibility=describe_backend_responsibility(mode, entities, "DB"),
        logic_category=logic_category,
        notes=notes,
    )


def extract_api_entities(path_value: str, action_name: str) -> List[str]:
    path_entities: List[str] = []
    for part in re.split(r"/+", path_value):
        part = re.sub(r"\{\{[\s\S]*?\}\}", "", part).strip()
        if not part or re.fullmatch(r"v\d+", part.lower()):
            continue
        lowered = part.lower()
        if "_to_" in lowered:
            continue
        if lowered in {"api", "arak", "rest"} | GENERIC_API_SEGMENTS:
            continue
        label = normalize_entity_label(part)
        if label:
            path_entities.append(label)
    return unique_ordered(path_entities)


def analyze_api_action(name: str, body: str, cfg: Dict[str, Any], bindings: Sequence[BindingInfo]) -> QueryAnalysis:
    method = str(cfg.get("httpMethod") or "UNKNOWN").upper()
    path_value = clean_api_path(str(cfg.get("path") or ""))
    mode = "read" if method in {"GET", "HEAD"} else "write" if method in {"POST", "PUT", "PATCH", "DELETE"} else "unknown"
    entities = extract_api_entities(path_value, name)
    params: List[str] = list(extract_input_references(bindings))
    for item in cfg.get("queryParameters") or []:
        if isinstance(item, dict) and item.get("key"):
            params.append(str(item["key"]))
    notes: List[str] = []
    if path_value:
        notes.append(f"API path: {path_value}")
    if "{{" in body or "{{" in path_value:
        notes.append("Dynamic path/body bindings detected in API request.")
    if mode == "write":
        notes.append("State-changing API request is initiated from Appsmith.")
    return QueryAnalysis(
        operation_type=method,
        mode=mode,
        entities=entities,
        parameters=unique_ordered(params),
        backend_responsibility=describe_backend_responsibility(mode, entities, "API"),
        endpoint=path_value or None,
        http_method=method,
        logic_category="business_logic" if mode == "write" else "frontend_orchestration",
        notes=notes,
    )


def analyze_js_proxy_action(name: str, cfg: Dict[str, Any]) -> QueryAnalysis:
    args = []
    for item in cfg.get("jsArguments") or []:
        if isinstance(item, dict) and item.get("name"):
            args.append(str(item["name"]))
    return QueryAnalysis(
        operation_type="JS_METHOD",
        mode="unknown",
        entities=[],
        parameters=unique_ordered(args),
        backend_responsibility="JSObject method proxy; inspect the backing JSObject for actual responsibility.",
        logic_category="frontend_orchestration",
        notes=["This action is a generated proxy to a JSObject method, not a datasource call."],
    )


def parse_action_metadata(path: str, data: Dict[str, Any], text_body: str) -> ActionInfo:
    action = data.get("unpublishedAction") if isinstance(data.get("unpublishedAction"), dict) else data
    cfg = action.get("actionConfiguration") if isinstance(action.get("actionConfiguration"), dict) else {}
    datasource = action.get("datasource")
    datasource_name = None
    if isinstance(datasource, dict):
        datasource_name = datasource.get("name") or datasource.get("id")
    elif isinstance(datasource, str):
        datasource_name = datasource

    body = text_body.strip() or (cfg.get("body") if isinstance(cfg.get("body"), str) else None)
    bindings = dedupe_bindings(extract_bindings(action))
    if body and body != cfg.get("body"):
        bindings.extend(extract_bindings(body, "body"))
    bindings = dedupe_bindings(bindings)

    plugin_type = first_present(data, "pluginType", default="unknown") or "unknown"
    plugin_id = first_present(data, "pluginId")
    name = first_present(action, "name", "actionName", default=infer_query_name_from_path(path)) or infer_query_name_from_path(path)
    if plugin_type == "DB":
        analysis = analyze_db_action(name, body or "", bindings)
    elif plugin_type == "API":
        analysis = analyze_api_action(name, body or "", cfg, bindings)
    elif plugin_type == "JS":
        analysis = analyze_js_proxy_action(name, cfg)
    else:
        analysis = QueryAnalysis(notes=["Unsupported or unknown action plugin type."])

    return ActionInfo(
        name=name,
        action_type=plugin_type,
        path=path,
        plugin_id=plugin_id,
        datasource=datasource_name,
        execute_on_load=bool(action.get("executeOnLoad")),
        body=body,
        bindings=bindings,
        raw_config_keys=sorted(cfg.keys()),
        parameters=analysis.parameters,
        references=unique_ordered(reference for binding in bindings for reference in binding.references),
        query_analysis=analysis,
        proxy_of=action.get("fullyQualifiedName"),
    )


def extract_export_object(js_body: str) -> str:
    parse_view = mask_js_comments(scrub_string_literals(js_body))
    start = parse_view.find("export default")
    if start == -1:
        return js_body
    brace_start = parse_view.find("{", start)
    if brace_start == -1:
        return js_body
    end_index = find_matching_brace(parse_view, brace_start)
    return js_body[brace_start + 1 : end_index]


def extract_js_methods(js_body: str) -> List[JSMethodInfo]:
    object_body = extract_export_object(js_body)
    parse_view = mask_js_comments(scrub_string_literals(object_body))
    methods: List[JSMethodInfo] = []
    signatures: List[Tuple[int, int, bool, str, str]] = []
    signature_spans: List[Tuple[int, int]] = []
    signature_patterns = [
        re.compile(
            r"(?:(?P<prefix_async>async)\s+)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*:\s*(?:(?P<body_async>async)\s*)?\((?P<args>[^)]*)\)\s*=>\s*\{",
            re.MULTILINE,
        ),
        re.compile(
            r"(?:(?P<prefix_async>async)\s+)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*:\s*(?:(?P<body_async>async)\s*)?function\s*\((?P<args>[^)]*)\)\s*\{",
            re.MULTILINE,
        ),
        re.compile(
            r"(?:(?P<prefix_async>async)\s+)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\((?P<args>[^)]*)\)\s*\{",
            re.MULTILINE,
        ),
    ]
    for pattern in signature_patterns:
        for match in pattern.finditer(parse_view):
            prefix = parse_view[: match.start()]
            if prefix.count("{") != prefix.count("}"):
                continue
            span = match.span()
            if any(ranges_overlap(span, existing) for existing in signature_spans):
                continue
            groups = match.groupdict()
            signatures.append(
                (
                    match.start(),
                    match.end() - 1,
                    bool(groups.get("prefix_async") or groups.get("body_async")),
                    groups["name"],
                    groups["args"],
                )
            )
            signature_spans.append(span)
    signatures.sort(key=lambda item: item[0])

    for _, brace_index, async_marker, name, args_text in signatures:
        end_index = find_matching_brace(parse_view, brace_index)
        method_body = object_body[brace_index + 1 : end_index]
        method_parse_view = parse_view[brace_index + 1 : end_index]
        references = extract_references(method_parse_view)
        query_calls = unique_ordered(match_.group(1) for match_ in RUN_CALL_RE.finditer(method_parse_view))
        js_calls = unique_ordered(
            f"{owner}.{method}"
            for owner, method in JS_CALL_RE.findall(method_parse_view)
            if owner not in IGNORE_REFERENCE_PREFIXES and method != "run" and method not in IGNORE_JS_METHOD_NAMES
        )
        logic_patterns: List[str] = []
        lowered = method_parse_view.lower()
        if "if (" in lowered or "if(" in lowered or "?" in method_parse_view:
            logic_patterns.append("conditional_branching")
        if any(token in lowered for token in [".map(", ".filter(", ".reduce(", ".sort(", ".foreach(", "for ("]):
            logic_patterns.append("collection_transformation")
        if "promise.all" in lowered:
            logic_patterns.append("parallel_execution")
        if any(token in lowered for token in ["showmodal(", "closemodal(", "navigateto(", "storevalue("]):
            logic_patterns.append("ui_orchestration")
        role = "frontend_orchestration" if query_calls or "ui_orchestration" in logic_patterns else "business_logic" if logic_patterns else "unknown"
        notes: List[str] = []
        if len(query_calls) > 1:
            notes.append("Method coordinates multiple queries.")
        if "collection_transformation" in logic_patterns:
            notes.append("Method performs client-side data shaping or diffing.")
        methods.append(
            JSMethodInfo(
                name=name,
                args=[arg.strip() for arg in args_text.split(",") if arg.strip()],
                async_method=async_marker or "await " in method_parse_view,
                body=method_body.strip(),
                references=references,
                query_calls=query_calls,
                js_calls=js_calls,
                logic_patterns=logic_patterns,
                role=role,
                notes=notes,
            )
        )
    return methods


def parse_jsobject(meta_path: str, meta: Dict[str, Any], js_body: str) -> JSObjectInfo:
    collection = meta.get("unpublishedCollection") if isinstance(meta.get("unpublishedCollection"), dict) else meta
    name = first_present(collection, "name", default=infer_jsobject_name_from_path(meta_path)) or infer_jsobject_name_from_path(meta_path)
    methods = extract_js_methods(js_body)
    findings: List[str] = []
    if any(method.query_calls for method in methods):
        findings.append("JSObject coordinates datasource/query execution.")
    if any("collection_transformation" in method.logic_patterns for method in methods):
        findings.append("JSObject contains client-side data transformation logic.")
    return JSObjectInfo(
        name=name,
        path=meta_path,
        methods=methods,
        references=unique_ordered(reference for method in methods for reference in method.references),
        findings=findings,
    )


def add_category_finding(findings: Dict[str, List[str]], category: str, message: str) -> None:
    bucket = category if category in findings else "ambiguities"
    if message not in findings[bucket]:
        findings[bucket].append(message)


def node_id(node_type: str, page_name: Optional[str], name: str) -> str:
    if page_name:
        return f"{node_type}:{page_name}:{name}"
    return f"{node_type}:{name}"


def build_page_dependency_context(page: PageInfo) -> Tuple[Set[str], Set[str], Set[str]]:
    action_names = {action.name for action in page.actions}
    widget_names = {widget.name for widget in page.widgets}
    js_method_names: Set[str] = set()
    for js_object in page.js_objects:
        for method in js_object.methods:
            js_method_names.add(f"{js_object.name}.{method.name}")
    return action_names, widget_names, js_method_names


def expand_steps(
    steps: Sequence[EventStep],
    method_index: Dict[str, JSMethodInfo],
    seen: Optional[Set[str]] = None,
) -> List[EventStep]:
    seen = seen or set()
    expanded: List[EventStep] = []
    for step in steps:
        if step.step_type == "js_method" and step.name not in method_index:
            continue
        expanded.append(step)
        if step.step_type == "js_method" and step.name in method_index and step.name not in seen:
            seen.add(step.name)
            nested_method = method_index[step.name]
            nested_steps = parse_call_steps(nested_method.body)
            expanded.extend(expand_steps(nested_steps, method_index, seen))
    return expanded


def is_technical_entity_label(label: str) -> bool:
    tokens = [token.lower() for token in split_identifier(label)]
    if not tokens:
        return True
    if len(tokens) == 1 and re.fullmatch(
        r"(?:chart|widget|table|button|text|image|icon|container|modal|statbox)\d*",
        tokens[0],
    ):
        return True
    return bool(set(tokens) & TECHNICAL_ENTITY_TOKENS)


def is_crm_infrastructure_entity_label(label: str) -> bool:
    return any(token.lower() in CRM_INFRASTRUCTURE_NOUNS for token in split_identifier(label))


def infer_page_entities(page: PageInfo) -> Tuple[List[str], Optional[str], str]:
    scores: Dict[str, Dict[str, int]] = {}

    def record(
        label: str,
        *,
        action_score: int = 0,
        structure_score: int = 0,
        explicit: int = 0,
        technical: int = 0,
        mode: str = "",
    ) -> None:
        if not label:
            return
        bucket = scores.setdefault(
            label,
            {
                "action_score": 0,
                "structure_score": 0,
                "explicit": 0,
                "technical": 0,
                "write_count": 0,
                "read_count": 0,
            },
        )
        bucket["action_score"] += action_score
        bucket["structure_score"] += structure_score
        bucket["explicit"] += explicit
        bucket["technical"] += technical
        if mode == "write":
            bucket["write_count"] += 1
        elif mode == "read":
            bucket["read_count"] += 1

    def effective_score(bucket: Dict[str, int]) -> int:
        return bucket["action_score"] + bucket["structure_score"]

    for action in page.actions:
        if action.action_type == "JS":
            continue
        for entity in action.query_analysis.entities:
            label = normalize_entity_label(entity) or entity
            if label:
                record(
                    label,
                    action_score=40 if action.query_analysis.mode == "write" else 30,
                    explicit=1,
                    technical=int(is_technical_entity_label(label)),
                    mode=action.query_analysis.mode,
                )
        label = normalize_entity_label(action.name)
        if not action.query_analysis.entities and label and not is_technical_entity_label(label):
            record(label, action_score=8, mode=action.query_analysis.mode)
    page_label = normalize_entity_label(page.name)
    if page_label and not is_technical_entity_label(page_label):
        record(page_label, structure_score=10)
    for widget in page.widgets:
        lowered = widget.name.lower()
        if lowered in GENERIC_WIDGET_ENTITY_NAMES:
            continue
        label = normalize_entity_label(widget.name)
        if label and label.lower() not in {"table", "button", "container", "text", "image"} and not is_technical_entity_label(label):
            record(label, structure_score=2)

    # Approach A: Demote read-only lookup entities when the page also writes
    written_entities = {label for label, b in scores.items() if b["write_count"] > 0}
    if written_entities:
        for label, bucket in scores.items():
            if label not in written_entities and bucket["read_count"] > 0 and bucket["write_count"] == 0:
                bucket["action_score"] = int(bucket["action_score"] * 0.5)

    # Approach B: Demote read-only CRM infrastructure entities when 3+ co-occur
    crm_matches = {
        label for label in scores
        if any(tok.lower() in CRM_INFRASTRUCTURE_NOUNS for tok in label.split())
    }
    if len(crm_matches) >= 3:
        for label in crm_matches:
            bucket = scores[label]
            if bucket["write_count"] == 0:
                bucket["action_score"] = int(bucket["action_score"] * 0.4)

    ranked = sorted(
        scores.items(),
        key=lambda item: (effective_score(item[1]), item[1]["explicit"], -item[1]["technical"], item[0]),
        reverse=True,
    )
    candidates = [label for label, _ in ranked[:5]]
    if not ranked:
        return [], None, "ambiguous"

    best_label, best = ranked[0]
    best_score = effective_score(best)
    runner_up_score = effective_score(ranked[1][1]) if len(ranked) > 1 else None
    if best["explicit"] >= 3 and best_score >= 90:
        confidence = "high"
    elif best["explicit"] >= 1 and best_score >= 30:
        confidence = "medium"
    elif best_score >= 12 and best["technical"] == 0:
        confidence = "low"
    else:
        confidence = "ambiguous"
    if runner_up_score is not None and best_score - runner_up_score < 5 and best["explicit"] <= 1:
        confidence = "ambiguous"
    if confidence == "ambiguous":
        return candidates, None, confidence
    return candidates, best_label, confidence


def infer_actions_available(page: PageInfo) -> List[str]:
    actions: List[str] = []
    for action in page.actions:
        operation = action.query_analysis.operation_type
        method = action.query_analysis.http_method
        name = action.name.lower()
        if operation in {"INSERT"} or method == "POST" or name.startswith(("create", "new", "ins")):
            actions.append("create")
        if operation in {"UPDATE"} or method in {"PUT", "PATCH"} or name.startswith(("update", "edit", "upd", "disable", "enable")):
            actions.append("update")
        if operation in {"DELETE"} or method == "DELETE" or name.startswith(("delete", "del", "remove")):
            actions.append("delete")
        if operation in {"SELECT"} or method in {"GET", "HEAD"} or name.startswith(("get", "list", "select")):
            actions.append("read")
        if "export" in name or "download" in name:
            actions.append("export")
    return unique_ordered(actions)


def infer_page_type(page: PageInfo) -> str:
    has_tables = any(widget.widget_type in TABLE_WIDGET_TYPES for widget in page.widgets)
    has_forms = any(widget.role == "data_entry" or "modal" in widget.name.lower() for widget in page.widgets)
    has_dashboard_widgets = any(widget.widget_type in DASHBOARD_WIDGET_TYPES for widget in page.widgets)
    has_writes = any(action.query_analysis.mode == "write" for action in page.actions)
    has_reads = any(action.query_analysis.mode == "read" for action in page.actions)
    if has_tables and has_forms and has_writes:
        return "crud"
    if has_tables and has_writes:
        return "crud"
    if has_reads and not has_writes:
        return "reporting"
    if has_tables and has_reads:
        return "reporting"
    if not page.actions and all(widget.role in {"presentation", "unknown"} for widget in page.widgets):
        return "landing"
    if has_dashboard_widgets:
        return "dashboard"
    if has_writes and not has_tables:
        return "workflow"
    return "unknown"


def infer_page_purpose(page: PageInfo) -> str:
    entity = page.main_entity or (page.candidate_entities[0] if page.candidate_entities else "domain data")
    if page.page_type == "crud":
        return f"Likely CRUD workflow for {entity}."
    if page.page_type == "reporting":
        return f"Likely reporting/listing page for {entity}."
    if page.page_type == "landing":
        return "Likely landing or navigational page with minimal embedded logic."
    if page.page_type == "dashboard":
        return f"Likely dashboard or monitoring page related to {entity}."
    if page.page_type == "workflow":
        return f"Likely task/workflow page that mutates {entity}."
    return f"Purpose is ambiguous; {entity} appears to be the dominant domain focus."


def register_datasource(inventory: AppInventory, datasource: DatasourceInfo) -> None:
    for existing in inventory.datasources:
        if existing.name == datasource.name:
            if datasource.plugin_id and not existing.plugin_id:
                existing.plugin_id = datasource.plugin_id
            if datasource.plugin_type != "unknown":
                existing.plugin_type = datasource.plugin_type
            if datasource.path and not existing.path:
                existing.path = datasource.path
            return
    inventory.datasources.append(datasource)


def find_datasource(inventory: AppInventory, name: str) -> Optional[DatasourceInfo]:
    simplified = re.sub(r"\s*\(.*\)$", "", name).strip().lower()
    for datasource in inventory.datasources:
        candidates = {
            datasource.name.lower(),
            re.sub(r"\s*\(.*\)$", "", datasource.name).strip().lower(),
        }
        if name.lower() in candidates or simplified in candidates:
            return datasource
    return None


def parse_datasources(bundle: SourceBundle, inventory: AppInventory) -> None:
    for path, raw in bundle.files.items():
        if "/datasources/" not in path or not path.endswith(".json"):
            continue
        data = bundle.json(path)
        if not isinstance(data, dict):
            continue
        datasource = DatasourceInfo(
            name=first_present(data, "name", default=Path(path).stem) or Path(path).stem,
            plugin_id=first_present(data, "pluginId"),
            plugin_type="API" if first_present(data, "pluginId", default="").startswith("restapi") else "DB" if "plugin" in (first_present(data, "pluginId", default="")) else "unknown",
            path=path,
        )
        register_datasource(inventory, datasource)


def parse_application_metadata(bundle: SourceBundle, inventory: AppInventory) -> None:
    for path in bundle.files:
        if path.endswith("/application.json") or path == "application.json":
            data = bundle.json(path)
            if not isinstance(data, dict):
                continue
            inventory.application_name = first_present(data, "name", "applicationName") or Path(path).parent.name
            inventory.declared_pages = [
                page.get("id")
                for page in data.get("pages", [])
                if isinstance(page, dict) and page.get("id")
            ]
            break


def add_graph_node(nodes: List[DependencyNode], seen: Set[str], node: DependencyNode) -> None:
    if node.node_id not in seen:
        nodes.append(node)
        seen.add(node.node_id)


def add_graph_edge(edges: List[DependencyEdge], seen: Set[Tuple[str, str, str, Optional[str]]], edge: DependencyEdge) -> None:
    key = (edge.source, edge.target, edge.relation, edge.page)
    if key not in seen:
        edges.append(edge)
        seen.add(key)


def summarize_duplication(inventory: AppInventory) -> List[str]:
    duplicates: Counter[Tuple[str, str]] = Counter()
    for page in inventory.pages:
        for action in page.actions:
            for entity in action.query_analysis.entities:
                duplicates[(action.query_analysis.operation_type, normalize_entity_label(entity) or entity)] += 1
    findings: List[str] = []
    for (operation, entity), count in duplicates.items():
        if count > 1 and entity:
            findings.append(f"{operation} logic for {entity} appears in {count} actions.")
    return findings


def aggregate_entities(inventory: AppInventory) -> List[DomainEntityInfo]:
    entity_map: Dict[str, DomainEntityInfo] = {}
    entity_has_write_action: Dict[str, bool] = {}
    primary_entities = {
        page.main_entity
        for page in inventory.pages
        if page.main_entity and page.main_entity_confidence in {"high", "medium"}
    }
    for page in inventory.pages:
        for action in page.actions:
            for entity in action.query_analysis.entities:
                label = normalize_entity_label(entity) or entity
                if not label:
                    continue
                if action.query_analysis.mode == "write":
                    entity_has_write_action[label] = True
                else:
                    entity_has_write_action.setdefault(label, False)
                info = entity_map.setdefault(label, DomainEntityInfo(name=label))
                info.source_names = unique_ordered(info.source_names + [entity, action.name])
                info.evidence = unique_ordered(info.evidence + [f"{page.name}: {action.name}"])
                info.operations = unique_ordered(info.operations + [action.query_analysis.operation_type])
                info.pages = unique_ordered(info.pages + [page.name])
                info.confidence = "high" if len(info.evidence) >= 3 else "medium"
        for candidate in page.candidate_entities:
            info = entity_map.get(candidate)
            if info is None:
                continue
            info.pages = unique_ordered(info.pages + [page.name])
            info.evidence = unique_ordered(info.evidence + [f"{page.name}: inferred from page structure"])
    for label, info in entity_map.items():
        if label in primary_entities:
            info.role = "primary"
        elif not entity_has_write_action.get(label, False) and is_crm_infrastructure_entity_label(label):
            info.role = "infrastructure"
        else:
            info.role = "supporting"
    return sorted(entity_map.values(), key=lambda item: item.name.lower())


def post_process_inventory(inventory: AppInventory) -> None:
    graph_nodes: List[DependencyNode] = []
    graph_edges: List[DependencyEdge] = []
    node_seen: Set[str] = set()
    edge_seen: Set[Tuple[str, str, str, Optional[str]]] = set()

    actual_pages = {page.name for page in inventory.pages}
    missing_pages = [page_name for page_name in inventory.declared_pages if page_name not in actual_pages]
    if missing_pages:
        inventory.global_findings.append("Declared pages without parsed artifacts: " + ", ".join(missing_pages))
        add_category_finding(
            inventory.findings_by_category,
            "ambiguities",
            "Declared pages are missing parsed artifacts: " + ", ".join(missing_pages),
        )

    for datasource in inventory.datasources:
        add_graph_node(
            graph_nodes,
            node_seen,
            DependencyNode(
                node_id=node_id("datasource", None, datasource.name),
                node_type="datasource",
                name=datasource.name,
                path=datasource.path,
            ),
        )

    for page in inventory.pages:
        add_graph_node(
            graph_nodes,
            node_seen,
            DependencyNode(node_id=node_id("page", page.name, page.name), node_type="page", name=page.name, page=page.name, path=page.path),
        )
        action_names, widget_names, js_method_names = build_page_dependency_context(page)
        method_index: Dict[str, JSMethodInfo] = {}
        js_object_names = {js_object.name for js_object in page.js_objects}
        for js_object in page.js_objects:
            add_graph_node(
                graph_nodes,
                node_seen,
                DependencyNode(
                    node_id=node_id("jsobject", page.name, js_object.name),
                    node_type="jsobject",
                    name=js_object.name,
                    page=page.name,
                    path=js_object.path,
                ),
            )
            for method in js_object.methods:
                full_name = f"{js_object.name}.{method.name}"
                method_index[full_name] = method
                method.widget_refs = unique_ordered(reference for reference in method.references if reference in widget_names)
                method.query_calls = unique_ordered(query for query in method.query_calls if query in action_names)
                method.js_calls = unique_ordered(call for call in method.js_calls if call in js_method_names)
                for query_name in method.query_calls:
                    edge = DependencyEdge(
                        source=node_id("jsobject", page.name, js_object.name),
                        target=node_id("action", page.name, query_name),
                        relation="calls_query",
                        page=page.name,
                        evidence=f"{full_name} invokes {query_name}.run(...)",
                    )
                    page.dependencies.append(edge)
                    add_graph_edge(graph_edges, edge_seen, edge)
                for widget_name in method.widget_refs:
                    edge = DependencyEdge(
                        source=node_id("jsobject", page.name, js_object.name),
                        target=node_id("widget", page.name, widget_name),
                        relation="reads_widget_state",
                        page=page.name,
                        evidence=f"{full_name} reads widget `{widget_name}` state.",
                    )
                    page.dependencies.append(edge)
                    add_graph_edge(graph_edges, edge_seen, edge)

        for action in page.actions:
            add_graph_node(
                graph_nodes,
                node_seen,
                DependencyNode(
                    node_id=node_id("action", page.name, action.name),
                    node_type="action",
                    name=action.name,
                    page=page.name,
                    path=action.path,
                ),
            )
            if action.datasource:
                datasource_id = node_id("datasource", None, action.datasource)
                edge = DependencyEdge(
                    source=node_id("action", page.name, action.name),
                    target=datasource_id,
                    relation="uses_datasource",
                    page=page.name,
                    evidence=f"{action.name} targets datasource `{action.datasource}`.",
                )
                page.dependencies.append(edge)
                add_graph_edge(graph_edges, edge_seen, edge)
                datasource = find_datasource(inventory, action.datasource)
                if datasource is None:
                    datasource = DatasourceInfo(name=action.datasource)
                    register_datasource(inventory, datasource)
                datasource.actions = unique_ordered(datasource.actions + [action.name])
            if action.execute_on_load:
                page.page_load_actions.append(action.name)
                flow = EventFlow(
                    name=f"{page.name} page load",
                    trigger="onPageLoad",
                    source_type="page",
                    source_name=page.name,
                    steps=[EventStep(step_type="query", name=action.name)],
                    notes=["Action is marked executeOnLoad in Appsmith metadata."],
                )
                page.event_flows.append(flow)
                edge = DependencyEdge(
                    source=node_id("page", page.name, page.name),
                    target=node_id("action", page.name, action.name),
                    relation="page_load_triggers",
                    page=page.name,
                    evidence=f"{action.name} executes on page load.",
                )
                page.dependencies.append(edge)
                add_graph_edge(graph_edges, edge_seen, edge)
                add_category_finding(
                    page.findings_by_category,
                    "frontend_orchestration",
                    f"Page load triggers `{action.name}` automatically.",
                )
            if action.action_type == "DB":
                add_category_finding(
                    page.findings_by_category,
                    "business_logic",
                    f"`{action.name}` performs direct database access from Appsmith.",
                )
            if action.query_analysis.mode == "write":
                add_category_finding(
                    page.findings_by_category,
                    "business_logic",
                    f"`{action.name}` is a client-triggered write operation.",
                )
            if action.action_type == "API" and action.query_analysis.endpoint:
                add_category_finding(
                    page.findings_by_category,
                    "frontend_orchestration",
                    f"`{action.name}` binds client state into API path/body: `{action.query_analysis.endpoint}`.",
                )
            if action.action_type == "JS":
                add_category_finding(
                    page.findings_by_category,
                    "frontend_orchestration",
                    f"`{action.name}` is a JSObject proxy action and depends on hidden JS logic.",
                )

        for widget in page.widgets:
            add_graph_node(
                graph_nodes,
                node_seen,
                DependencyNode(
                    node_id=node_id("widget", page.name, widget.name),
                    node_type="widget",
                    name=widget.name,
                    page=page.name,
                    path=widget.path,
                ),
            )
            for binding in widget.bindings:
                binding.widget_refs = unique_ordered(reference for reference in binding.references if reference in widget_names and reference != widget.name)
                if binding.category_hint == "presentation":
                    add_category_finding(
                        page.findings_by_category,
                        "presentation",
                        f"`{widget.name}` uses `{binding.source_key}` binding: `{binding.expression}`.",
                    )
                elif binding.category_hint == "business_logic":
                    add_category_finding(
                        page.findings_by_category,
                        "business_logic",
                        f"`{widget.name}` embeds rule-like binding in `{binding.source_key}`.",
                    )
                elif binding.category_hint == "frontend_orchestration":
                    add_category_finding(
                        page.findings_by_category,
                        "frontend_orchestration",
                        f"`{widget.name}` coordinates behavior through `{binding.source_key}`.",
                    )
                for action_name in binding.action_calls:
                    if action_name not in action_names:
                        continue
                    edge = DependencyEdge(
                        source=node_id("widget", page.name, widget.name),
                        target=node_id("action", page.name, action_name),
                        relation="references_query",
                        page=page.name,
                        evidence=f"Binding `{binding.source_key}` references `{action_name}`.",
                    )
                    page.dependencies.append(edge)
                    add_graph_edge(graph_edges, edge_seen, edge)
                for js_call in binding.js_calls:
                    owner = js_call.split(".", 1)[0]
                    if owner not in js_object_names:
                        continue
                    edge = DependencyEdge(
                        source=node_id("widget", page.name, widget.name),
                        target=node_id("jsobject", page.name, owner),
                        relation="references_jsobject",
                        page=page.name,
                        evidence=f"Binding `{binding.source_key}` calls `{js_call}`.",
                    )
                    page.dependencies.append(edge)
                    add_graph_edge(graph_edges, edge_seen, edge)
            if widget.visibility_bindings:
                add_category_finding(
                    page.findings_by_category,
                    "presentation",
                    f"`{widget.name}` has conditional visibility logic.",
                )
            if widget.disabled_bindings:
                add_category_finding(
                    page.findings_by_category,
                    "presentation",
                    f"`{widget.name}` has conditional disabled-state logic.",
                )
            for handler in widget.event_handlers:
                expanded_steps = expand_steps(handler.steps, method_index)
                flow = EventFlow(
                    name=f"{widget.name} {handler.event_name}",
                    trigger=handler.event_name,
                    source_type="widget",
                    source_name=widget.name,
                    steps=expanded_steps,
                    notes=handler.notes,
                )
                page.event_flows.append(flow)
                for action_name in handler.action_calls:
                    if action_name not in action_names:
                        continue
                    edge = DependencyEdge(
                        source=node_id("widget", page.name, widget.name),
                        target=node_id("action", page.name, action_name),
                        relation="triggers_query",
                        page=page.name,
                        evidence=f"Event `{handler.event_name}` triggers `{action_name}`.",
                    )
                    page.dependencies.append(edge)
                    add_graph_edge(graph_edges, edge_seen, edge)
                for js_call in handler.js_calls:
                    owner = js_call.split(".", 1)[0]
                    if owner not in js_object_names:
                        continue
                    edge = DependencyEdge(
                        source=node_id("widget", page.name, widget.name),
                        target=node_id("jsobject", page.name, owner),
                        relation="triggers_jsobject",
                        page=page.name,
                        evidence=f"Event `{handler.event_name}` calls `{js_call}`.",
                    )
                    page.dependencies.append(edge)
                    add_graph_edge(graph_edges, edge_seen, edge)
                if len(expanded_steps) > 3:
                    add_category_finding(
                        page.findings_by_category,
                        "frontend_orchestration",
                        f"`{widget.name}` `{handler.event_name}` expands into a multi-step flow.",
                    )

        if not page.widgets and not page.actions and not page.js_objects:
            add_category_finding(
                page.findings_by_category,
                "ambiguities",
                "No widgets, actions, or JSObjects were detected for this page from the available source.",
            )

        for js_object in page.js_objects:
            if any(method.role == "business_logic" for method in js_object.methods):
                add_category_finding(
                    page.findings_by_category,
                    "business_logic",
                    f"`{js_object.name}` contains reusable client-side business/data logic.",
                )
            if any(method.role == "frontend_orchestration" for method in js_object.methods):
                add_category_finding(
                    page.findings_by_category,
                    "frontend_orchestration",
                    f"`{js_object.name}` acts as an orchestration helper across queries/widgets.",
                )

        page.candidate_entities, page.main_entity, page.main_entity_confidence = infer_page_entities(page)
        page.actions_available = infer_actions_available(page)
        page.page_type = infer_page_type(page)
        page.purpose = infer_page_purpose(page)
        if not page.main_entity:
            add_category_finding(
                page.findings_by_category,
                "ambiguities",
                f"Main domain entity for page `{page.name}` could not be inferred confidently.",
            )
        elif page.main_entity_confidence == "low":
            add_category_finding(
                page.findings_by_category,
                "ambiguities",
                f"Main domain entity for page `{page.name}` is only weakly supported: `{page.main_entity}`.",
            )

        page.findings = unique_ordered(
            page.findings_by_category["business_logic"]
            + page.findings_by_category["frontend_orchestration"]
            + page.findings_by_category["presentation"]
            + page.findings_by_category["ambiguities"]
        )

    inventory.entities = aggregate_entities(inventory)
    duplication_findings = summarize_duplication(inventory)
    for finding in duplication_findings:
        add_category_finding(inventory.findings_by_category, "business_logic", finding)

    if any(datasource.plugin_id and "postgres" in datasource.plugin_id for datasource in inventory.datasources):
        inventory.global_findings.append("Postgres datasources are called directly from Appsmith.")
    if any(datasource.plugin_id and "mssql" in datasource.plugin_id for datasource in inventory.datasources):
        inventory.global_findings.append("MSSQL datasource access is embedded in the UI layer.")
    if any(datasource.plugin_id and "restapi" in datasource.plugin_id for datasource in inventory.datasources):
        inventory.global_findings.append("REST API calls are mixed with widget- and JSObject-level orchestration.")
    if any(page.js_objects for page in inventory.pages):
        inventory.global_findings.append("Reusable JSObjects were found; hidden workflow logic exists outside widget JSON.")

    for page in inventory.pages:
        for category, messages in page.findings_by_category.items():
            for message in messages:
                add_category_finding(inventory.findings_by_category, category, f"{page.name}: {message}")

    inventory.dependency_graph = {
        "nodes": graph_nodes,
        "edges": graph_edges,
    }


def analyze_source(source: Path) -> AppInventory:
    source = source.expanduser().resolve()
    bundle = load_source(source)
    inventory = AppInventory(
        source=str(source),
        source_type="zip" if source.is_file() else "directory",
        findings_by_category=empty_findings(),
    )
    parse_application_metadata(bundle, inventory)
    parse_datasources(bundle, inventory)

    for path, raw in bundle.files.items():
        if not path.endswith(".json"):
            continue
        data = bundle.json(path)
        if not isinstance(data, dict) or "/pages/" not in path:
            continue

        page_name = infer_page_name(path, data)
        page = get_page_bucket(inventory, page_name, path)

        if "/widgets/" in path:
            page.widgets.append(parse_widget(path, data))
            continue

        if "/queries/" in path and path.endswith("/metadata.json"):
            action_name = infer_query_name_from_path(path)
            text_path = str(Path(path).with_name(f"{action_name}.txt"))
            text_body = bundle.text(text_path)
            page.actions.append(parse_action_metadata(path, data, text_body))
            continue

        if "/jsobjects/" in path and path.endswith("/metadata.json"):
            js_name = infer_jsobject_name_from_path(path)
            js_path = str(Path(path).with_name(f"{js_name}.js"))
            page.js_objects.append(parse_jsobject(path, data, bundle.text(js_path)))

    post_process_inventory(inventory)
    inventory.pages.sort(key=lambda page: page.name.lower())
    inventory.datasources.sort(key=lambda datasource: datasource.name.lower())
    return inventory
