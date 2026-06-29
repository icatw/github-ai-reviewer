## ADDED Requirements

### Requirement: Durable service manager for verified deployment
Verified deployments SHALL be managed by a durable host service manager rather than an interactive shell process.

#### Scenario: E2E-verified service survives session exit
- **WHEN** a deployment has passed real GitHub App E2E verification
- **THEN** the service is configured to run under systemd or an equivalent durable service manager
- **AND** the health endpoint remains available after the initiating shell or agent session ends
