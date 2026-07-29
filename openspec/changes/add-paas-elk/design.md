# Design: PaaS ELK

## Context

PaaS services share one Terraform resource. Each service type implements the
existing `services.ServiceManager` contract and contributes its own nested schema
and parameter conversion. Child entities with their own PaaS API lifecycle are
modeled as separate Terraform resources.

The pinned K2 Cloud AWS SDK already exposes:

- `CreateLogstashPipeline`
- `ListLogstashPipelines`
- `ModifyLogstashPipeline`
- `DeleteLogstashPipeline`

The API has no operation to describe one pipeline, so refresh must list all
pipelines for an ELK service and select the exact pipeline ID.

The implementation was also checked against `C2Devel/botocore` `master` at
commit `3030cb72079bdc2e57a11796732cb46d9a6c9068`. The four Logstash operations,
their HTTP methods and paths, required request members, and response shapes are
identical to the model bundled with the pinned ROCKIT21 SDK. The generic
`parameters` member of the service operations remains untyped in both models,
so ELK-specific parameter validation continues to follow the published K2 Cloud
contract and live API behavior.

## Goals

- Reuse the current PaaS service manager architecture.
- Keep all changes additive and ELK-specific.
- Preserve Terraform import and drift-recovery behavior.
- Reject only constraints that are explicitly documented and stable.
- Provide useful regression coverage without requiring cloud credentials.

## Decisions

### ELK uses a dedicated service manager

The manager has these capabilities:

| Capability | Value |
|---|---|
| service type | `elk` |
| service class | `logging` |
| default class | `logging` |
| arbitrator | supported |
| backup settings | unsupported |
| data volume | required |
| users and databases | unsupported |
| log forwarding | unsupported |
| monitoring | supported |

ELK does not expose `kibana`: Kibana is an integral ELK component in K2 Cloud.
ELK also does not expose the common `logging` block because it is the logging
destination rather than a service that forwards logs.

### Anonymous role adapts to the live scalar contract

The public ELK API documentation defines `anonymousRole` as an array of strings,
but the live API rejects both one-element and multi-element arrays and accepts a
single `viewer` or `editor` string. Terraform models `anonymous_role` as a list
matching the published configuration shape, restricts it to one item,
and sends that item as a scalar. Refresh tolerates either a scalar or an array so
the provider remains compatible if the API response is aligned with the
documentation later.

### Versions are delegated to the API

The public API currently lists `7.17` and `8.17`, but the set of available
versions can change independently of the provider and is also returned by PaaS.
Terraform rejects an empty version and delegates support checks to the API. This
matches the existing PaaS managers and avoids requiring a provider release for
every new ELK version.

### Logstash pipelines are separate resources

`aws_paas_logstash_pipeline` contains:

- `service_id`: required and replacement-only;
- `pipeline_id`: computed;
- `name`: required and replacement-only because ModifyLogstashPipeline cannot
  rename a pipeline;
- `configuration`: required, sensitive, and editable in place.

The Terraform state ID is the PaaS pipeline ID. Import uses
`service_id/pipeline_id`, because both identifiers are required for every read,
update, and delete request.

### Pipeline refresh uses list and exact ID matching

Read calls ListLogstashPipelines for the parent ELK service and matches only the
state ID. A missing result clears Terraform state. A delete API not-found response
is treated as success.

### Parent service readiness is observed after mutations

After create, update, and delete, the resource uses the existing PaaS service
waiter. If the API keeps the parent in `READY`, the waiter completes immediately;
if it transitions through `UPDATING`, Terraform waits for a stable result.

### Only monitoring parameters are editable

The published ELK contract marks only `monitoring`, `monitorBy`, and
`monitoringLabels` as editable. ModifyServiceParameters therefore receives only
those three expanded fields. Create-only `version`, `password`,
`allowAnonymous`, `anonymousRole`, and `options` are never repeated in an update
request.

`options` is replacement-only in Terraform. The live DescribeService response
can omit both `options` and `password` after accepting them during create, so an
existing resource preserves those values from its prior state. A data source or
fresh import cannot reconstruct values that the API does not return.

## Risks and Mitigations

- The public documentation and live API disagree about `anonymousRole`. Focused
  tests lock the one-item Terraform schema, scalar request adapter, and tolerant
  response conversion. Manual QA records the live scalar value.
- DescribeService can omit ELK `password` and `options`. Refresh preserves
  existing resource-state values, preventing false replacement diffs. A fresh
  import cannot recover either omitted create input; configurations that set
  them must retain the original state or explicitly accept replacement.
- Logstash configuration syntax is validated by the service, not duplicated in
  the provider. It is marked sensitive and excluded from provider debug logs.
  Although the published examples use multiline configuration, the live API
  currently rejects literal newlines with `control characters are not allowed`.
  The working example is one line, and manual QA records the multiline response
  as a cloud defect without adding an undocumented provider-side restriction.
- The live API rejects `c5.large` for ELK because it has only 4 GiB of memory.
  The example uses the verified `m5.large` flavor. Instance capabilities remain
  API-owned and are not hard-coded in provider validation.
- The repository-wide cloud E2E suite is not usable. Focused unit tests and the
  committed ELK-only QA procedure provide deterministic and live coverage
  without expanding this change into repairs of unrelated acceptance tests.
