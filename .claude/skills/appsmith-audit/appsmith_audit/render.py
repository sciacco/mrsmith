from __future__ import annotations

import re
from collections import Counter
from pathlib import Path
from typing import Iterable, List

from .models import AppInventory, DependencyEdge, PageInfo


def repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def template_path(name: str) -> Path:
    return repo_root() / "templates" / name


def load_template_header(name: str) -> str:
    path = template_path(name)
    if not path.exists():
        return ""
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("#"):
            return line
    return ""


def bullet_list(values: Iterable[str], fallback: str = "- None detected.") -> List[str]:
    items = [value for value in values if value]
    if not items:
        return [fallback]
    return [f"- {value}" for value in items]


def unique_ordered(values: Iterable[str]) -> List[str]:
    seen = set()
    ordered = []
    for value in values:
        if value and value not in seen:
            ordered.append(value)
            seen.add(value)
    return ordered


def slugify(value: str) -> str:
    text = re.sub(r"[^A-Za-z0-9]+", "-", value.strip().lower()).strip("-")
    return text or "page"


def summarize_widget(widget) -> str:
    details = [f"Type: {widget.widget_type}", f"Role: {widget.role}"]
    if widget.event_handlers:
        details.append("Events: " + ", ".join(handler.event_name for handler in widget.event_handlers))
    if widget.visibility_bindings:
        details.append("Visibility logic")
    if widget.disabled_bindings:
        details.append("Disabled-state logic")
    return f"{widget.name}: " + "; ".join(details)


def summarize_action(action) -> str:
    trigger = "page load" if action.execute_on_load else "user/event driven"
    entities = ", ".join(action.query_analysis.entities) or "ambiguous entity"
    mode = action.query_analysis.mode
    return (
        f"{action.name}: {action.action_type} {action.query_analysis.operation_type}; "
        f"trigger={trigger}; mode={mode}; entities={entities}"
    )


def summarize_flow_steps(page: PageInfo) -> List[str]:
    items: List[str] = []
    for flow in page.event_flows:
        if flow.steps:
            chain = " -> ".join(f"{step.step_type}:{step.name}" for step in flow.steps)
        else:
            chain = "No downstream steps extracted."
        items.append(f"{flow.source_name} {flow.trigger}: {chain}")
    return items


def summarize_category_findings(messages: List[str], *, limit: int = 8) -> List[str]:
    direct_db = [message for message in messages if "direct database access from Appsmith" in message]
    writes = [message for message in messages if "client-triggered write operation" in message]
    page_load = [message for message in messages if message.startswith("Page load triggers")]
    proxy = [message for message in messages if "JSObject proxy action" in message]
    multi_step = [message for message in messages if "multi-step flow" in message]
    helper = [message for message in messages if "orchestration helper" in message or "business/data logic" in message]

    summary: List[str] = []
    if direct_db:
        summary.append(f"{len(direct_db)} direct DB-backed actions are executed from Appsmith.")
    if writes:
        summary.append(f"{len(writes)} client-triggered write actions were detected.")
    if page_load:
        summary.append(f"{len(page_load)} actions execute automatically on page load.")
    if proxy:
        summary.append(f"{len(proxy)} generated JSObject proxy actions were detected.")
    if helper:
        summary.extend(helper[:2])
    if multi_step:
        summary.extend(multi_step[:3])

    drop_patterns = (
        "coordinates behavior through `onClick`",
        "coordinates behavior through `onFilterUpdate`",
        "coordinates behavior through `default",
        "coordinates behavior through `isDisabled`",
        "coordinates behavior through `text`",
        "coordinates behavior through `sourceData`",
        "coordinates behavior through `title`",
        "embeds rule-like binding in `borderRadius`",
    )
    remaining = [
        message
        for message in messages
        if message not in direct_db + writes + page_load + proxy + multi_step + helper
        and not any(pattern in message for pattern in drop_patterns)
    ]
    summary.extend(remaining[: max(0, limit - len(summary))])
    return unique_ordered(summary)[:limit]


def page_hidden_logic(page: PageInfo) -> List[str]:
    business = summarize_category_findings(page.findings_by_category.get("business_logic", []), limit=8)
    orchestration = summarize_category_findings(page.findings_by_category.get("frontend_orchestration", []), limit=8)
    return unique_ordered(business + orchestration)


def summarize_inventory_risks(inv: AppInventory) -> List[str]:
    risks = summarize_category_findings(inv.findings_by_category.get("ambiguities", []), limit=6)
    if risks:
        return risks
    if any(not page.main_entity for page in inv.pages):
        return ["Some pages still have ambiguous main entities."]
    return ["No major ambiguities flagged."]


def summarize_findings_section(messages: List[str], *, limit: int = 12) -> List[str]:
    if not messages:
        return []

    if any(" uses `" in message for message in messages):
        theme_count = sum(1 for message in messages if "appsmith.theme." in message)
        counter = Counter()
        passthrough: List[str] = []
        for message in messages:
            if "appsmith.theme." in message:
                continue
            match = re.match(r"^(.+?): `(.+?)` uses `(.+?)` binding: `(.+)`\.$", message)
            if match:
                page_name, _, binding_key, _ = match.groups()
                counter[f"{page_name}: {binding_key}"] += 1
            else:
                passthrough.append(message)
        summarized = [
            f"{key} presentation binding appears {count} times."
            for key, count in counter.most_common(limit)
        ]
        if theme_count:
            summarized.insert(0, f"Theme-driven presentation bindings appear {theme_count} times.")
        return unique_ordered(summarized + passthrough)[:limit]

    if any(message.startswith("SELECT logic for") or message.startswith("GET logic for") or message.startswith("POST logic for") or message.startswith("PUT logic for") or message.startswith("DELETE logic for") or message.startswith("JS_METHOD logic for") for message in messages):
        counter = Counter()
        passthrough: List[str] = []
        for message in messages:
            match = re.match(r"^(SELECT|GET|POST|PUT|DELETE|INSERT|UPDATE|JS_METHOD) logic for (.+) appears in (\d+) actions\.$", message)
            if match:
                op, entity, count = match.groups()
                counter[f"{op} {entity}"] += int(count)
            else:
                passthrough.append(message)
        summarized = [
            f"{key} is duplicated across {count} actions."
            for key, count in counter.most_common(limit)
        ]
        return unique_ordered(summarized + passthrough)[:limit]

    return summarize_category_findings(messages, limit=limit)


def render_inventory_markdown(inv: AppInventory) -> str:
    header = load_template_header("app-inventory.md") or "# Application Inventory"
    jsobject_names = [js_object.name for page in inv.pages for js_object in page.js_objects]
    lines = [
        header,
        "",
        "## Application",
        f"- Name: {inv.application_name or 'Unknown'}",
        f"- Source type: {inv.source_type}",
        f"- Source path: {inv.source}",
        "",
        "## Pages",
        *bullet_list(f"{page.name}: {page.page_type} ({page.main_entity or 'ambiguous entity'})" for page in inv.pages),
        "",
        "## Datasources",
        *bullet_list(f"{datasource.name}: {datasource.plugin_id or 'unknown plugin'}" for datasource in inv.datasources),
        "",
        "## JSObjects",
        *bullet_list(jsobject_names, fallback="- No JSObjects detected."),
        "",
        "## Global Findings",
        *bullet_list(inv.global_findings, fallback="- No global findings generated."),
        "",
        "## Risks",
        *bullet_list(summarize_inventory_risks(inv)),
    ]
    return "\n".join(lines).rstrip() + "\n"


def render_page_audit(page: PageInfo) -> str:
    header = load_template_header("page-audit.md") or "# Page Audit: {{PAGE_NAME}}"
    header = header.replace("{{PAGE_NAME}}", page.name)
    lines = [
        header,
        "",
        "## Purpose",
        page.purpose,
        "",
        "## Widgets",
        *bullet_list((summarize_widget(widget) for widget in page.widgets), fallback="- No widgets detected."),
        "",
        "## Queries and Actions",
        *bullet_list((summarize_action(action) for action in page.actions), fallback="- No actions detected."),
        "",
        "## Event Flow",
        *bullet_list(summarize_flow_steps(page), fallback="- No event flows reconstructed."),
        "",
        "## Hidden Logic",
        *bullet_list(page_hidden_logic(page), fallback="- No hidden logic findings extracted."),
        "",
        "## Candidate Domain Entities",
        *bullet_list(page.candidate_entities, fallback="- No domain entities inferred."),
        "",
        "## Migration Notes",
        *bullet_list(
            [
                f"Main entity: {page.main_entity}" if page.main_entity else "Main entity remains ambiguous.",
                f"Confidence: {page.main_entity_confidence}",
                f"Page type: {page.page_type}",
                "Available actions: " + ", ".join(page.actions_available) if page.actions_available else "No CRUD/reporting actions inferred.",
            ]
            + page.findings_by_category.get("ambiguities", []),
            fallback="- No migration notes generated.",
        ),
    ]
    return "\n".join(lines).rstrip() + "\n"


def render_datasource_catalog(inv: AppInventory) -> str:
    header = load_template_header("datasource-catalog.md") or "# Datasource and Query Catalog"
    lines = [header, ""]
    if not inv.datasources and not any(page.actions for page in inv.pages):
        lines.append("No datasources or queries detected.")
        return "\n".join(lines).rstrip() + "\n"

    for datasource in inv.datasources:
        lines.extend(
            [
                f"## {datasource.name}",
                f"- Type: {datasource.plugin_id or datasource.plugin_type}",
                "- Purpose: Datasource connection referenced by one or more Appsmith actions.",
                "- Read/Write: Mixed or depends on downstream queries.",
                f"- Inputs: Actions referencing this datasource: {', '.join(datasource.actions) if datasource.actions else 'none detected'}",
                "- Outputs: Depends on individual queries.",
                f"- Dependencies: {datasource.path or 'definition path unavailable'}",
                "- Rewrite recommendation: Move access behind explicit backend APIs and keep datasource secrets out of the client.",
                "",
            ]
        )

    for page in inv.pages:
        for action in page.actions:
            lines.extend(
                [
                    f"## {action.name}",
                    f"- Type: {action.action_type} / {action.query_analysis.operation_type}",
                    f"- Purpose: Supports page `{page.name}` with entities: {', '.join(action.query_analysis.entities) or 'ambiguous'}.",
                    f"- Read/Write: {action.query_analysis.mode}",
                    f"- Inputs: {', '.join(action.parameters) if action.parameters else 'none detected'}",
                    "- Outputs: Output shape is not inferred statically; inspect runtime data samples if needed.",
                    f"- Dependencies: datasource={action.datasource or 'none'}, backend={action.query_analysis.backend_responsibility}",
                    f"- Rewrite recommendation: {action.query_analysis.backend_responsibility}",
                    "",
                ]
            )
    return "\n".join(lines).rstrip() + "\n"


def render_findings_summary(inv: AppInventory) -> str:
    header = load_template_header("findings-summary.md") or "# Findings Summary"
    duplication = []
    seen = set()
    for page in inv.pages:
        for finding in page.findings_by_category.get("business_logic", []):
            if "appears in" in finding and finding not in seen:
                duplication.append(finding)
                seen.add(finding)
    concerns = []
    if any(datasource.plugin_id and "postgres" in datasource.plugin_id for datasource in inv.datasources):
        concerns.append("Postgres access is executed directly from Appsmith.")
    if any(datasource.plugin_id and "mssql" in datasource.plugin_id for datasource in inv.datasources):
        concerns.append("MSSQL access is executed directly from Appsmith.")
    if any(action.query_analysis.mode == "write" for page in inv.pages for action in page.actions):
        concerns.append("Client-triggered write operations should be reviewed for security and transactional integrity.")
    blockers = inv.findings_by_category.get("ambiguities", [])
    next_steps = [
        "Validate ambiguous entities and page purposes with a domain expert.",
        "Replace direct DB access with explicit backend endpoints before rewrite planning.",
        "Use the dependency graph to prioritize pages with heavy JSObject orchestration.",
    ]

    lines = [
        header,
        "",
        "## Embedded Business Rules",
        *bullet_list(summarize_findings_section(inv.findings_by_category.get("business_logic", []), limit=14), fallback="- No business-rule findings generated."),
        "",
        "## Frontend Orchestration Findings",
        *bullet_list(
            summarize_findings_section(inv.findings_by_category.get("frontend_orchestration", []), limit=14),
            fallback="- No orchestration findings generated.",
        ),
        "",
        "## Presentation Findings",
        *bullet_list(
            summarize_findings_section(inv.findings_by_category.get("presentation", []), limit=12),
            fallback="- No presentation findings generated.",
        ),
        "",
        "## Duplication",
        *bullet_list(duplication, fallback="- No duplicated logic heuristics triggered."),
        "",
        "## Security or Architecture Concerns",
        *bullet_list(concerns, fallback="- No architecture concerns generated."),
        "",
        "## Migration Blockers",
        *bullet_list(summarize_findings_section(blockers, limit=10), fallback="- No major blockers flagged."),
        "",
        "## Recommended Next Steps",
        *bullet_list(next_steps),
    ]
    return "\n".join(lines).rstrip() + "\n"


def render_markdown(inv: AppInventory) -> str:
    return render_inventory_markdown(inv)


def write_markdown_artifacts(inv: AppInventory, output_dir: Path) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    pages_dir = output_dir / "pages"
    pages_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "app-inventory.md").write_text(render_inventory_markdown(inv), encoding="utf-8")
    (output_dir / "datasource-catalog.md").write_text(render_datasource_catalog(inv), encoding="utf-8")
    (output_dir / "findings-summary.md").write_text(render_findings_summary(inv), encoding="utf-8")
    for page in inv.pages:
        (pages_dir / f"{slugify(page.name)}.md").write_text(render_page_audit(page), encoding="utf-8")
