# Tasks

## 1. Regression tests first

- [x] Add ELK manager schema and conversion tests.
- [x] Add `aws_paas_service` and data-source registration tests.
- [x] Add Logstash pipeline schema, import parser, finder, and not-found tests.
- [x] Add provider registration validation.
- [x] Run the focused tests before implementation. The expected red result was
  recorded on 2026-07-28: the new tests failed to compile because the ELK manager,
  constants, and Logstash resource did not yet exist.

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

- [ ] Execute every scenario in `examples/paas-elk/MANUAL_QA.md` after dedicated
  K2 Cloud credentials and QA infrastructure IDs are supplied.
