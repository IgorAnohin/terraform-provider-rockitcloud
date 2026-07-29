# PaaS ELK Specification

## ADDED Requirements

### Requirement: ELK service declaration

The provider SHALL allow an ELK service to be declared with an `elk` block in
`aws_paas_service`.

#### Scenario: Create a minimal ELK service

- **GIVEN** a valid PaaS network, security group, root volume, data volume, and
  instance type
- **WHEN** the user configures `elk.version` with a supported version
- **THEN** the provider SHALL call the PaaS service lifecycle with service type
  `elk`
- **AND** service class SHALL default to `logging`
- **AND** a successful refresh SHALL preserve the `elk` block without a
  Terraform diff

#### Scenario: Read ELK through the generic data source

- **GIVEN** an existing ELK service ID
- **WHEN** `data.aws_paas_service` reads the service
- **THEN** the provider SHALL populate the `elk` block and common service
  attributes
- **AND** it SHALL NOT invent `password` or `options` when the API omits them

#### Scenario: Import an ELK service

- **GIVEN** an ELK service created outside the current Terraform state
- **WHEN** it is imported by service ID
- **THEN** refresh SHALL select the ELK manager from the API service type
- **AND** a matching configuration containing only API-readable values SHALL
  produce no changes
- **AND** API-elided `password` and `options` SHALL NOT be inferred or silently
  suppressed

### Requirement: ELK topology constraints

The provider SHALL require a data volume, allow an arbitrator, and reject backup
settings for ELK.

#### Scenario: Configure an HA ELK service with arbitrator

- **GIVEN** valid subnets for a high-availability deployment
- **WHEN** `high_availability` and `arbitrator_required` are true
- **THEN** Terraform schema validation SHALL accept the configuration

#### Scenario: Omit the data volume

- **WHEN** an ELK service block is configured without `data_volume`
- **THEN** Terraform schema validation SHALL reject the configuration before an
  API request

#### Scenario: Configure backup settings

- **WHEN** `backup_settings` is configured together with an ELK service block
- **THEN** Terraform schema validation SHALL reject the configuration

### Requirement: ELK service parameters

The provider SHALL support `version`, `password`, `allow_anonymous`,
`anonymous_role`, `options`, and the common `monitoring` block according to the
published ELK API contract.

#### Scenario: Reject an empty ELK version

- **WHEN** the configured version is empty
- **THEN** Terraform schema validation SHALL reject it

#### Scenario: Delegate version support to PaaS

- **WHEN** the configured version is non-empty
- **THEN** the provider SHALL pass it to PaaS without applying a static version
  allowlist
- **AND** PaaS SHALL remain the source of truth for supported versions

#### Scenario: Validate an ELK password

- **WHEN** the configured password is shorter than 8 characters, longer than 128
  characters, or contains a hyphen, exclamation mark, colon, semicolon, percent
  sign, single quote, double quote, backtick, or backslash
- **THEN** Terraform schema validation SHALL reject it

#### Scenario: Preserve API-elided ELK create inputs

- **GIVEN** an existing ELK service whose state contains configured `password`
  or `options`
- **WHEN** the PaaS read response omits either value
- **THEN** refresh SHALL preserve the prior resource-state value
- **AND** the omission SHALL NOT cause a false replacement diff
- **AND** provider debug logs SHALL NOT include ELK service parameters

#### Scenario: Configure anonymous access

- **GIVEN** anonymous access is enabled
- **WHEN** the user assigns an anonymous role
- **THEN** the role SHALL be either `viewer` or `editor`
- **AND** Terraform SHALL reject more than one configured role
- **AND** the provider SHALL send the one role as a scalar value
- **AND** refresh SHALL accept either the live scalar or a documented array
  response

#### Scenario: Configure additional ELK options

- **WHEN** the user configures `options`
- **THEN** the provider SHALL pass them during service creation
- **AND** changing `options` SHALL require service replacement
- **AND** an API omission SHALL NOT erase them from existing resource state

#### Scenario: Update monitoring

- **GIVEN** an existing ELK service and a Prometheus service in the same project
  and VPC
- **WHEN** the user adds, changes, or removes the ELK `monitoring` block
- **THEN** the provider SHALL pass monitoring, monitor target, and labels through
  the existing service-parameter update lifecycle
- **AND** the ModifyServiceParameters request SHALL contain no ELK create-only
  parameters

#### Scenario: Reject unrelated service fields

- **WHEN** the user declares ELK
- **THEN** the ELK block SHALL NOT expose `kibana`, `logging`, users, or databases

### Requirement: Logstash pipeline lifecycle

The provider SHALL manage ELK Logstash pipelines with
`aws_paas_logstash_pipeline`.

#### Scenario: Create a pipeline

- **GIVEN** an existing ELK service
- **WHEN** the user provides a non-empty name and Logstash configuration
- **THEN** the provider SHALL create the pipeline under that service
- **AND** store the returned pipeline ID as the Terraform state ID
- **AND** expose the same value as `pipeline_id`

#### Scenario: Refresh one of multiple pipelines

- **GIVEN** two or more pipelines in one ELK service
- **WHEN** Terraform refreshes one pipeline
- **THEN** the provider SHALL list the service pipelines
- **AND** select only the item whose ID exactly equals the Terraform state ID

#### Scenario: Update pipeline configuration

- **GIVEN** a managed pipeline
- **WHEN** only `configuration` changes
- **THEN** the provider SHALL update the pipeline in place
- **AND** preserve its name and state ID

#### Scenario: PaaS rejects a multiline pipeline configuration

- **GIVEN** an existing managed pipeline
- **WHEN** the live PaaS API rejects literal newline characters
- **THEN** the provider SHALL return the PaaS error without replacing the
  pipeline or clearing its state
- **AND** the provider SHALL NOT add an undocumented schema restriction that
  rejects otherwise non-empty multiline configuration

#### Scenario: Protect pipeline configuration

- **WHEN** Terraform displays a plan containing pipeline configuration
- **THEN** the configuration SHALL be marked sensitive
- **AND** provider debug logs SHALL NOT include the configuration contents

#### Scenario: Rename a pipeline

- **GIVEN** a managed pipeline
- **WHEN** `name` changes
- **THEN** Terraform SHALL replace the pipeline because the API does not support
  rename

#### Scenario: Reject the system pipeline name

- **WHEN** the configured name is `beats-to-elasticsearch`
- **THEN** Terraform schema validation SHALL reject it
- **AND** refresh SHALL refuse to adopt or delete a system pipeline with that
  name

#### Scenario: Pipeline is deleted out of band

- **GIVEN** a managed pipeline that no longer appears in the API list
- **WHEN** Terraform refreshes state
- **THEN** the provider SHALL remove the pipeline from state

#### Scenario: Delete an already absent pipeline

- **GIVEN** the API reports the pipeline as not found during delete
- **WHEN** Terraform destroys the resource
- **THEN** deletion SHALL succeed without an error

### Requirement: Logstash pipeline import

The provider SHALL import a pipeline using `service_id/pipeline_id`.

#### Scenario: Import a valid pipeline ID

- **WHEN** the import identifier contains exactly two non-empty slash-separated
  parts
- **THEN** the first part SHALL populate `service_id`
- **AND** the second part SHALL populate both the Terraform state ID and
  `pipeline_id`

#### Scenario: Reject an invalid pipeline import ID

- **WHEN** either part is empty or extra slash-separated parts are present
- **THEN** import SHALL fail with an error that states the required
  `service_id/pipeline_id` format

### Requirement: Existing PaaS compatibility

Adding ELK SHALL NOT change the schema or lifecycle behavior of any existing PaaS
service or Prometheus child resource.

#### Scenario: Validate existing provider schemas

- **WHEN** the provider performs internal schema validation and the existing PaaS
  regression tests run
- **THEN** all pre-existing checks SHALL continue to pass
