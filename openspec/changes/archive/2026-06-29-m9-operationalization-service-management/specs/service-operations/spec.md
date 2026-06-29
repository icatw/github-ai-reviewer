## ADDED Requirements

### Requirement: Durable service management
The project SHALL provide documented systemd-based service management for the GitHub AI Reviewer deployment.

#### Scenario: Version-controlled service unit exists
- **WHEN** an operator prepares a production host
- **THEN** the repository provides a systemd unit template or deployment artifact for `github-ai-reviewer.service`
- **AND** the unit runs the built server from the repository working directory using an environment file outside git

#### Scenario: Service restarts after process failure
- **WHEN** the service process exits unexpectedly
- **THEN** systemd is configured to restart it according to the documented restart policy
- **AND** the restart path does not require an interactive shell or Hermes session

#### Scenario: Service status is safe to inspect
- **WHEN** an operator checks service status or recent logs
- **THEN** documented commands show service state and safe lifecycle metadata
- **AND** they do not print `.env.production`, private keys, tokens, raw webhook payloads, raw prompts, raw model responses, or private source

### Requirement: Operations runbook
The project SHALL document repeatable operational commands for deploying, checking, and rolling back the service.

#### Scenario: Operator installs or refreshes service unit
- **WHEN** an operator follows the operations runbook
- **THEN** it provides commands for building the server, installing or refreshing the service unit, reloading systemd, enabling the service, and starting or restarting it
- **AND** it names which files must remain outside git

#### Scenario: Operator verifies health through nginx
- **WHEN** an operator verifies the deployment
- **THEN** the runbook includes local and public `/healthz` checks
- **AND** it includes nginx route verification for `/healthz` and `/github/webhook` to the configured local service port

#### Scenario: Operator rolls back service changes
- **WHEN** a deployment or service restart fails
- **THEN** the runbook provides rollback or diagnostic commands using systemd and safe logs
- **AND** it does not require exposing secrets or reverting unrelated repository changes

### Requirement: Systemd cutover verification
The project SHALL verify that the live deployment is owned by systemd rather than an interactive background process.

#### Scenario: Live service runs under systemd
- **WHEN** M9 cutover is complete
- **THEN** `github-ai-reviewer.service` is enabled and active
- **AND** the public health endpoint remains successful through nginx

#### Scenario: Temporary process is not the long-term owner
- **WHEN** the service has been cut over to systemd
- **THEN** the previous interactive or Hermes-managed background process is stopped or no longer serving the deployment port
- **AND** service restart commands use systemd as the source of truth
