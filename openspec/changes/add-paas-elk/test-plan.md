# ELK test plan

## Automated regression coverage

The credential-free suite must verify:

1. ELK is registered in the service manager, `aws_paas_service`, and
   `data.aws_paas_service`.
2. The ELK schema exposes only documented fields and inherits monitoring but not
   logging, users, databases, backups, or Kibana.
3. Service type, class, data-volume, arbitrator, non-empty version, password, and
   anonymous role constraints are enforced.
4. Terraform parameter names expand to the API contract, `anonymous_role` has at
   most one item and expands to a live-API scalar, scalar or array responses
   flatten safely, and API-elided password/options remain stable in resource
   state.
5. `aws_paas_logstash_pipeline` has correct lifecycle flags and import format.
6. Pipeline lookup selects an exact ID and represents empty or missing results as
   Terraform not-found.
7. The provider registers the new resource and passes internal schema validation.
8. Existing PaaS service and Prometheus-focused tests continue to pass.
9. ELK monitoring produces an in-place diff and its ModifyServiceParameters
   payload contains only monitoring fields; changing options requires
   replacement.
10. Generic ELK data-source read exposes `nodes` without panic and tolerates
    absent password/options.
11. SDK HTTP-body debug logging cannot expose ELK passwords or Logstash
    configuration in create or read paths.

## Manual cloud coverage

The executable procedure is maintained in
`examples/paas-elk/MANUAL_QA.md`. It covers:

- single-node ELK creation and stable second plan;
- data-source read, ELK service import, and Logstash pipeline import;
- high-availability ELK with an arbitrator;
- Prometheus monitoring and labels;
- one managed pipeline, exact-ID unit coverage, in-place configuration update,
  replacement on rename, and import;
- a successful HTTP event at the configured Logstash input and explicit scalar
  anonymous-role payload verification;
- options create, API omission, resource-state preservation, and replacement on
  change;
- the known live multiline-control-character failure and the one-line
  configuration workaround;
- out-of-band pipeline deletion and recreation;
- idempotent delete when the API reports not-found;
- ordered destroy of pipelines and the parent ELK service;
- empty version, invalid password, anonymous role, missing data volume, backup
  settings, conflicting service blocks, and invalid Logstash configuration;
- regression smoke tests for an existing PaaS service and Prometheus child
  resources.

## Release exit criteria

- All ELK-focused credential-free tests and linters pass.
- Core credentialed create/read/update/import/drift/delete scenarios pass and
  leave no acceptance ELK behind.
- The manual QA owner records the remaining private-network HTTP and UI evidence
  before release.
- A second `terraform plan` after every successful apply reports no changes.
- No code or documentation outside ELK integration points has changed.

## Credentialed validation results (2026-07-29)

- `TestAccPaaSELK_logstashPipelineBasic` passed in 1259.97 seconds. It covered ELK
  create/read, generic data source, pipeline create/read, pipeline and service
  import, in-place configuration update, rename replacement, out-of-band
  deletion and recreation, and ordered destroy.
- A high-availability ELK create completed and passed all first-step checks:
  scalar anonymous role, password, `options`, three ready instances, raw
  DescribeService `nodes.arbitrator`, an instance with role `arbitrator`, and no
  Kibana/Logstash endpoint on that arbitrator. The next plan exposed that the
  live API omits accepted `options`; the provider was then fixed tests-first to
  preserve that create input.
- The final exact-current-source single-node parameter scenario passed all three
  steps in 1100.58 seconds. It repeated the earlier 1127.24-second post-fix run
  after the final `TypeList` schema simplification and proved a stable plan with
  API-elided password/options, in-place monitoring-label change, monitoring
  removal, matching data-source refresh, and an unchanged service ID.
- One separate HA retry exceeded the cloud create timeout. Terraform cleanup
  completed, and the project inventory returned to its exact baseline. This did
  not exercise provider update code and is recorded as transient cloud behavior.
- Live contract discrepancies found during validation:
  `c5.large` is rejected because it has only 4 GiB of memory; arrays are rejected
  for `anonymousRole` while a scalar is accepted; literal newlines in Logstash
  configuration are rejected as control characters; and DescribeService omits
  accepted password/options.
- Independent inventory after every failed or successful run confirmed zero ELK
  services, the original three VPCs, and the pre-existing Prometheus service in
  `READY`.
