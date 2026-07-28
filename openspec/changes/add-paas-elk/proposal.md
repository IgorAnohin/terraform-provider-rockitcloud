# Change: Add ELK support to the PaaS provider

## Why

The provider can manage several K2 Cloud PaaS service types, but it cannot declare
an ELK logging service or manage the Logstash pipelines that belong to that
service. Users therefore cannot keep the ELK service and Logstash pipeline
configuration in Terraform.

## What Changes

- Add an `elk` service block to the existing `aws_paas_service` resource and data
  source.
- Map the ELK service to PaaS `serviceType = "elk"` and
  `serviceClass = "logging"`.
- Support the documented ELK parameters: version, password, anonymous access,
  anonymous roles, monitoring, monitoring labels, and additional options.
- Add `aws_paas_logstash_pipeline` with create, read, update, delete, drift
  handling, and import support.
- Add unit regression tests, provider documentation, and a reproducible manual QA
  configuration and test plan.

## Scope

This change is intentionally limited to ELK:

- no changes to existing PaaS service behavior;
- no refactoring of generic PaaS or Prometheus child-resource helpers;
- no Elasticsearch or ELK snapshot repository support;
- no changes to backup resources;
- no repair or expansion of the repository's existing cloud E2E suite;
- no ELK-specific AWS SDK upgrade, because the pinned SDK already provides the
  required Logstash pipeline operations.

## Impact

- Affected capability: `paas-elk`
- Affected Terraform objects:
  - `aws_paas_service`
  - `data.aws_paas_service`
  - new resource `aws_paas_logstash_pipeline`
- Affected provider registration and PaaS documentation
- No breaking schema changes to existing services
