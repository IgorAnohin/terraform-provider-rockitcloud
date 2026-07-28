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

### Anonymous roles are represented as a set

The public ELK API documentation defines `anonymousRole` as an array of strings.
Terraform therefore exposes `anonymous_role` as a set restricted to `viewer` and
`editor`. A set prevents order-only diffs. Live QA must verify the actual
CreateService and DescribeService payloads because the cloud UI presents the
setting as a single conceptual role.

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

## Risks and Mitigations

- The API documentation and UI may disagree about the shape of
  `anonymousRole`. Unit tests lock the documented array contract; live QA records
  the actual payload before release.
- DescribeService may omit the ELK password. Refresh preserves an existing
  sensitive state value when the API response does not contain one, preventing a
  false replacement diff.
- Logstash configuration syntax is validated by the service, not duplicated in
  the provider. It is marked sensitive and excluded from provider debug logs.
  Manual QA covers a valid create and update and an invalid configuration
  response.
- Existing cloud E2E tests are not usable. Focused unit tests and the committed
  manual QA procedure provide deterministic coverage until live credentials are
  supplied.
