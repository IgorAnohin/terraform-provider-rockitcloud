# Tasks

## 1. Regression tests first

- [x] Add ELK manager schema and conversion tests.
- [x] Add `aws_paas_service` and data-source registration tests.
- [x] Add Logstash pipeline schema, import parser, finder, and not-found tests.
- [x] Add provider registration validation.
- [x] Run the focused tests before implementation. The expected red result was
  recorded on 2026-07-28: the new tests failed to compile because the ELK manager,
  constants, and Logstash resource did not yet exist.
- [x] Add tests first for live regressions discovered on 2026-07-29: data-source
  `nodes`, scalar anonymous role with one-item limit, ELK update-field
  filtering, API-elided password/options preservation, and SDK HTTP-body secret
  suppression. Each new contract was observed failing before its implementation
  fix.

## 2. ELK service implementation

- [x] Add ELK service type and logging service class constants.
- [x] Implement and register the ELK service manager.
- [x] Register ELK in the PaaS service resource and data source.

## 3. Logstash pipeline implementation

- [x] Implement create, list-based read, update, delete, and import.
- [x] Register `aws_paas_logstash_pipeline` in the provider.
- [x] Handle missing pipelines and delete-time 404 responses.

## 4. Documentation and QA

- [x] Document the ELK service block and Logstash pipeline resource.
- [x] Add an executable Terraform manual QA fixture.
- [x] Document positive, negative, drift, import, destroy, and regression
  scenarios.

## 5. Validation

- [x] Format Go and Terraform files.
- [x] Run focused PaaS and provider unit tests.
- [x] Run provider internal validation and documentation lint.
- [x] Check diffs for whitespace errors and changes outside the ELK scope.
- [x] Perform a final simplicity and overengineering review.

## 6. Credentialed cloud validation

- [x] Run ELK and Logstash pipeline create/read/update/import/drift/delete
  acceptance with strict error handling.
- [x] Verify the live scalar anonymous-role adapter and one-role schema.
- [x] Verify HA/arbitrator topology through raw DescribeService and instance
  roles.
- [x] Verify options create, API omission, resource-state preservation, and
  replacement-only lifecycle.
- [x] Verify monitoring labels change and monitoring removal in place with an
  unchanged service ID.
- [x] Record live API discrepancies for minimum memory, anonymousRole,
  multiline Logstash configuration, and API-elided create inputs.
- [x] Independently confirm cleanup after every run: zero ELK services, original
  VPC count, and existing Prometheus still READY.
- [ ] Manual QA owner: execute the private-network HTTP event and Kibana UI
  checks documented in `examples/paas-elk/MANUAL_QA.md` before release.
