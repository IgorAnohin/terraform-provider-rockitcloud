# ELK test plan

## Automated regression coverage

The credential-free suite must verify:

1. ELK is registered in the service manager, `aws_paas_service`, and
   `data.aws_paas_service`.
2. The ELK schema exposes only documented fields and inherits monitoring but not
   logging, users, databases, backups, or Kibana.
3. Service type, class, data-volume, arbitrator, non-empty version, password, and
   anonymous role constraints are enforced.
4. Terraform parameter names expand to the API contract and camelCase API
   parameters flatten without order-only diffs; an API-elided password remains
   stable in state.
5. `aws_paas_logstash_pipeline` has correct lifecycle flags and import format.
6. Pipeline lookup selects an exact ID and represents empty or missing results as
   Terraform not-found.
7. The provider registers the new resource and passes internal schema validation.
8. Existing PaaS service and Prometheus-focused tests continue to pass.
9. ELK monitoring and options produce in-place diffs, while replacement-only
   fields retain their declared lifecycle.

## Manual cloud coverage

The executable procedure is maintained in
`examples/paas-elk/MANUAL_QA.md`. It covers:

- single-node ELK creation and stable second plan;
- data-source read, ELK service import, and Logstash pipeline import;
- high-availability ELK with an arbitrator;
- Prometheus monitoring and labels;
- one managed pipeline, exact-ID unit coverage, in-place configuration update,
  replacement on rename, and import;
- a successful HTTP event at the configured Logstash input and explicit
  anonymous-role payload verification;
- out-of-band pipeline deletion and recreation;
- idempotent delete when the API reports not-found;
- ordered destroy of pipelines and the parent ELK service;
- empty version, invalid password, anonymous role, missing data volume, backup
  settings, conflicting service blocks, and invalid Logstash configuration;
- regression smoke tests for an existing PaaS service and Prometheus child
  resources.

## Release exit criteria

- All ELK-focused credential-free tests and linters pass.
- Every manual scenario has recorded command output and API/UI evidence.
- A second `terraform plan` after every successful apply reports no changes.
- No code or documentation outside ELK integration points has changed.
